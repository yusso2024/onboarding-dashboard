# Systems Design with Microservices — 5-Day Course

## Course Overview

A hands-on, project-based course that teaches systems design by building a production-grade microservices architecture from scratch. Students build the **Onboarding Dashboard** — a real application with 15 containers, 3 Go services, 5 databases/caches, an API gateway, service mesh, distributed tracing, and CI/CD pipeline.

**Prerequisite knowledge**: Basic programming (any language), basic command line usage. No prior systems design, Go, or Docker experience required.

**Lab environment**: Each student gets a Linux VM (Ubuntu 24.04, 4+ GB RAM, Docker pre-installed).

**Teaching approach**: Every concept is introduced through a problem ("what breaks without this?"), then solved hands-on. Theory is embedded in practice — never lecture-first.

---

## Day 1: Foundations — From Monolith to Microservices

### Morning (4 hours)

#### Slide Block 1: What is Systems Design? (45 min)
- **Slide 1**: Title — "Systems Design: Building Software That Survives Reality"
- **Slide 2**: The gap between "it works on my machine" and "it works for 10,000 users"
- **Slide 3**: What systems design interviews actually test (trade-offs, not memorization)
- **Slide 4**: The 4 pillars: Scalability, Reliability, Maintainability, Performance
- **Slide 5**: Real-world example — "Why did GitHub go down last week?" (incident analysis)
- **Slide 6**: Course roadmap — what we're building over 5 days (show final architecture diagram)

#### Slide Block 2: Monolith vs Microservices (60 min)
- **Slide 7**: The monolith — one codebase, one database, one deployment
- **Slide 8**: Monolith advantages (simplicity, debugging, transactions)
- **Slide 9**: When monoliths break — deployment coupling, team scaling, blast radius
- **Slide 10**: The microservices idea — independent services, independent databases, independent deployments
- **Slide 11**: Microservices tradeoffs — network complexity, data consistency, operational overhead
- **Slide 12**: Decision framework — "When should you NOT use microservices?"
- **Slide 13**: Our project: 3 services (Auth, User, Inventory) — why these boundaries?
- **Slide 14**: Domain-Driven Design in 5 minutes — bounded contexts

#### Slide Block 3: Containers & Docker (75 min)
- **Slide 15**: The deployment problem — "works on my machine" = different OS, different versions, different configs
- **Slide 16**: What a container actually is (isolated process, not a VM)
- **Slide 17**: Docker concepts — images, containers, volumes, networks
- **Slide 18**: Dockerfile anatomy — FROM, COPY, RUN, CMD
- **Slide 19**: Multi-stage builds — why build image != runtime image (1GB vs 15MB)
- **Slide 20**: Docker Compose — orchestrating multiple containers
- **Lab**: Students install nothing. Docker is pre-installed. Pull and run `hello-world`, then `nginx`. Explore `docker ps`, `docker logs`, `docker exec`.

### Afternoon (4 hours)

#### Slide Block 4: Hands-On — Project Setup (60 min)
- **Slide 21**: Project structure walkthrough — `cmd/`, `internal/`, `go.mod`
- **Slide 22**: Why Go for microservices (same language as Docker, K8s, Prometheus)
- **Slide 23**: Go in 30 minutes — just enough to read the code (structs, interfaces, error handling, goroutines)
- **Lab**: Create the project skeleton. Create `.env` file. Create `docker-compose.yml` with just PostgreSQL + Redis.

#### Slide Block 5: Hands-On — Auth Service (120 min)
- **Slide 24**: Authentication fundamentals — passwords, hashing, tokens
- **Slide 25**: Why bcrypt (not MD5/SHA) — intentional slowness defeats brute force
- **Slide 26**: JWT explained — header.payload.signature, stateless auth
- **Slide 27**: JWT tradeoffs — can't revoke, token size, clock skew
- **Slide 28**: API design — REST conventions (POST for create, GET for read)
- **Lab**: Build the Auth Service. Write model, handler, middleware, main.go. Test with curl. Register a user, get a token, decode it at jwt.io.

#### Day 1 Wrap-Up (30 min)
- **Slide 29**: Recap — monolith vs microservices, containers, auth patterns
- **Slide 30**: What breaks if we add more services to this monolith? (motivation for Day 2)
- **Homework**: Read about the 12-Factor App methodology

