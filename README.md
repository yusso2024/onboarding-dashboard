# Microservices Onboarding Dashboard

A production-grade microservices architecture built in Go for learning systems design patterns. Three independent services communicating through an API gateway and gRPC, with separate databases, Redis caching, circuit breakers, distributed tracing, and Prometheus/Grafana monitoring.

**Built**: April 22, 2026 | **Runtime**: Docker Compose on Ubuntu 24.04 LTS

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
                              │   (Observability)│
                              └─────────────────┘
```

## Systems Design Patterns Implemented

| Pattern | Where | Why |
|---------|-------|-----|
| **Database-per-Service** | Each service has its own DB | Schema isolation, independent scaling, blast radius containment |
| **API Gateway** | Traefik routes all external traffic | Single entry point, cross-cutting concerns, service discovery |
| **Cache-Aside** | User + Inventory services use Redis | 100x read-to-write ratio on profiles; sub-ms reads vs 5-10ms DB |
| **Circuit Breaker** | Inventory Service → Redis | Graceful degradation when Redis dies; fail fast, not slow |
| **Distributed Tracing** | OpenTelemetry + Jaeger across all services | Trace requests across service boundaries; find latency bottlenecks |
| **gRPC Inter-Service** | User Service → Inventory Service | Binary protocol, strict contracts via protobuf, 10x smaller than JSON |
| **Dependency Injection** | Handler structs receive DB/Redis clients | Testability, explicit dependencies, no hidden global state |
| **Health Checks** | Every service exposes /health | Docker healthchecks, load balancer readiness, dependency-aware |
| **Graceful Degradation** | Services work without Redis (slower) | Cache is an optimization, not a requirement |
| **Event-Driven Triggers** | Onboarding complete → gRPC starter pack | Eventual consistency, async processing, domain separation |
| **Network Segmentation** | frontend/backend/monitoring networks | Defense in depth; databases never exposed externally |
| **Resource Limits** | CPU/memory caps per container | Fair scheduling, OOM prevention on shared infrastructure |

## Quick Start

### Prerequisites
- Docker 29+ with Docker Compose v5+
- 4GB+ available RAM

### Start Everything
```bash
cd ~/onboarding-dashboard
cp .env.example .env  # Or create .env with the required variables
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

# 3. Complete onboarding (triggers gRPC → auto-assigns starter pack)
curl -s -X PATCH http://localhost:8100/api/users/onboarding \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"onboarding_step":5}'

# 4. View assigned assets
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
| PATCH | `/api/users/onboarding` | JWT | Advance onboarding step |
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
| Inventory | 4000 | `AssignStarterPack` | User completes onboarding |

## Deep Dive: Key Patterns

### Circuit Breaker (Inventory → Redis)

When Redis is down, instead of failing every request:

```
CLOSED (normal)     → Requests flow to Redis
         ↓ (5 consecutive failures)
OPEN (tripped)      → Skip Redis, go straight to MongoDB
         ↓ (after 30 seconds)
HALF-OPEN (testing) → Try ONE Redis request
         ↓ (success)              ↓ (failure)
CLOSED (recovered)       OPEN (still broken)
```

Health endpoint reports circuit breaker state:
```json
{"status": "healthy", "redis_circuit_breaker": "OPEN"}
```

The service stays functional — just slower (DB-only, no cache).

### gRPC Inter-Service Communication

```protobuf
service InventoryGrpc {
    rpc AssignStarterPack(AssignStarterPackRequest) returns (AssignStarterPackResponse);
}
```

User Service calls Inventory Service internally via gRPC when onboarding completes. This demonstrates:
- **Data ownership**: User Service doesn't write to inventory DB directly
- **Protobuf contracts**: Type-safe, binary, auto-generated client/server code
- **Async processing**: gRPC call runs in a goroutine; user gets immediate response

### Distributed Tracing (OpenTelemetry → Jaeger)

Every HTTP request gets a trace ID that follows it through:
```
Traefik → Auth Service → PostgreSQL
Traefik → User Service → PostgreSQL + Redis
Traefik → Inventory Service → MongoDB + Redis
```

View traces at Jaeger UI (`:16686`). Select a service, find a trace, see the waterfall breakdown of where time was spent.

