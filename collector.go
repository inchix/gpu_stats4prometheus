package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// queryFields are the nvidia-smi --query-gpu fields in the exact order parsed.
var queryFields = []string{
	"uuid",
	"name",
	"driver_version",
	"pci.bus_id",
	"vbios_version",
	"temperature.gpu",
	"temperature.memory",
	"utilization.gpu",
	"utilization.memory",
	"utilization.encoder",
	"utilization.decoder",
	"memory.total",
	"memory.used",
	"memory.free",
	"power.draw",
	"power.limit",
	"clocks.current.graphics",
	"clocks.current.memory",
	"clocks.current.sm",
	"pstate",
	"pcie.link.gen.gpucurrent",
	"pcie.link.width.current",
	"ecc.errors.corrected.aggregate.total",
	"ecc.errors.uncorrected.aggregate.total",
}

// field index constants for readability.
const (
	fUUID = iota
	fName
	fDriverVersion
	fPCIBusID
	fVBIOSVersion
	fTempGPU
	fTempMemory
	fUtilGPU
	fUtilMemory
	fUtilEncoder
	fUtilDecoder
	fMemTotal
	fMemUsed
	fMemFree
	fPowerDraw
	fPowerLimit
	fClockGraphics
	fClockMemory
	fClockSM
	fPState
	fPCIEGen
	fPCIEWidth
	fECCCorrected
	fECCUncorrected
	fieldCount
)

// Collector runs nvidia-smi and produces Prometheus text output.
type Collector struct {
	nvidiaSMIPath string
	cacheTTL      time.Duration

	mu        sync.Mutex
	cached    string
	cachedAt  time.Time
}

// NewCollector creates a collector with the given nvidia-smi path and cache TTL.
func NewCollector(smiPath string, cacheTTL time.Duration) *Collector {
	return &Collector{
		nvidiaSMIPath: smiPath,
		cacheTTL:      cacheTTL,
	}
}

// Collect returns Prometheus text exposition format metrics.
func (c *Collector) Collect() (string, error) {
	if c.cacheTTL > 0 {
		c.mu.Lock()
		if c.cached != "" && time.Since(c.cachedAt) < c.cacheTTL {
			out := c.cached
			c.mu.Unlock()
			return out, nil
		}
		c.mu.Unlock()
	}

	raw, err := c.runNvidiaSMI()
	if err != nil {
		return "", err
	}

	out := c.format(raw)

	if c.cacheTTL > 0 {
		c.mu.Lock()
		c.cached = out
		c.cachedAt = time.Now()
		c.mu.Unlock()
	}

	return out, nil
}

// CheckNvidiaSMI verifies that nvidia-smi can be executed.
func (c *Collector) CheckNvidiaSMI() error {
	cmd := exec.Command(c.nvidiaSMIPath, "--list-gpus")
	return cmd.Run()
}

// runNvidiaSMI executes nvidia-smi and returns stdout.
func (c *Collector) runNvidiaSMI() (string, error) {
	query := strings.Join(queryFields, ",")
	cmd := exec.Command(c.nvidiaSMIPath, "--query-gpu="+query, "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("nvidia-smi: %w", err)
	}
	return string(out), nil
}

