# ---- Build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Dependencies are vendored into ./vendor on the host (`go mod vendor`), so the
# in-container build needs NO network — avoids flaky module downloads.
COPY . .

# Static, stripped binary, built entirely from the vendored modules.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" -o /gateway ./cmd/gateway

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk --no-cache add ca-certificates \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app

# Binary and default config. In Kubernetes the config is overridden by a
# mounted ConfigMap; the baked-in copy keeps `docker run` working standalone.
COPY --from=builder /gateway /app/gateway
COPY configs/config.yaml /app/configs/config.yaml

USER app

EXPOSE 8080

# Liveness/readiness are served on /healthz and /readyz.
CMD ["/app/gateway"]
