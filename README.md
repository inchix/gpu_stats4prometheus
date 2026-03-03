# gpu_stats4prometheus

Lightweight Prometheus exporter for NVIDIA GPU metrics. Zero external dependencies — built with Go stdlib only.

Designed as a replacement for third-party GPU exporters that fail to detect all GPUs. Targets Podman with CDI GPU passthrough.

## Quick Start

### Binary

```bash
go build -o gpu_stats4prometheus .
./gpu_stats4prometheus
curl http://localhost:9835/metrics
```

### Container (Podman + CDI)

```bash
podman build -t gpu_stats4prometheus .
podman run --rm -p 9835:9835 \
  --device nvidia.com/gpu=all \
  --read-only --cap-drop=ALL --security-opt=no-new-privileges \
  gpu_stats4prometheus
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `GPU_EXPORTER_PORT` | `9835` | Listen port |
| `GPU_EXPORTER_METRICS_PATH` | `/metrics` | Metrics endpoint path |
| `GPU_EXPORTER_NVIDIA_SMI_PATH` | `/usr/bin/nvidia-smi` | Path to nvidia-smi binary |
| `GPU_EXPORTER_CACHE_TTL` | `0s` | Cache duration (0 = fresh each scrape) |

## Endpoints

| Path | Description |
|------|-------------|
| `/metrics` | Prometheus metrics |
| `/health` | Liveness check (always 200) |
| `/ready` | Readiness check (verifies nvidia-smi) |
| `/` | Landing page |

## Metrics

All metrics are gauges labeled with `gpu_uuid` and `gpu_name`:

| Metric | Description |
|--------|-------------|
| `gpu_stats_gpu_info` | GPU info (labels: driver_version, pci_bus_id, vbios_version) |
| `gpu_stats_temperature_celsius` | Temperature (labels: sensor=gpu\|memory) |
| `gpu_stats_utilization_ratio` | Utilization 0-1 (labels: type=gpu\|memory\|encoder\|decoder) |
| `gpu_stats_memory_total_bytes` | Total GPU memory in bytes |
| `gpu_stats_memory_used_bytes` | Used GPU memory in bytes |
| `gpu_stats_memory_free_bytes` | Free GPU memory in bytes |
| `gpu_stats_power_draw_watts` | Current power draw |
| `gpu_stats_power_limit_watts` | Power limit |
| `gpu_stats_clock_speed_mhz` | Clock speed (labels: type=graphics\|memory\|sm) |
| `gpu_stats_pstate` | Performance state (P0=0, P12=12) |
| `gpu_stats_pcie_link_generation` | PCIe link generation |
| `gpu_stats_pcie_link_width` | PCIe link width |
| `gpu_stats_ecc_errors_total` | ECC errors (labels: type=corrected\|uncorrected) |

## Container Security

- Static Go binary (`CGO_ENABLED=0`, stripped)
- Runs as `nobody` (65534:65534)
- UBI9-micro base image
- Runtime: `--read-only --cap-drop=ALL --security-opt=no-new-privileges`

## Examples

See [`examples/`](examples/) for:
- `podman-run.sh` — quick-start script
- `gpu-exporter.container` — Podman quadlet for systemd
- `prometheus.yml` — Prometheus scrape config

## License

MIT
