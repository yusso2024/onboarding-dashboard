# Microservices Onboarding Dashboard

A production-grade microservices architecture built in Go for learning systems design patterns. Three independent services communicating through an API gateway, Envoy service mesh, and gRPC, with separate databases, Redis caching, circuit breakers, distributed tracing, and Prometheus/Grafana monitoring.

**Built**: April 22-23, 2026 | **Runtime**: Docker Compose on Ubuntu 24.04 LTS
**Repo**: [github.com/yusso2024/onboarding-dashboard](https://github.com/yusso2024/onboarding-dashboard)

---

## Architecture

```
                      ┌─────────────────┐
                      │  Client/Browser  │
                      └────────┬────────┘
                               │ HTTP
                      ┌────────▼────────┐
                      │   API Gateway    │  :8100
                      │   (Traefik)      │  Dashboard :8101
                      └──┬─────┬─────┬──┘
                         │     │     │
             ┌───────────┘     │     └───────────┐
             │                 │                 │
     ┌───────▼───────┐ ┌──────▼──────┐ ┌────────▼──────┐
     │  Envoy Proxy   │ │ Envoy Proxy │ │  Envoy Proxy  │
     │  (auth-proxy)  │ │ (user-proxy)│ │  (inv-proxy)  │
     │  retries,      │ │ retries,    │ │  retries,     │
     │  timeouts,     │ │ timeouts,   │ │  timeouts,    │
     │  circuit break │ │ circuit br. │ │  circuit br.  │
     └───────┬───────┘ └──────┬──────┘ └────────┬──────┘
             │                 │                 │
     ┌───────▼───────┐ ┌──────▼──────┐ ┌────────▼──────┐
     │  Auth Service  │ │ User Service│ │Inventory Svc  │
     │  (Go, :3000)   │ │ (Go, :3000) │ │ HTTP :3000    │
     └───────┬───────┘ └──────┬──────┘ │ gRPC :4000    │
             │                │    gRPC └────────┬──────┘
     ┌───────▼───────┐ ┌──────▼──────┐ ┌────────▼──────┐
     │  PostgreSQL    │ │ PostgreSQL  │ │   MongoDB     │
     │  (auth_db)     │ │ (user_db)   │ │ (inventory_db)│
     └───────────────┘ └─────────────┘ └───────────────┘
             │                 │                 │
             └────────┬────────┴────────┬────────┘
                      │                 │
              ┌───────▼───────┐ ┌──────▼──────────┐
              │     Redis     │ │   Prometheus     │
              │   (Cache)     │ │   + Grafana      │
              └───────────────┘ │   + Jaeger       │
                                │  (Observability) │
                                └─────────────────┘
```

## Systems Design Patterns Implemented

| Pattern | Where | Why |
|---------|-------|-----|
| **Database-per-Service** | Each service has its own DB | Schema isolation, independent scaling, blast radius containment |
| **API Gateway** | Traefik routes all external traffic | Single entry point, cross-cutting concerns, service discovery |
| **Service Mesh** | Envoy sidecar proxies per service | Retries, timeouts, circuit breaking at infrastructure level — zero code changes |
| **Cache-Aside** | User + Inventory services use Redis | 100x read-to-write ratio on profiles; sub-ms reads vs 5-10ms DB |
| **Circuit Breaker** | Inventory Service (code) + Envoy (proxy) | Graceful degradation when dependencies fail; fail fast, not slow |
| **Distributed Tracing** | OpenTelemetry + Jaeger across all services | Trace requests across service boundaries; find latency bottlenecks |
| **gRPC Inter-Service** | User Service calls Inventory Service | Binary protocol, strict contracts via protobuf, 10x smaller than JSON |
| **Sidecar Pattern** | Envoy proxy next to each service | Infrastructure concerns separated from business logic |
| **Dependency Injection** | Handler structs receive DB/Redis clients | Testability, explicit dependencies, no hidden global state |
| **Health Checks** | Every service exposes /health | Docker healthchecks, load balancer readiness, dependency-aware |
| **Graceful Degradation** | Services work without Redis (slower) | Cache is an optimization, not a requirement |
| **Event-Driven Triggers** | Onboarding complete triggers gRPC starter pack | Eventual consistency, async processing, domain separation |
| **Network Segmentation** | frontend/backend/monitoring networks | Defense in depth; databases never exposed externally |
| **Resource Limits** | CPU/memory caps per container | Fair scheduling, OOM prevention on shared infrastructure |

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
| API Gateway | `http://<host>:8100/api/...` |
| Traefik Dashboard | `http://<host>:8101` |
| Prometheus | `http://<host>:9090` |
| Grafana | `http://<host>:3100` (admin/admin) |
| Jaeger (Tracing) | `http://<host>:16686` |

## How to Use the App

### Complete Onboarding Flow

```bash
# 1. Register
TOKEN=$(curl -s -X POST http://localhost:8100/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@company.com","password":"SecurePass123"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# 2. Create profile
curl -s -X POST http://localhost:8100/api/users/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"display_name":"Jane Smith","role":"engineer"}'

# 3. Complete onboarding (triggers gRPC -> auto-assigns starter pack)
curl -s -X PATCH http://localhost:8100/api/users/onboarding \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"onboarding_step":5}'

# 4. View assigned assets (3 starter pack items auto-created via gRPC)
curl -s http://localhost:8100/api/inventory/assets | python3 -m json.tool
```

When onboarding reaches step 5, the User Service calls the Inventory Service via gRPC to automatically assign a starter pack (Dev VM, Onboarding Guide, Staging API Key).

### API Reference

#### Auth Service (`/api/auth`)
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/auth/register` | No | Create account, receive JWT |
| POST | `/api/auth/login` | No | Login, receive JWT |
| GET | `/api/auth/health` | No | Health check |

#### User Service (`/api/users`)
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/users/profile` | JWT | Create profile |
| GET | `/api/users/profile/me` | JWT | Get profile (cache-aside) |
| PATCH | `/api/users/onboarding` | JWT | Advance onboarding (step 5 = gRPC trigger) |
| GET | `/api/users/health` | No | Health check |

#### Inventory Service (`/api/inventory`)
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/inventory/assets` | No | List assets (?category= filter) |
| POST | `/api/inventory/assets` | JWT | Create asset |
| PATCH | `/api/inventory/assets/assign` | JWT | Assign asset to user |
| GET | `/api/inventory/health` | No | Health + circuit breaker state |

#### Internal gRPC (not exposed via gateway)
| Service | Port | RPC | Trigger |
|---------|------|-----|---------|
| Inventory | 4000 | `AssignStarterPack` | User completes onboarding (step 5) |

## Deep Dive: Key Patterns

### Service Mesh (Envoy Sidecar Proxies)

Every request flows through an Envoy proxy before reaching the service:

```
Client -> Traefik -> Envoy auth-proxy -> Auth Service
                     ^
                     | Automatic retries (3x on 5xx)
                     | Timeouts (10s total, 3s per try)
                     | Circuit breaking (max 100 connections)
                     | Zero Go code changes
```

The `Server: envoy` response header proves traffic flows through the mesh.

**Why this matters**: Add a 4th service? Just add another Envoy sidecar config. No retry logic to code. No timeout handling to implement. The mesh handles it.

Config: `mesh/envoy-auth.yml`, `mesh/envoy-user.yml`, `mesh/envoy-inventory.yml`

### Circuit Breaker (Inventory -> Redis)

Two layers of circuit breaking:

1. **Application level** (`internal/circuitbreaker/breaker.go`): Custom Go implementation
   ```
   CLOSED -> OPEN (after 5 failures) -> HALF-OPEN (probe after 30s) -> CLOSED
   ```

2. **Mesh level** (Envoy config): Proxy-level connection limits
   ```yaml
   circuit_breakers:
     thresholds:
       - max_connections: 100
         max_pending_requests: 50
   ```

Health endpoint reports circuit breaker state:
```json
{"status": "healthy", "redis_circuit_breaker": "OPEN"}
```

### gRPC Inter-Service Communication

```protobuf
service InventoryGrpc {
    rpc AssignStarterPack(AssignStarterPackRequest) returns (AssignStarterPackResponse);
}
```

- **Data ownership**: User Service doesn't write to inventory DB directly
- **Protobuf contracts**: Type-safe, binary, auto-generated client/server code
- **Async processing**: gRPC call runs in a goroutine; user gets immediate response
- Proto definition: `proto/inventory.proto`

### Distributed Tracing (OpenTelemetry -> Jaeger)

Every HTTP request gets a trace ID that follows it through:
```
Traefik -> Envoy -> Auth Service -> PostgreSQL
Traefik -> Envoy -> User Service -> PostgreSQL + Redis
Traefik -> Envoy -> Inventory Service -> MongoDB + Redis
```

View traces at Jaeger UI (`:16686`). Select a service, find a trace, see the waterfall.

## Chaos Testing

```bash
./chaos/chaos-test.sh
```

| Test | Result | Finding |
|------|--------|---------|
| Service crash & recovery | ✅ | Envoy retries 3x automatically before returning 503 |
| Database failure cascade | ✅ | Fault isolation — other services unaffected |
| Cache failure (Redis) | ✅ | Circuit breaker trips, services fall back to DB |
| Gateway SPOF | ✅ | Confirmed single point of failure |
| Load test | ✅ | 7ms avg response through mesh |

## Technology Choices

| Choice | Why |
|--------|-----|
| **Go** | Kubernetes, Docker, Prometheus, Envoy ecosystem. 5-10MB per service. |
| **Envoy** | Same proxy behind Istio, Linkerd, AWS App Mesh. Industry standard. |
| **PostgreSQL** | ACID guarantees for auth/profile data. |
| **MongoDB** | Flexible schemas for varied asset types. |
| **Redis** | Sub-ms cache reads. LRU eviction at 64MB. |
| **Traefik** | Native Prometheus metrics, OpenTelemetry tracing. |
| **Jaeger** | Standard distributed tracing. Accepts OTLP. |
| **gRPC + Protobuf** | Binary protocol, strict contracts, code generation. |
| **JWT + bcrypt** | Stateless auth, brute-force resistant hashing. |

## Operations

```bash
# Start everything
docker compose up -d --build

# Stop (keep data)
docker compose down

# Stop (delete all data)
docker compose down -v

# Logs for specific service
docker compose logs -f auth-service

# Envoy proxy stats
curl http://localhost:8100/api/auth/health -v 2>&1 | grep Server
# Should show: Server: envoy

# Rebuild one service
docker compose up -d --build inventory-service

# Resource usage
docker stats --no-stream
```

## Project Structure

```
onboarding-dashboard/
├── .env.example                            # Environment template
├── docker-compose.yml                      # 14 containers orchestrated
├── README.md
├── proto/
│   └── inventory.proto                     # gRPC service contract (protobuf)
├── gateway/
│   └── traefik.yml                         # API routing (through mesh)
├── mesh/                                   # SERVICE MESH configs
│   ├── envoy-auth.yml                      # Auth sidecar (retries, timeouts, CB)
│   ├── envoy-user.yml                      # User sidecar
│   └── envoy-inventory.yml                 # Inventory sidecar
├── services/
│   ├── auth/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── handler/auth.go
│   │   │   ├── middleware/jwt.go
│   │   │   ├── model/user.go
│   │   │   └── tracing/tracing.go
│   │   └── Dockerfile
│   ├── user/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── handler/user.go             # Includes gRPC client
│   │   │   ├── model/profile.go
│   │   │   └── tracing/tracing.go
│   │   ├── proto/inventorypb/              # Generated gRPC client
│   │   └── Dockerfile
│   └── inventory/
│       ├── cmd/server/main.go              # HTTP + gRPC dual server
│       ├── internal/
│       │   ├── handler/
│       │   │   ├── inventory.go            # REST handlers
│       │   │   └── grpc.go                 # gRPC handler
│       │   ├── model/asset.go
│       │   ├── circuitbreaker/breaker.go
│       │   └── tracing/tracing.go
│       ├── proto/inventorypb/              # Generated gRPC server
│       └── Dockerfile
├── monitoring/
│   ├── prometheus.yml                      # Scrapes services + Envoy stats
│   └── grafana/dashboards/
└── chaos/
    └── chaos-test.sh                       # 5 failure scenario tests
```

## Container Inventory (14 total)

| Layer | Container | Image | Purpose |
|-------|-----------|-------|---------|
| Gateway | gateway | traefik:v3.2 | Path-based routing, metrics, tracing |
| Mesh | auth-proxy | envoyproxy/envoy:v1.31 | Sidecar: retries, timeouts, CB |
| Mesh | user-proxy | envoyproxy/envoy:v1.31 | Sidecar: retries, timeouts, CB |
| Mesh | inventory-proxy | envoyproxy/envoy:v1.31 | Sidecar: retries, timeouts, CB |
| Service | auth-service | Go binary | JWT auth, registration, login |
| Service | user-service | Go binary | Profiles, onboarding, gRPC client |
| Service | inventory-service | Go binary | Assets, gRPC server |
| Database | auth-db | postgres:17-alpine | Auth credentials |
| Database | user-db | postgres:17-alpine | User profiles |
| Database | inventory-db | mongo:7 | Asset documents |
| Cache | redis | redis:7-alpine | Cache-aside, token storage |
| Monitoring | prometheus | prom/prometheus | Metrics collection |
| Monitoring | grafana | grafana/grafana | Dashboards |
| Tracing | jaeger | jaegertracing/all-in-one | Distributed traces |

## Future Improvements

- [ ] **mTLS between services** — Envoy can terminate and originate TLS for encrypted inter-service traffic
- [ ] **Rate Limiting** — Per-user request throttling at gateway or mesh level
- [ ] **Database Migrations** — golang-migrate for versioned, reversible schema changes
- [ ] **Frontend Dashboard** — Minimal web UI for the onboarding flow
- [ ] **CI/CD Pipeline** — Automated build, test, and deploy on push
- [ ] **Kubernetes Migration** — Move from Docker Compose to K8s with Istio service mesh
