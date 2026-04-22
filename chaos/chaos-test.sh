#!/bin/bash
# ============================================
# Chaos Testing Script for Onboarding Dashboard
# ============================================
#
# WHY chaos testing?
# Netflix invented "Chaos Monkey" because they learned that
# the only way to know if your system handles failure is to
# CAUSE failure intentionally, in a controlled way.
#
# Hoping things won't break is not a strategy.
# Proving things recover from breakage IS a strategy.

GATEWAY="http://localhost:8100"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "==========================================="
echo " Onboarding Dashboard — Chaos Test Suite"
echo "==========================================="
echo ""

# Helper: check if a service responds healthy
check_health() {
    local service=$1
    local url=$2
    local response=$(curl -s -o /dev/null -w "%{http_code}" "$url")
    if [ "$response" = "200" ]; then
        echo -e "  ${GREEN}✓ $service is HEALTHY${NC}"
        return 0
    else
        echo -e "  ${RED}✗ $service is DOWN (HTTP $response)${NC}"
        return 1
    fi
}

# ---- Baseline Check ----
echo "PHASE 0: Baseline Health Check"
echo "-------------------------------"
check_health "Auth Service"      "$GATEWAY/api/auth/health"
check_health "User Service"      "$GATEWAY/api/users/health"
check_health "Inventory Service" "$GATEWAY/api/inventory/health"
echo ""

# =========================================
# TEST 1: Kill a service, observe recovery
# =========================================
# WHY this test?
# Docker's "restart: unless-stopped" should automatically restart
# a crashed container. This test proves it actually works.
# The systems design concept: SELF-HEALING INFRASTRUCTURE.
#
# Expected behavior:
# 1. Service dies → health check fails immediately
# 2. Docker detects the exit → restarts the container
# 3. Service comes back → health check passes again
# Recovery time tells you your MTTR (Mean Time To Recovery).

echo "==========================================="
echo "TEST 1: Service Crash & Recovery"
echo "==========================================="
echo "Killing auth-service container..."
docker compose kill auth-service
sleep 2

echo "Checking health (should be DOWN):"
check_health "Auth Service" "$GATEWAY/api/auth/health"
echo ""

echo -e "${YELLOW}Waiting for Docker to restart the container (10s)...${NC}"
sleep 10

echo "Checking health (should be RECOVERED):"
check_health "Auth Service" "$GATEWAY/api/auth/health"

echo ""
echo "Other services should be UNAFFECTED (fault isolation):"
check_health "User Service"      "$GATEWAY/api/users/health"
check_health "Inventory Service" "$GATEWAY/api/inventory/health"
echo ""

# =========================================
# TEST 2: Kill a database, observe cascade
# =========================================
# WHY this test?
# When a database dies, the service that depends on it should:
# - Report itself as unhealthy (not pretend everything is fine)
# - NOT crash (graceful degradation)
# - Recover automatically when the DB comes back
#
# This tests the DEPENDENCY HEALTH CHECK pattern we built
# into every service's /health endpoint.

echo "==========================================="
echo "TEST 2: Database Failure & Cascade"
echo "==========================================="
echo "Killing auth-db (PostgreSQL)..."
docker compose kill auth-db
sleep 3

echo "Auth Service health (should report UNHEALTHY — DB is gone):"
curl -s "$GATEWAY/api/auth/health" | python3 -m json.tool 2>/dev/null || curl -s "$GATEWAY/api/auth/health"
echo ""

echo "Attempting login (should fail gracefully, not crash):"
curl -s -X POST "$GATEWAY/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"test"}' | python3 -m json.tool 2>/dev/null || \
  curl -s -X POST "$GATEWAY/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"test"}'
echo ""

echo "User & Inventory should be UNAFFECTED (separate databases):"
check_health "User Service"      "$GATEWAY/api/users/health"
check_health "Inventory Service" "$GATEWAY/api/inventory/health"
echo ""

echo -e "${YELLOW}Restarting auth-db...${NC}"
docker compose up -d auth-db
sleep 8

echo "Auth Service health (should auto-reconnect):"
check_health "Auth Service" "$GATEWAY/api/auth/health"
echo ""

