# Go-Secure-Gateway

A lightweight API gateway written in Go (Gin) that fronts internal microservices
with JWT authentication, per-IP rate limiting, round-robin load balancing, and
per-route circuit breaking, and exposes Prometheus metrics.

## Features

- **JWT auth** (HS256 only, expiry required) on all proxied routes.
- **Per-IP rate limiting** with a token bucket and background eviction of idle IPs.
- **Reverse proxy** with optional prefix stripping and authenticated-user
  propagation via the `X-User-Id` header (client-supplied values are stripped to
  prevent spoofing).
- **Round-robin load balancing** across multiple backends per route (lock-free reads).
- **Circuit breaker** per route (closed → open → half-open with a single probe).
- **Observability**: `/metrics` (Prometheus), `/healthz` (liveness),
  `/readyz` (readiness, flips to 503 during graceful shutdown).
- **Graceful shutdown** on SIGINT/SIGTERM and hardened HTTP server timeouts.

## Documentation

- [`docs/project-structure.md`](docs/project-structure.md) — every directory/file
  and what it does, plus which files each test point maps to.
- [`docs/test-points.md`](docs/test-points.md) — module-by-module test points,
  with a testing-gap vs feature-gap classification.
- [`docs/openapi.yaml`](docs/openapi.yaml) — the gateway API contract.
- [`e2e/README.md`](e2e/README.md) — the black-box API test framework and how to
  extend it.

## Configuration

See [`configs/config.yaml`](configs/config.yaml). Key points:

- `debug: true` exposes `/debug/token` (mints a JWT) — **local development only**.
- `jwt.secret` can be overridden by the `JWT_SECRET` environment variable
  (preferred in production; sourced from a Kubernetes Secret).
- Each route supports either `target_url` (single) or `targets` (load-balanced
  list), plus an optional `strip_prefix`.
- `CONFIG_PATH` env var overrides the config file location (default `configs/config.yaml`).

## Run locally

```bash
go run ./cmd/gateway
```

Then mint a dev token (requires `debug: true`) and call a route:

```bash
TOKEN=$(curl -s localhost:8080/debug/token | sed 's/.*"token":"//;s/".*//')
curl -H "Authorization: Bearer $TOKEN" localhost:8080/interaction/ping
```

## Test

```bash
go test ./...
```

## End-to-end demo (docker-compose)

Spin up the gateway together with mock backends and a demo frontend:

```bash
docker compose up --build
```

- **Frontend**: <http://localhost:8088> — mint a token, call the routes, watch
  the `/storage` responses alternate between `storage-a` / `storage-b` (round-robin
  load balancing), and try a request without a token to see the `401`.
- **Gateway (direct)**: <http://localhost:8080>

The stack: `web` (nginx, serves the frontend and reverse-proxies `/api/*` to the
gateway on the same origin — no CORS needed) → `gateway` → `backend-*` mock echo
services. See [`docker-compose.yml`](docker-compose.yml) and
[`configs/config.docker.yaml`](configs/config.docker.yaml).

A quick CLI smoke test against the running stack:

```bash
make smoke
```

## Test target (controllable mock backends)

The backends behind the routes are **test doubles** designed for automated
testing — deterministic and fully steerable:

| Capability | How |
|---|---|
| Force any status | `?status=503` on any path (e.g. trip the circuit breaker on `/compute`) |
| Inject latency | `?delay=2s` on any path (e.g. exercise timeouts) |
| Create / read / update / delete | `POST/GET/PUT/DELETE /data/items[/{id}]` (in-memory) |
| Reset state before a test | `POST /data/_reset` |
| Inspect what the backend received | default echo response (path, `X-User-Id`, forwarded headers, serving replica) |

Route roles: `/interaction` (echo, no strip — auth/header tests), `/storage`
(2 replicas — load-balancing), `/compute` (fault injection — breaker/timeout),
`/data` (single backend — stateful CRUD).

The full API is described in [`docs/openapi.yaml`](docs/openapi.yaml) — import it
into Postman/Bruno or generate a client to bootstrap a test framework. A
module-by-module list of test points is in
[`docs/test-points.md`](docs/test-points.md).

## Monitoring (Prometheus + Grafana)

Opt-in via a compose profile (kept separate so the base demo stays light):

```bash
docker compose --profile monitoring up --build
```

- **Prometheus**: <http://localhost:9090> — scrapes the gateway's `/metrics`.
- **Grafana**: <http://localhost:3000> (login `admin` / `admin`) — the
  **Go-Secure-Gateway** dashboard is auto-provisioned (request rate by route,
  status-class breakdown, p95 latency, 5xx error ratio).

Generate some traffic (e.g. `make smoke` or clicking around the frontend) and
watch the panels populate.

## Logging

The `monitoring` profile also brings up **Loki + Promtail** (lightweight). Open
Grafana → Explore → the **Loki** datasource to search container logs
(e.g. `{service="gateway"}`) alongside your metrics — no extra tool needed.

A full **ELK** stack (Elasticsearch + Kibana + Filebeat) is provided as a
**standby alternative**, off by default. Enable it explicitly (needs ~1GB+ RAM):

```bash
docker compose --profile logging-elk up
```

Kibana is then at <http://localhost:5601>.

## Docker (single image)

```bash
docker build -t my-secure-gateway:v1.0 .
docker run -p 8080:8080 -e JWT_SECRET=change-me my-secure-gateway:v1.0
```

## CI/CD (GitHub Actions)

- [`ci.yml`](.github/workflows/ci.yml) — on every push/PR: gofmt check, `go vet`,
  build, race-enabled tests with coverage, golangci-lint, and Docker image builds.
- [`release.yml`](.github/workflows/release.yml) — on a `v*.*.*` tag: builds and
  pushes the gateway and mock images to GHCR (`ghcr.io/<owner>/<repo>` and
  `…-mock`).
- [`deploy.yml`](.github/workflows/deploy.yml) — manual (`workflow_dispatch`):
  applies the k8s manifests, creates the JWT secret from a repo secret, and rolls
  out the requested image tag. Requires `KUBE_CONFIG` (base64 kubeconfig) and
  `JWT_SECRET` repository secrets.

Common tasks are wrapped in the [`Makefile`](Makefile) — run `make help`.

## Kubernetes

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

The JWT secret is injected from the `gateway-secret` Secret; replace the sample
value before deploying.