## Chaos Testing

Run the chaos test suite:
```bash
./chaos/chaos-test.sh
```

### Test Results

| Test | Result | Finding |
|------|--------|---------|
| Service crash & recovery | ⚠️ | `docker compose kill` = admin stop, not crash. Real crashes trigger restart. |
| Database failure cascade | ✅ | User + Inventory unaffected when auth-db dies |
| Cache failure (Redis) | ✅ | Circuit breaker trips → services fall back to DB |
| Gateway SPOF | ✅ | Confirmed: gateway death = total external outage |
| Load test | ✅ | 7ms avg response time at 50 req burst |

## Technology Choices

| Choice | Why |
|--------|-----|
| **Go** | Kubernetes, Docker, Prometheus all written in Go. 5-10MB per service. Static typing enforces contracts. |
| **PostgreSQL** (auth, user) | Relational data with ACID guarantees. Referential integrity for users/profiles. |
| **MongoDB** (inventory) | Flexible schemas for varied asset types (VMs, docs, API keys). |
| **Redis** | Sub-millisecond reads for cache-aside pattern. LRU eviction at 64MB. |
| **Traefik** | Auto-discovery, native Prometheus metrics, OpenTelemetry tracing built-in. |
| **Jaeger** | Standard distributed tracing backend. Accepts OTLP protocol. |
| **JWT** | Stateless auth. Any service validates without calling auth service. |
| **bcrypt** | Intentionally slow hashing (250ms/hash). Brute-force resistant. |

## Operations

```bash
# Start
docker compose up -d --build

# Stop (keep data)
docker compose down

# Stop (delete all data)
docker compose down -v

# Logs
docker compose logs -f auth-service

# Rebuild one service
docker compose up -d --build inventory-service

# Resource usage
docker stats --no-stream
```

## Project Structure

```
onboarding-dashboard/
├── .env                                    # Environment variables (not committed)
├── .env.example                            # Template for .env
├── docker-compose.yml                      # All 11 containers orchestrated
├── README.md
├── proto/
│   └── inventory.proto                     # gRPC service contract
├── gateway/
│   └── traefik.yml                         # API routing config
├── services/
│   ├── auth/
│   │   ├── cmd/server/main.go              # Entry point
│   │   ├── internal/
│   │   │   ├── handler/auth.go             # Register, Login, Health
│   │   │   ├── middleware/jwt.go           # JWT generation + validation
│   │   │   ├── model/user.go              # User struct + DTOs
│   │   │   └── tracing/tracing.go         # OpenTelemetry init
│   │   ├── Dockerfile
│   │   ├── go.mod / go.sum
│   ├── user/
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── handler/user.go             # Profile CRUD + gRPC client
│   │   │   ├── model/profile.go
│   │   │   └── tracing/tracing.go
│   │   ├── proto/inventorypb/              # Generated gRPC client code
│   │   ├── Dockerfile
│   │   ├── go.mod / go.sum
│   └── inventory/
│       ├── cmd/server/main.go              # HTTP + gRPC dual server
│       ├── internal/
│       │   ├── handler/
│       │   │   ├── inventory.go            # REST handlers
│       │   │   └── grpc.go                 # gRPC handler
│       │   ├── model/asset.go
│       │   ├── circuitbreaker/breaker.go   # Circuit breaker implementation
│       │   └── tracing/tracing.go
│       ├── proto/inventorypb/              # Generated gRPC server code
│       ├── Dockerfile
│       ├── go.mod / go.sum
├── monitoring/
│   ├── prometheus.yml                      # Scrape config
│   └── grafana/dashboards/
└── chaos/
    └── chaos-test.sh                       # 5 failure scenario tests
```

## Future Improvements

- [ ] **Service Mesh** — Sidecar proxies for mTLS, retries, and traffic shaping without code changes
- [ ] **Rate Limiting** — Per-user request throttling at the gateway level
- [ ] **Database Migrations** — golang-migrate for versioned, reversible schema changes
- [ ] **Connection Reconnection** — Auto-reconnect to databases after DB restarts
- [ ] **Frontend Dashboard** — Minimal web UI for the onboarding flow
- [ ] **CI/CD Pipeline** — Automated build, test, and deploy on push
