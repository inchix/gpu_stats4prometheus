# Stage 1: Build
FROM docker.io/library/golang:1.22-bookworm AS builder

WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /gpu_stats4prometheus .

# Stage 2: Runtime
FROM registry.access.redhat.com/ubi9-micro:latest

COPY --from=builder /gpu_stats4prometheus /usr/local/bin/gpu_stats4prometheus

USER 65534:65534
EXPOSE 9835

ENTRYPOINT ["/usr/local/bin/gpu_stats4prometheus"]