---

## Day 2: Building Services — Databases, APIs, and Data Ownership

### Morning (4 hours)

#### Slide Block 6: Database-per-Service Pattern (45 min)
- **Slide 31**: Shared database problem — schema coupling, permission sprawl, blast radius
- **Slide 32**: Database-per-service — each service owns its data
- **Slide 33**: SQL vs NoSQL decision framework (when to use which)
- **Slide 34**: Why PostgreSQL for Auth/User (relational, ACID, referential integrity)
- **Slide 35**: Why MongoDB for Inventory (flexible schemas, document model)
- **Slide 36**: Polyglot persistence — use the right database for each service's access pattern
- **Slide 37**: The data ownership rule — "If you need another service's data, call its API"

#### Slide Block 7: Hands-On — User Service (90 min)
- **Slide 38**: Separation of concerns — Auth owns credentials, User owns profiles
- **Slide 39**: Partial updates — pointer fields in Go (`*string` vs `string`)
- **Slide 40**: Data Transfer Objects — why request/response types differ from DB models
- **Lab**: Build the User Service. Create profile, get profile, advance onboarding. Test end-to-end: register -> create profile -> advance steps.

#### Slide Block 8: Hands-On — Inventory Service (90 min)
- **Slide 41**: Document databases — flexible schemas for varied asset types
- **Slide 42**: MongoDB BSON vs JSON — binary format, ObjectID, dual struct tags
- **Slide 43**: Atomic operations — `findOneAndUpdate` prevents race conditions
- **Slide 44**: REST filtering — query parameters for GET requests (cacheable, bookmarkable)
- **Lab**: Build the Inventory Service. Create assets with different metadata shapes. Filter by category. Assign assets atomically.

### Afternoon (4 hours)

#### Slide Block 9: API Gateway Pattern (60 min)
- **Slide 45**: Without a gateway — client needs to know every service address
- **Slide 46**: Gateway responsibilities — routing, rate limiting, auth, logging
- **Slide 47**: Path-based routing — `/api/auth/*` -> Auth, `/api/users/*` -> User
- **Slide 48**: Traefik — auto-discovery, native metrics, config-driven routing
- **Slide 49**: Gateway as single point of failure — what happens when it dies?
- **Lab**: Add Traefik to docker-compose. Configure routing. Test all 3 services through one URL.

#### Slide Block 10: Caching with Redis (60 min)
- **Slide 50**: Why cache? — read-heavy workloads, latency reduction
- **Slide 51**: Cache-aside pattern — check cache -> miss -> query DB -> populate cache
- **Slide 52**: Cache invalidation — "one of the two hard problems in CS"
- **Slide 53**: Delete vs update on write — why delete is safer
- **Slide 54**: Eviction policies — LRU, LFU, TTL. Why `allkeys-lru` for our use case
- **Slide 55**: The `X-Cache: HIT/MISS` header — observability for caching
- **Lab**: Add Redis caching to User and Inventory services. Observe HIT/MISS headers. Measure response time difference.

#### Slide Block 11: Docker Networking Deep Dive (60 min)
- **Slide 56**: Docker bridge networks — container DNS, isolation
- **Slide 57**: Network segmentation — frontend, backend, monitoring
- **Slide 58**: Why services use port 3000 internally without conflicts (network namespaces)
- **Slide 59**: Service discovery — Docker DNS resolves container names to IPs
- **Slide 60**: `depends_on` with health checks — startup ordering
- **Lab**: `docker compose up -d --build`. Verify all 10 containers. Test health endpoints. Explore Docker networks.

#### Day 2 Wrap-Up (30 min)
- **Slide 61**: Recap — 3 services, 3 databases, gateway, cache. Full stack running.
- **Slide 62**: "Everything works. Now what happens when things break?"

---

## Day 3: Observability — Monitoring, Tracing, and Debugging

### Morning (4 hours)

#### Slide Block 12: Why Observability Matters (45 min)
- **Slide 63**: The three pillars — Logs, Metrics, Traces
- **Slide 64**: Logs tell you WHAT happened. Metrics tell you HOW MUCH. Traces tell you WHERE.
- **Slide 65**: Without observability — "the API is slow" -> check 3 log files -> guess
- **Slide 66**: With observability — click one trace -> see exact bottleneck
- **Slide 67**: Google's Golden Signals — Latency, Traffic, Errors, Saturation

