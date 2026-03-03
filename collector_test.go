package main

import (
	"strings"
	"testing"
)

// mockCSV simulates nvidia-smi output for 3x Tesla V100 GPUs.
const mockCSV = `GPU-aaaa-1111-2222-3333, Tesla V100-SXM2-16GB, 535.129.03, 00000000:3B:00.0, 88.00.7E.00.04, 42, 38, 15, 8, 0, 0, 16384, 1024, 15360, 55.23, 300.00, 1380, 877, 1380, P0, 3, 16, 0, 0
GPU-bbbb-4444-5555-6666, Tesla V100-SXM2-16GB, 535.129.03, 00000000:86:00.0, 88.00.7E.00.04, 45, 40, 92, 73, 55, 30, 16384, 14000, 2384, 285.50, 300.00, 1530, 877, 1530, P0, 3, 16, 2, 0
GPU-cccc-7777-8888-9999, Tesla V100-SXM2-32GB, 535.129.03, 00000000:AF:00.0, 88.00.7E.00.04, 38, 35, 0, 0, 0, 0, 32768, 512, 32256, 45.10, 300.00, 135, 877, 135, P2, 3, 16, 0, 1
`

func TestParseCSV(t *testing.T) {
	gpus := parseCSV(mockCSV)
	if len(gpus) != 3 {
		t.Fatalf("expected 3 GPUs, got %d", len(gpus))
	}

	// Check first GPU fields
	if gpus[0][fUUID] != "GPU-aaaa-1111-2222-3333" {
		t.Errorf("unexpected UUID: %s", gpus[0][fUUID])
	}
	if gpus[0][fName] != "Tesla V100-SXM2-16GB" {
		t.Errorf("unexpected name: %s", gpus[0][fName])
	}
	if gpus[0][fTempGPU] != "42" {
		t.Errorf("unexpected temp: %s", gpus[0][fTempGPU])
	}

	// Check third GPU (32GB)
	if gpus[2][fName] != "Tesla V100-SXM2-32GB" {
		t.Errorf("unexpected name for GPU 3: %s", gpus[2][fName])
	}
	if gpus[2][fMemTotal] != "32768" {
		t.Errorf("unexpected memory total for GPU 3: %s", gpus[2][fMemTotal])
	}
}

func TestParseCSVEmptyInput(t *testing.T) {
	gpus := parseCSV("")
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs for empty input, got %d", len(gpus))
	}
}

func TestParseCSVShortLine(t *testing.T) {
	gpus := parseCSV("too, few, fields\n")
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs for short line, got %d", len(gpus))
	}
}

