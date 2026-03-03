package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	port := envOrDefault("GPU_EXPORTER_PORT", "9835")
	metricsPath := envOrDefault("GPU_EXPORTER_METRICS_PATH", "/metrics")
	smiPath := envOrDefault("GPU_EXPORTER_NVIDIA_SMI_PATH", "/usr/bin/nvidia-smi")
	cacheTTLStr := envOrDefault("GPU_EXPORTER_CACHE_TTL", "0s")

	cacheTTL, err := time.ParseDuration(cacheTTLStr)
	if err != nil {
		log.Fatalf("invalid GPU_EXPORTER_CACHE_TTL %q: %v", cacheTTLStr, err)
	}

	collector := NewCollector(smiPath, cacheTTL)

	mux := http.NewServeMux()

	mux.HandleFunc(metricsPath, func(w http.ResponseWriter, r *http.Request) {
		out, err := collector.Collect()
		if err != nil {
			http.Error(w, "nvidia-smi error: "+err.Error(), http.StatusInternalServerError)
			log.Printf("ERROR collecting metrics: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprint(w, out)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := collector.CheckNvidiaSMI(); err != nil {
			http.Error(w, "nvidia-smi not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ready")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>GPU Stats Exporter</title></head>
<body>
<h1>GPU Stats Exporter</h1>
<p>Version: %s</p>
<p><a href="%s">Metrics</a></p>
<p><a href="/health">Health</a></p>
<p><a href="/ready">Ready</a></p>
</body></html>`, version, metricsPath)
	})

	addr := ":" + port
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("gpu_stats4prometheus %s listening on %s (metrics: %s)", version, addr, metricsPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