#### Slide Block 13: Monitoring with Prometheus + Grafana (90 min)
- **Slide 68**: Pull vs Push model — why Prometheus pulls (detects down services automatically)
- **Slide 69**: Metrics types — counters, gauges, histograms
- **Slide 70**: PromQL basics — `rate()`, `sum()`, `histogram_quantile()`
- **Slide 71**: Grafana dashboards — visualizing system health
- **Slide 72**: Health endpoints — why a service that's "up" but can't reach its DB is NOT healthy
- **Lab**: Configure Prometheus scraping. Open Grafana. Add Prometheus data source. Build a dashboard with request rate, error rate, and latency.

#### Slide Block 14: Distributed Tracing with Jaeger (90 min)
- **Slide 73**: The problem — a request touches 3 services. Which one is slow?
- **Slide 74**: Trace anatomy — trace ID, spans, parent-child relationships
- **Slide 75**: Context propagation — trace ID travels in HTTP headers (W3C TraceContext)
- **Slide 76**: OpenTelemetry — vendor-neutral standard (works with Jaeger, Datadog, Zipkin)
- **Slide 77**: `otelhttp.NewHandler` — automatic span creation per HTTP request
- **Slide 78**: Trace visualization — waterfall view in Jaeger
- **Lab**: Add OpenTelemetry to all 3 services. Generate traffic. Open Jaeger. Find a trace. Identify the slowest span.

### Afternoon (4 hours)

#### Slide Block 15: Chaos Engineering — Breaking Things on Purpose (120 min)
- **Slide 79**: Netflix Chaos Monkey — "the only way to know if recovery works is to test it"
- **Slide 80**: Types of failure — service crash, database death, cache failure, network partition
- **Slide 81**: Fault isolation — why separate databases prevent cascade
- **Slide 82**: Self-healing — `restart: unless-stopped`, health checks, automatic recovery
- **Slide 83**: MTTR vs MTTF — Mean Time To Recovery matters more than Mean Time To Failure
- **Lab**: Run `chaos-test.sh`. Watch services die and recover. Observe: which failures cascade? Which are isolated? Document findings.

#### Slide Block 16: Interpreting Chaos Results (60 min)
- **Slide 84**: Class discussion — what broke? What recovered? What surprised you?
- **Slide 85**: The chaos test scorecard — analyze as a group
- **Slide 86**: Identifying improvement areas — "Inventory returned 500 when Redis died. Is that acceptable?"
- **Slide 87**: Graceful degradation vs hard failure — design philosophy

#### Day 3 Wrap-Up (30 min)
- **Slide 88**: Recap — monitoring, tracing, chaos testing
- **Slide 89**: "We found bugs through chaos testing. Tomorrow we fix them."

---

## Day 4: Resilience — Circuit Breakers, gRPC, and Service Mesh

### Morning (4 hours)

#### Slide Block 17: Circuit Breaker Pattern (90 min)
- **Slide 90**: The problem — Redis is dead, but every request still tries to call it (slow failure)
- **Slide 91**: Electrical circuit breaker analogy — trip on overload, test before closing
- **Slide 92**: Three states — CLOSED (normal), OPEN (reject fast), HALF-OPEN (probe)
- **Slide 93**: Thresholds — how many failures before tripping? How long before probing?
- **Slide 94**: Implementation walkthrough — the `Breaker` struct, `Execute()` wrapper, thread safety with `sync.Mutex`
- **Slide 95**: Graceful degradation — cache miss is slow, not broken
- **Lab**: Implement circuit breaker in Inventory Service. Kill Redis. Verify: before = 500 errors. After = data from MongoDB (slower but functional).

#### Slide Block 18: gRPC and Protocol Buffers (90 min)
- **Slide 96**: REST limitations for inter-service calls — text-based, no strict contracts, manual serialization
- **Slide 97**: gRPC — binary protocol (protobuf), strict contracts (.proto files), code generation
- **Slide 98**: Proto file anatomy — `message`, `service`, `rpc`
- **Slide 99**: Code generation — `protoc` generates client AND server code automatically
- **Slide 100**: When to use gRPC vs REST — internal (gRPC) vs external (REST)
- **Slide 101**: Data ownership through APIs — User Service says "assign starter pack," Inventory Service decides what to assign
- **Slide 102**: Async processing — goroutine fires gRPC call, user gets immediate response (eventual consistency)
- **Lab**: Define proto file. Generate Go code. Add gRPC server to Inventory. Add gRPC client to User. Complete onboarding -> verify starter pack appears.