# =========================================
# TEST 3: Kill Redis, test cache degradation
# =========================================
# WHY this test?
# Redis is a CACHE, not a primary store. When it dies:
# - Services should still work (just slower — hitting DB directly)
# - Health endpoints should report Redis as down
# - When Redis returns, caching resumes automatically
#
# This is GRACEFUL DEGRADATION — the system loses performance
# but not functionality. A common systems design interview topic.

echo "==========================================="
echo "TEST 3: Cache Failure (Redis)"
echo "==========================================="
echo "Killing Redis..."
docker compose kill redis
sleep 3

echo "Service health (should show Redis unhealthy):"
curl -s "$GATEWAY/api/auth/health" | python3 -m json.tool 2>/dev/null || curl -s "$GATEWAY/api/auth/health"
echo ""

echo "Attempting to list inventory (should still work — DB fallback):"
curl -s "$GATEWAY/api/inventory/assets" | python3 -m json.tool 2>/dev/null || curl -s "$GATEWAY/api/inventory/assets"
echo ""

echo -e "${YELLOW}Restarting Redis...${NC}"
docker compose up -d redis
sleep 5

echo "Services should recover:"
check_health "Auth Service"      "$GATEWAY/api/auth/health"
check_health "User Service"      "$GATEWAY/api/users/health"
check_health "Inventory Service" "$GATEWAY/api/inventory/health"
echo ""

# =========================================
# TEST 4: Gateway failure
# =========================================
# WHY this test?
# The API gateway is a SINGLE POINT OF FAILURE.
# If Traefik dies, ALL external traffic stops — even though
# services are perfectly healthy internally.
#
# Systems design lesson: every "single point" is a risk.
# In production, you'd have multiple gateway instances
# behind a cloud load balancer (ALB, NLB).

echo "==========================================="
echo "TEST 4: Gateway Single Point of Failure"
echo "==========================================="
echo "Killing the API Gateway..."
docker compose kill gateway
sleep 2

echo "Trying to reach services through gateway (should FAIL):"
check_health "Auth (via gateway)" "$GATEWAY/api/auth/health"
echo ""

echo "But services are still alive internally:"
docker compose exec user-service wget -qO- http://localhost:3000/api/users/health 2>/dev/null || echo "  (direct check requires wget/curl in container)"
echo ""

echo -e "${YELLOW}Restarting gateway...${NC}"
docker compose up -d gateway
sleep 3

echo "Gateway restored:"
check_health "Auth Service"      "$GATEWAY/api/auth/health"
check_health "User Service"      "$GATEWAY/api/users/health"
check_health "Inventory Service" "$GATEWAY/api/inventory/health"
echo ""

# =========================================
# TEST 5: Load test (simple)
# =========================================
# WHY this test?
# Sending many requests quickly tests:
# - Connection pool exhaustion
# - Resource limit enforcement (our 128M memory cap)
# - Response time under load
#
# This is a basic throughput test. In production, you'd use
# tools like k6, wrk, or Locust for proper load testing.

echo "==========================================="
echo "TEST 5: Basic Load Test (50 requests)"
echo "==========================================="
echo "Sending 50 rapid health checks..."
SUCCESS=0
FAIL=0
START=$(date +%s%N)

for i in $(seq 1 50); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" "$GATEWAY/api/auth/health")
    if [ "$CODE" = "200" ]; then
        SUCCESS=$((SUCCESS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
done

END=$(date +%s%N)
DURATION=$(( (END - START) / 1000000 ))

echo -e "  ${GREEN}Success: $SUCCESS${NC}  ${RED}Failed: $FAIL${NC}"
echo "  Total time: ${DURATION}ms"
echo "  Avg response: $((DURATION / 50))ms per request"
echo ""

# ---- Final Status ----
echo "==========================================="
echo "FINAL: System Status After All Chaos Tests"
echo "==========================================="
check_health "Auth Service"      "$GATEWAY/api/auth/health"
check_health "User Service"      "$GATEWAY/api/users/health"
check_health "Inventory Service" "$GATEWAY/api/inventory/health"
echo ""
echo "==========================================="
echo " Chaos testing complete."
echo "==========================================="