// format parses CSV output and produces Prometheus text.
func (c *Collector) format(raw string) string {
	var b strings.Builder

	gpus := parseCSV(raw)
	if len(gpus) == 0 {
		return ""
	}

	// gpu_stats_gpu_info
	writeHelp(&b, "gpu_stats_gpu_info", "GPU device information")
	writeType(&b, "gpu_stats_gpu_info", "gauge")
	for _, g := range gpus {
		fmt.Fprintf(&b, "gpu_stats_gpu_info{gpu_uuid=%q,gpu_name=%q,driver_version=%q,pci_bus_id=%q,vbios_version=%q} 1\n",
			g[fUUID], g[fName], g[fDriverVersion], g[fPCIBusID], g[fVBIOSVersion])
	}

	// temperature
	writeHelp(&b, "gpu_stats_temperature_celsius", "GPU temperature in Celsius")
	writeType(&b, "gpu_stats_temperature_celsius", "gauge")
	for _, g := range gpus {
		writeGaugeWithLabel(&b, "gpu_stats_temperature_celsius", g[fUUID], g[fName], "sensor", "gpu", g[fTempGPU])
		writeGaugeWithLabel(&b, "gpu_stats_temperature_celsius", g[fUUID], g[fName], "sensor", "memory", g[fTempMemory])
	}

	// utilization (percent -> ratio)
	writeHelp(&b, "gpu_stats_utilization_ratio", "GPU utilization as a ratio (0-1)")
	writeType(&b, "gpu_stats_utilization_ratio", "gauge")
	for _, g := range gpus {
		writeRatioWithLabel(&b, "gpu_stats_utilization_ratio", g[fUUID], g[fName], "type", "gpu", g[fUtilGPU])
		writeRatioWithLabel(&b, "gpu_stats_utilization_ratio", g[fUUID], g[fName], "type", "memory", g[fUtilMemory])
		writeRatioWithLabel(&b, "gpu_stats_utilization_ratio", g[fUUID], g[fName], "type", "encoder", g[fUtilEncoder])
		writeRatioWithLabel(&b, "gpu_stats_utilization_ratio", g[fUUID], g[fName], "type", "decoder", g[fUtilDecoder])
	}

	// memory bytes (MiB -> bytes)
	writeMemMetric(&b, "gpu_stats_memory_total_bytes", "Total GPU memory in bytes", gpus, fMemTotal)
	writeMemMetric(&b, "gpu_stats_memory_used_bytes", "Used GPU memory in bytes", gpus, fMemUsed)
	writeMemMetric(&b, "gpu_stats_memory_free_bytes", "Free GPU memory in bytes", gpus, fMemFree)

	// power
	writeSimpleMetric(&b, "gpu_stats_power_draw_watts", "GPU power draw in watts", gpus, fPowerDraw)
	writeSimpleMetric(&b, "gpu_stats_power_limit_watts", "GPU power limit in watts", gpus, fPowerLimit)

	// clocks
	writeHelp(&b, "gpu_stats_clock_speed_mhz", "GPU clock speed in MHz")
	writeType(&b, "gpu_stats_clock_speed_mhz", "gauge")
	for _, g := range gpus {
		writeGaugeWithLabel(&b, "gpu_stats_clock_speed_mhz", g[fUUID], g[fName], "type", "graphics", g[fClockGraphics])
		writeGaugeWithLabel(&b, "gpu_stats_clock_speed_mhz", g[fUUID], g[fName], "type", "memory", g[fClockMemory])
		writeGaugeWithLabel(&b, "gpu_stats_clock_speed_mhz", g[fUUID], g[fName], "type", "sm", g[fClockSM])
	}

	// pstate
	writeHelp(&b, "gpu_stats_pstate", "GPU performance state (P0=max, P12=min)")
	writeType(&b, "gpu_stats_pstate", "gauge")
	for _, g := range gpus {
		writePState(&b, g[fUUID], g[fName], g[fPState])
	}

	// PCIe
	writeSimpleMetric(&b, "gpu_stats_pcie_link_generation", "PCIe link generation", gpus, fPCIEGen)
	writeSimpleMetric(&b, "gpu_stats_pcie_link_width", "PCIe link width", gpus, fPCIEWidth)

	// ECC errors
	writeHelp(&b, "gpu_stats_ecc_errors_total", "GPU ECC error count")
	writeType(&b, "gpu_stats_ecc_errors_total", "gauge")
	for _, g := range gpus {
		writeGaugeWithLabel(&b, "gpu_stats_ecc_errors_total", g[fUUID], g[fName], "type", "corrected", g[fECCCorrected])
		writeGaugeWithLabel(&b, "gpu_stats_ecc_errors_total", g[fUUID], g[fName], "type", "uncorrected", g[fECCUncorrected])
	}

	return b.String()
}

// parseCSV splits nvidia-smi CSV output into rows of trimmed fields.
func parseCSV(raw string) [][]string {
	var result [][]string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < fieldCount {
			continue
		}
		row := make([]string, fieldCount)
		for i := 0; i < fieldCount; i++ {
			row[i] = strings.TrimSpace(parts[i])
		}
		result = append(result, row)
	}
	return result
}

// isNA returns true if the nvidia-smi field is not available.
func isNA(val string) bool {
	return val == "[N/A]" || val == "N/A" || val == "[Not Supported]" || val == "Not Supported" || val == ""
}

func writeHelp(b *strings.Builder, name, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
}

func writeType(b *strings.Builder, name, typ string) {
	fmt.Fprintf(b, "# TYPE %s %s\n", name, typ)
}

func writeGaugeWithLabel(b *strings.Builder, metric, uuid, name, labelKey, labelVal, rawVal string) {
	if isNA(rawVal) {
		return
	}
	v, err := strconv.ParseFloat(rawVal, 64)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "%s{gpu_uuid=%q,gpu_name=%q,%s=%q} %s\n",
		metric, uuid, name, labelKey, labelVal, formatFloat(v))
}

func writeRatioWithLabel(b *strings.Builder, metric, uuid, name, labelKey, labelVal, rawVal string) {
	if isNA(rawVal) {
		return
	}
	v, err := strconv.ParseFloat(rawVal, 64)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "%s{gpu_uuid=%q,gpu_name=%q,%s=%q} %s\n",
		metric, uuid, name, labelKey, labelVal, formatFloat(v/100.0))
}

func writeMemMetric(b *strings.Builder, metricName, help string, gpus [][]string, fieldIdx int) {
	writeHelp(b, metricName, help)
	writeType(b, metricName, "gauge")
	for _, g := range gpus {
		val := g[fieldIdx]
		if isNA(val) {
			continue
		}
		mib, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		bytes := mib * 1048576
		fmt.Fprintf(b, "%s{gpu_uuid=%q,gpu_name=%q} %s\n",
			metricName, g[fUUID], g[fName], formatFloat(bytes))
	}
}

func writeSimpleMetric(b *strings.Builder, metricName, help string, gpus [][]string, fieldIdx int) {
	writeHelp(b, metricName, help)
	writeType(b, metricName, "gauge")
	for _, g := range gpus {
		val := g[fieldIdx]
		if isNA(val) {
			continue
		}
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		fmt.Fprintf(b, "%s{gpu_uuid=%q,gpu_name=%q} %s\n",
			metricName, g[fUUID], g[fName], formatFloat(v))
	}
}

func writePState(b *strings.Builder, uuid, name, pstate string) {
	if isNA(pstate) {
		return
	}
	pstate = strings.TrimPrefix(pstate, "P")
	v, err := strconv.Atoi(pstate)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "gpu_stats_pstate{gpu_uuid=%q,gpu_name=%q} %d\n", uuid, name, v)
}

// formatFloat formats a float for Prometheus output, avoiding scientific notation.
func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