### Afternoon (4 hours)

#### Slide Block 19: Service Mesh with Envoy (120 min)
- **Slide 103**: The problem — we coded retries in Go. What if we add 10 more services?
- **Slide 104**: Infrastructure-level resilience — move retries, timeouts, circuit breaking to the proxy layer
- **Slide 105**: Sidecar pattern — proxy sits next to every service, intercepts all traffic
- **Slide 106**: Envoy — the data plane proxy behind Istio, Linkerd, AWS App Mesh
- **Slide 107**: Envoy config walkthrough — listeners, clusters, retry policies, circuit breakers
- **Slide 108**: Traffic flow comparison — before mesh vs after mesh
- **Slide 109**: `Server: envoy` header — proof that traffic flows through the mesh
- **Slide 110**: The key insight — ZERO Go code changes. All resilience at infrastructure level.
- **Lab**: Add 3 Envoy sidecar containers. Update Traefik routing through proxies. Verify `Server: envoy` header. Kill a service — watch Envoy retry 3 times.

#### Slide Block 20: Comparing Resilience Layers (60 min)
- **Slide 111**: Application vs Infrastructure resilience — when to use which
- **Slide 112**: The defense-in-depth stack: Gateway -> Mesh -> Application -> Database
- **Slide 113**: Class exercise: design a resilience strategy for a payment service
- **Slide 114**: Real-world architectures — how Netflix, Uber, and Google layer resilience

#### Day 4 Wrap-Up (30 min)
- **Slide 115**: Recap — circuit breaker, gRPC, service mesh
- **Slide 116**: "The system is resilient. But how do we ship changes safely?"

---

## Day 5: Production Readiness — CI/CD, Frontend, and Interview Prep

### Morning (4 hours)

#### Slide Block 21: CI/CD Pipeline (90 min)
- **Slide 117**: The deployment problem — manual builds, "forgot to test," broken main branch
- **Slide 118**: Continuous Integration — every push is automatically built and tested
- **Slide 119**: Continuous Delivery — every successful build is deployable
- **Slide 120**: GitHub Actions anatomy — triggers, jobs, steps, matrix strategy
- **Slide 121**: Our pipeline walkthrough:
  - lint-and-build (3x parallel) — `go vet` + `go build`
  - docker-build (3x parallel) — Dockerfile verification
  - frontend — nginx image build
  - compose-validate — YAML syntax check
- **Slide 122**: Matrix strategy — same workflow runs against auth, user, inventory
- **Slide 123**: Pipeline dependencies — `docker-build` waits for `lint-and-build`
- **Slide 124**: Artifacts and traceability — tagging images with git SHA
- **Lab**: Push a deliberate bug (typo in Go code). Watch CI fail. Fix it. Watch CI pass. Review the Actions UI.

#### Slide Block 22: Frontend Integration (60 min)
- **Slide 125**: Why minimal frontend? — the backend is the systems design lesson
- **Slide 126**: API-first design — the frontend is just a consumer of the API
- **Slide 127**: Single-page app architecture — one HTML page, JavaScript fetches data
- **Slide 128**: Authentication flow in the browser — JWT in localStorage, Authorization header
- **Lab**: Open the web dashboard. Register. Walk through onboarding. Watch starter pack appear.

#### Slide Block 23: Scaling Concepts (60 min)
- **Slide 129**: Horizontal vs vertical scaling
- **Slide 130**: Stateless services — why our services can scale horizontally (no local state)
- **Slide 131**: Database scaling — read replicas, sharding, connection pooling
- **Slide 132**: Load balancing — round robin, least connections, consistent hashing
- **Slide 133**: Kubernetes overview — what it adds beyond Docker Compose (self-healing, auto-scaling, rolling updates)
- **Slide 134**: From Docker Compose to Kubernetes — migration path

### Afternoon (4 hours)

