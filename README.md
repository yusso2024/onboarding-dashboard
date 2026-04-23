# Microservices Onboarding Dashboard

A production-grade microservices architecture built in Go for learning systems design patterns. Three independent services communicating through an API gateway, Envoy service mesh, and gRPC, with a web UI, separate databases, Redis caching, circuit breakers, distributed tracing, Prometheus/Grafana monitoring, and a GitHub Actions CI/CD pipeline.

**Built**: April 22-23, 2026 | **Runtime**: Docker Compose on Ubuntu 24.04 LTS
**Repo**: [github.com/yusso2024/onboarding-dashboard](https://github.com/yusso2024/onboarding-dashboard)

---

## Architecture

```
                      ┌─────────────────┐
                      │     Browser      │
                      └────────┬────────┘
                               │ HTTP
                      ┌────────▼────────┐
                      │   API Gateway    │  :8100
                      │   (Traefik)      │  Dashboard :8101
                      └──┬──┬──┬─────┬──┘
                         │  │  │     │
                  ┌──────┘  │  │     └──────────┐
                  │         │  │                │
          ┌───────▼──┐ ┌───▼──▼───┐ ┌──────────▼──┐
          │ Frontend  │ │  Envoy   │ │   Envoy     │
          │ (nginx)   │ │ Sidecars │ │  Sidecars   │
          │ HTML/JS   │ │ retries  │ │  circuit    │
          └──────────┘ │ timeouts │ │  breaking   │
                       └──┬──┬───┘ └──────┬──────┘
                          │  │            │
                  ┌───────┘  │            │
                  │          │            │
          ┌───────▼───┐ ┌───▼────┐ ┌─────▼───────┐
          │Auth Service│ │  User  │ │  Inventory  │
          │  (Go)      │ │Service │ │  Service    │
          │  JWT+bcrypt│ │ (Go)   │ │  HTTP+gRPC  │
          └─────┬─────┘ └───┬────┘ └──────┬──────┘
                │            │    gRPC     │
          ┌─────▼─────┐ ┌───▼────┐ ┌──────▼──────┐
          │PostgreSQL  │ │Postgres│ │  MongoDB    │
          └───────────┘ └────────┘ └─────────────┘
                │            │            │
                └─────┬──────┴─────┬──────┘
                      │            │
              ┌───────▼───┐ ┌──────▼──────────┐
              │   Redis   │ │  Prometheus      │
              │  (Cache)  │ │  Grafana         │
              └───────────┘ │  Jaeger          │
                            └─────────────────┘
```

## Systems Design Patterns Implemented

| Pattern | Where | Why |
|---------|-------|-----|
| **Database-per-Service** | Each service owns its DB | Schema isolation, blast radius containment |
| **API Gateway** | Traefik routes all external traffic | Single entry point, path-based routing |
| **Service Mesh** | Envoy sidecar proxies | Retries, timeouts, circuit breaking — zero code changes |
| **Cache-Aside** | User + Inventory use Redis | Sub-ms reads, 99% DB load reduction |
| **Circuit Breaker** | App-level (Go) + Mesh-level (Envoy) | Graceful degradation, fail fast |
| **Distributed Tracing** | OpenTelemetry + Jaeger | Trace requests across service boundaries |
| **gRPC Inter-Service** | User Service calls Inventory | Binary protocol, strict protobuf contracts |
| **Sidecar Pattern** | Envoy proxy per service | Infrastructure concerns separated from business logic |
| **Event-Driven** | Onboarding complete triggers gRPC | Eventual consistency, async processing |
| **Network Segmentation** | frontend/backend/monitoring networks | Defense in depth |
| **CI/CD Pipeline** | GitHub Actions | Automated build, test, Docker image verification |

## Quick Start

### Prerequisites
- Docker 29+ with Docker Compose v5+
- 4GB+ available RAM

### Start Everything
```bash
cd ~/onboarding-dashboard
cp .env.example .env  # Edit passwords before production use
docker compose up -d --build
```

### Access Points

| Service | URL |
|---------|-----|
| **Web Dashboard** | `http://<host>:8100` |
| API Gateway | `http://<host>:8100/api/...` |
| Traefik Dashboard | `http://<host>:8101` |
| Prometheus | `http://<host>:9090` |
| Grafana | `http://<host>:3100` (admin/admin) |
| Jaeger (Tracing) | `http://<host>:16686` |

## Web UI

The frontend is a vanilla HTML/CSS/JS dashboard served through the Traefik gateway. No frameworks — just clean, minimal code.

### Features
- **Login/Register** — JWT-based authentication
- **Profile Card** — Display name and role
- **Onboarding Stepper** — 5-step progress tracker; step 5 triggers gRPC starter pack assignment
- **Inventory Table** — List, filter, and create assets
- **Health Indicators** — Live service health dots (auto-refresh every 15s)
- **Dark Theme** — Responsive layout

### Screenshots Flow
1. Open `http://<host>:8100` — see login/register form
2. Register with email/password + display name
3. Dashboard appears with profile, health dots, empty inventory
4. Click "Next Step" 4 times to advance onboarding
5. At step 5 — "Onboarding complete! Starter pack assigned"
6. Inventory table auto-populates with 3 assets (Dev VM, Guide, API Key)

## How to Use the App (API)

```bash
# Register
TOKEN=$(curl -s -X POST http://localhost:8100/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@company.com","password":"SecurePass123"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Create profile
curl -s -X POST http://localhost:8100/api/users/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"display_name":"Jane Smith","role":"engineer"}'

# Complete onboarding (triggers gRPC starter pack)
curl -s -X PATCH http://localhost:8100/api/users/onboarding \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"onboarding_step":5}'

# View assigned assets
curl -s http://localhost:8100/api/inventory/assets | python3 -m json.tool
```

## API Reference

### Auth Service (`/api/auth`)
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/auth/register` | No | Create account, receive JWT |
| POST | `/api/auth/login` | No | Login, receive JWT |
| GET | `/api/auth/health` | No | Health check |

### User Service (`/api/users`)
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/users/profile` | JWT | Create profile |
| GET | `/api/users/profile/me` | JWT | Get profile (cache-aside) |
| PATCH | `/api/users/onboarding` | JWT | Advance step (step 5 = gRPC trigger) |
| GET | `/api/users/health` | No | Health check |

### Inventory Service (`/api/inventory`)
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/inventory/assets` | No | List assets (?category= filter) |
| POST | `/api/inventory/assets` | JWT | Create asset |
| PATCH | `/api/inventory/assets/assign` | JWT | Assign asset |
| GET | `/api/inventory/health` | No | Health + circuit breaker state |

### Internal gRPC
| Service | Port | RPC | Trigger |
|---------|------|-----|---------|
| Inventory | 4000 | `AssignStarterPack` | Onboarding step 5 |

## CI/CD Pipeline

### How It Works

The GitHub Actions pipeline runs automatically on every push to `main` or pull request. It validates the entire stack in 4 parallel job groups:

```
Push to main
    │
    ├── lint-and-build (matrix: auth, user, inventory)   ← Go vet + compile
    │       │
    │       └── docker-build (matrix: auth, user, inventory)  ← Docker images
    │
    ├── frontend                                          ← nginx image
    │
    └── compose-validate                                  ← YAML syntax check
```

### Job Details

**1. lint-and-build** (runs 3x in parallel — one per service)
- `go mod download` — fetch dependencies
- `go vet ./...` — static analysis (catches bugs without running code)
- `go build -o /dev/null ./cmd/server` — compile check (output discarded)
- **Uses matrix strategy**: same workflow definition runs against auth, user, and inventory directories

**2. docker-build** (runs after lint-and-build passes)
- `docker build -t onboarding-<service>:<sha>` — builds the multi-stage Dockerfile
- Verifies the Dockerfile works, Go compiles inside the container, and the binary is produced
- Tags with git SHA for traceability

**3. frontend**
- Builds the nginx image with static HTML/CSS/JS
- Verifies Dockerfile and file copying work

**4. compose-validate**
- Copies `.env.example` to `.env` (secrets aren't in the repo)
- Runs `docker compose config --quiet` — validates YAML syntax, variable interpolation, and service dependencies

### Pipeline Status
View at: [github.com/yusso2024/onboarding-dashboard/actions](https://github.com/yusso2024/onboarding-dashboard/actions)

### Triggering Manually
```bash
gh workflow run ci.yml --ref main
```

## Deep Dive: Key Patterns

### Service Mesh (Envoy Sidecar Proxies)

```
Client -> Traefik -> Envoy auth-proxy -> Auth Service
                     ^
                     | Auto-retries (3x on 5xx)
                     | Timeouts (10s total, 3s per try)
                     | Circuit breaking (max 100 connections)
                     | Server: envoy header proves mesh routing
```

Config: `mesh/envoy-auth.yml`, `mesh/envoy-user.yml`, `mesh/envoy-inventory.yml`

### Circuit Breaker

Two layers:
1. **Application** (`internal/circuitbreaker/breaker.go`): CLOSED -> OPEN -> HALF-OPEN -> CLOSED
2. **Mesh** (Envoy config): Connection limits at proxy level

### gRPC Inter-Service

```protobuf
service InventoryGrpc {
    rpc AssignStarterPack(AssignStarterPackRequest) returns (AssignStarterPackResponse);
}
```

User Service calls Inventory via gRPC in a goroutine. User gets immediate HTTP response.

### Distributed Tracing

OpenTelemetry SDK exports spans to Jaeger. View at `:16686`.

## Chaos Testing

```bash
./chaos/chaos-test.sh
```

| Test | Result |
|------|--------|
| Service crash (Envoy retries 3x) | Verified |
| Database failure cascade | Fault isolation works |
| Redis failure | Circuit breaker trips, DB fallback |
| Gateway SPOF | Confirmed |

## Operations

```bash
docker compose up -d --build         # Start
docker compose down                  # Stop (keep data)
docker compose down -v               # Nuke everything
docker compose logs -f auth-service  # Logs
docker stats --no-stream             # Resources
```

## Container Inventory (15 total)

| Layer | Container | Purpose |
|-------|-----------|---------|
| UI | frontend | nginx serving HTML/CSS/JS |
| Gateway | gateway | Traefik path-based routing |
| Mesh | auth-proxy | Envoy sidecar |
| Mesh | user-proxy | Envoy sidecar |
| Mesh | inventory-proxy | Envoy sidecar |
| Service | auth-service | JWT auth, bcrypt |
| Service | user-service | Profiles, gRPC client |
| Service | inventory-service | Assets, gRPC server |
| Database | auth-db | PostgreSQL |
| Database | user-db | PostgreSQL |
| Database | inventory-db | MongoDB |
| Cache | redis | Cache-aside pattern |
| Monitoring | prometheus | Metrics |
| Monitoring | grafana | Dashboards |
| Tracing | jaeger | Distributed traces |

## Project Structure

```
onboarding-dashboard/
├── .github/workflows/ci.yml           # CI/CD pipeline
├── .env.example                        # Environment template
├── docker-compose.yml                  # 15 containers
├── README.md
├── docs/
│   └── course-syllabus.md              # 5-day systems design course
├── proto/
│   └── inventory.proto                 # gRPC contract
├── frontend/
│   ├── index.html                      # Dashboard UI
│   ├── style.css                       # Dark theme
│   ├── app.js                          # API client logic
│   └── Dockerfile                      # nginx-alpine
├── gateway/
│   └── traefik.yml                     # Routing (through mesh)
├── mesh/
│   ├── envoy-auth.yml                  # Auth sidecar
│   ├── envoy-user.yml                  # User sidecar
│   └── envoy-inventory.yml             # Inventory sidecar
├── services/
│   ├── auth/                           # JWT + bcrypt + PostgreSQL
│   ├── user/                           # Profiles + gRPC client
│   └── inventory/                      # Assets + gRPC server + circuit breaker
├── monitoring/
│   └── prometheus.yml                  # Scrapes services + Envoy
└── chaos/
    └── chaos-test.sh                   # 5 failure scenarios
```