func TestFormatOutput(t *testing.T) {
	c := NewCollector("/usr/bin/nvidia-smi", 0)
	out := c.format(mockCSV)

	// Check all 3 GPUs appear in gpu_info
	if strings.Count(out, "gpu_stats_gpu_info{") != 3 {
		t.Errorf("expected 3 gpu_info lines, got %d", strings.Count(out, "gpu_stats_gpu_info{"))
	}

	// Temperature
	if !strings.Contains(out, `gpu_stats_temperature_celsius{gpu_uuid="GPU-aaaa-1111-2222-3333",gpu_name="Tesla V100-SXM2-16GB",sensor="gpu"} 42`) {
		t.Error("missing GPU temperature for GPU 1")
	}
	if !strings.Contains(out, `sensor="memory"} 38`) {
		t.Error("missing memory temperature for GPU 1")
	}

	// Utilization ratio (15% -> 0.15)
	if !strings.Contains(out, `gpu_stats_utilization_ratio{gpu_uuid="GPU-aaaa-1111-2222-3333",gpu_name="Tesla V100-SXM2-16GB",type="gpu"} 0.15`) {
		t.Error("utilization ratio not correctly converted from percent to ratio")
	}

	// High utilization (92% -> 0.92)
	if !strings.Contains(out, `gpu_stats_utilization_ratio{gpu_uuid="GPU-bbbb-4444-5555-6666",gpu_name="Tesla V100-SXM2-16GB",type="gpu"} 0.92`) {
		t.Error("high utilization ratio not correctly converted")
	}

	// Memory bytes (16384 MiB -> 17179869184 bytes)
	if !strings.Contains(out, `gpu_stats_memory_total_bytes{gpu_uuid="GPU-aaaa-1111-2222-3333",gpu_name="Tesla V100-SXM2-16GB"} 17179869184`) {
		t.Error("memory total not correctly converted from MiB to bytes")
	}

	// 32GB GPU memory (32768 MiB -> 34359738368 bytes)
	if !strings.Contains(out, `gpu_stats_memory_total_bytes{gpu_uuid="GPU-cccc-7777-8888-9999",gpu_name="Tesla V100-SXM2-32GB"} 34359738368`) {
		t.Error("32GB GPU memory not correctly converted")
	}

	// Power
	if !strings.Contains(out, `gpu_stats_power_draw_watts{gpu_uuid="GPU-aaaa-1111-2222-3333",gpu_name="Tesla V100-SXM2-16GB"} 55.23`) {
		t.Error("power draw missing or incorrect")
	}

	// Clocks
	if !strings.Contains(out, `gpu_stats_clock_speed_mhz{gpu_uuid="GPU-aaaa-1111-2222-3333",gpu_name="Tesla V100-SXM2-16GB",type="graphics"} 1380`) {
		t.Error("clock speed missing or incorrect")
	}

	// PState (P0 -> 0, P2 -> 2)
	if !strings.Contains(out, `gpu_stats_pstate{gpu_uuid="GPU-aaaa-1111-2222-3333",gpu_name="Tesla V100-SXM2-16GB"} 0`) {
		t.Error("pstate P0 not converted to 0")
	}
	if !strings.Contains(out, `gpu_stats_pstate{gpu_uuid="GPU-cccc-7777-8888-9999",gpu_name="Tesla V100-SXM2-32GB"} 2`) {
		t.Error("pstate P2 not converted to 2")
	}

	// PCIe
	if !strings.Contains(out, `gpu_stats_pcie_link_generation{gpu_uuid="GPU-aaaa-1111-2222-3333"`) {
		t.Error("PCIe generation missing")
	}
	if !strings.Contains(out, `gpu_stats_pcie_link_width{gpu_uuid="GPU-aaaa-1111-2222-3333"`) {
		t.Error("PCIe width missing")
	}

	// ECC
	if !strings.Contains(out, `gpu_stats_ecc_errors_total{gpu_uuid="GPU-bbbb-4444-5555-6666",gpu_name="Tesla V100-SXM2-16GB",type="corrected"} 2`) {
		t.Error("ECC corrected errors missing or incorrect")
	}
	if !strings.Contains(out, `gpu_stats_ecc_errors_total{gpu_uuid="GPU-cccc-7777-8888-9999",gpu_name="Tesla V100-SXM2-32GB",type="uncorrected"} 1`) {
		t.Error("ECC uncorrected errors missing or incorrect")
	}

	// HELP and TYPE lines
	if !strings.Contains(out, "# HELP gpu_stats_gpu_info") {
		t.Error("missing HELP for gpu_info")
	}
	if !strings.Contains(out, "# TYPE gpu_stats_gpu_info gauge") {
		t.Error("missing TYPE for gpu_info")
	}
}

func TestFormatNAHandling(t *testing.T) {
	// Simulate [N/A] fields
	csvWithNA := "GPU-test-uuid, Test GPU, 535.00, 00:00.0, 88.00, 42, [N/A], 15, 8, [Not Supported], [Not Supported], 16384, 1024, 15360, 55.23, 300.00, 1380, 877, 1380, P0, 3, 16, N/A, N/A\n"
	c := NewCollector("/usr/bin/nvidia-smi", 0)
	out := c.format(csvWithNA)

	// Memory temp should be absent
	if strings.Contains(out, `sensor="memory"`) {
		t.Error("N/A memory temperature should be omitted")
	}

	// Encoder/decoder should be absent
	if strings.Contains(out, `type="encoder"`) {
		t.Error("[Not Supported] encoder utilization should be omitted")
	}
	if strings.Contains(out, `type="decoder"`) {
		t.Error("[Not Supported] decoder utilization should be omitted")
	}

	// ECC should be absent
	if strings.Contains(out, `gpu_stats_ecc_errors_total{gpu_uuid="GPU-test-uuid"`) {
		t.Error("N/A ECC errors should be omitted")
	}

	// Valid fields should still be present
	if !strings.Contains(out, `gpu_stats_temperature_celsius{gpu_uuid="GPU-test-uuid",gpu_name="Test GPU",sensor="gpu"} 42`) {
		t.Error("valid GPU temperature should be present")
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{42, "42"},
		{0, "0"},
		{55.23, "55.23"},
		{0.15, "0.15"},
		{17179869184, "17179869184"},
		{0.92, "0.92"},
	}
	for _, tt := range tests {
		got := formatFloat(tt.input)
		if got != tt.expected {
			t.Errorf("formatFloat(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsNA(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"[N/A]", true},
		{"N/A", true},
		{"[Not Supported]", true},
		{"Not Supported", true},
		{"", true},
		{"42", false},
		{"0", false},
		{"P0", false},
	}
	for _, tt := range tests {
		got := isNA(tt.input)
		if got != tt.expected {
			t.Errorf("isNA(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestFormatEmptyCSV(t *testing.T) {
	c := NewCollector("/usr/bin/nvidia-smi", 0)
	out := c.format("")
	if out != "" {
		t.Errorf("expected empty output for empty CSV, got %q", out)
	}
}