#### Slide Block 24: Systems Design Interview Prep (120 min)
- **Slide 135**: The interview format — 45 minutes to design a system
- **Slide 136**: The framework: Requirements -> Estimation -> Design -> Deep Dive -> Tradeoffs
- **Slide 137**: Practice question 1: "Design a URL shortener" — apply what you learned
- **Slide 138**: Practice question 2: "Design a notification service" — microservices approach
- **Slide 139**: Practice question 3: "Design an e-commerce checkout" — database selection, caching, resilience
- **Slide 140**: Common patterns cheat sheet — every pattern from this project mapped to interview questions
- **Slide 141**: Vocabulary that matters — CAP theorem, eventual consistency, idempotency, backpressure
- **Class exercise**: Students pair up and mock-interview each other using the framework

#### Slide Block 25: Course Recap and Architecture Review (60 min)
- **Slide 142**: Final architecture diagram — annotated with every pattern
- **Slide 143**: Pattern-by-pattern review: what we built, why, and what we'd do differently at scale
- **Slide 144**: The complete request lifecycle — browser -> gateway -> mesh -> service -> DB -> cache -> response
- **Slide 145**: What we didn't cover (and where to learn it) — Kafka, service discovery, blue-green deployments, feature flags

#### Slide Block 26: What's Next (30 min)
- **Slide 146**: Extending the project — mTLS, rate limiting, Kubernetes migration
- **Slide 147**: Recommended resources — Designing Data-Intensive Applications (book), system-design-primer (GitHub)
- **Slide 148**: Certificate of completion / Q&A

---

## Appendix A: Slide Count Summary

| Day | Topic | Slides | Labs |
|-----|-------|:------:|:----:|
| 1 | Foundations: Monolith -> Microservices, Docker, Auth | 30 | 3 |
| 2 | Services: Databases, APIs, Gateway, Caching | 32 | 5 |
| 3 | Observability: Monitoring, Tracing, Chaos | 27 | 4 |
| 4 | Resilience: Circuit Breaker, gRPC, Service Mesh | 27 | 4 |
| 5 | Production: CI/CD, Frontend, Scaling, Interviews | 22 | 3 |
| **Total** | | **138** | **19** |

## Appendix B: Lab Environment Setup

Each student VM needs:
- Ubuntu 24.04 LTS
- Docker 29+ with Docker Compose v5+
- 4 GB RAM minimum (8 GB recommended)
- 20 GB disk
- Go 1.22+ (for local development)
- `git`, `curl`, `python3` (for JSON formatting)
- Internet access (for pulling Docker images)

### Pre-course Setup Script
```bash
# Run on each student VM before Day 1
sudo apt update && sudo apt install -y docker.io docker-compose-v2 golang-go git curl python3
sudo usermod -aG docker $USER
```

## Appendix C: Pattern-to-Interview Mapping

| Interview Question | Patterns From This Course |
|-------------------|--------------------------|
| "Design a user authentication system" | JWT, bcrypt, middleware, stateless tokens |
| "How would you handle a database failure?" | Circuit breaker, graceful degradation, health checks |
| "Design a system with multiple services" | API gateway, database-per-service, gRPC, service mesh |
| "How do you debug a slow API?" | Distributed tracing (Jaeger), Prometheus metrics, golden signals |
| "What happens when your cache goes down?" | Cache-aside, circuit breaker, graceful fallback |
| "How do you deploy without downtime?" | CI/CD pipeline, Docker multi-stage builds, health checks |
| "Design for 10x traffic increase" | Horizontal scaling, stateless services, connection pooling, Redis |
| "How do you test system reliability?" | Chaos engineering, fault injection, MTTR measurement |

## Appendix D: Daily Schedule Template

| Time | Activity |
|------|----------|
| 09:00 - 09:15 | Day opener / recap of previous day |
| 09:15 - 10:30 | Slide Block A (theory + discussion) |
| 10:30 - 10:45 | Break |
| 10:45 - 12:15 | Slide Block B (theory + hands-on) |
| 12:15 - 13:15 | Lunch |
| 13:15 - 14:45 | Lab session (guided) |
| 14:45 - 15:00 | Break |
| 15:00 - 16:30 | Lab session (independent) |
| 16:30 - 17:00 | Day wrap-up, Q&A, preview of tomorrow |
