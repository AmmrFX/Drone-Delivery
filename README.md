# Drone Delivery API

A production-ready backend service for managing autonomous drone delivery operations. Built with Go, it orchestrates the full lifecycle of delivery orders — from placement through drone assignment, pickup, and completion — with built-in resilience patterns, geofencing, and role-based access control.

**Live API:** https://drone-delivery-3q0y.onrender.com
**Local API:** http://localhost:8080

## Table of Contents

- [Architecture](#architecture)
- [Domain Model](#domain-model)
- [Tech Stack](#tech-stack)
- [API Endpoints](#api-endpoints)
- [Resilience Patterns](#resilience-patterns)
- [Getting Started](#getting-started)
- [Testing](#testing)
- [Deployment](#deployment)

## Architecture

The project follows **Clean Architecture** with **Domain-Driven Design** principles:

```
cmd/server/          Entry point, routes, dependency wiring
internal/
  order/             Order aggregate (model, handler, service, repository)
  drone/             Drone aggregate (model, handler, service, repository)
  job/               Job aggregate (model, repository)
  delivery/          Orchestration domain — cross-aggregate transactions
  auth/              Token generation service
  jwt/               JWT signing and validation
  middleware/        Auth, rate limiter, bulkhead, idempotency, recovery
  common/            Shared types (Location, Mapbox client)
  errors/            Domain error types
  redis/             Caches (drone location, idempotency, rate limiting)
  repo/postgres/     Database connection and migrations
tests/
  unit/              Aggregate state-machine tests, geofence, ETA
  integration/       Full API lifecycle tests
```

Key architectural decisions:

- **Aggregate roots** (Order, Drone, Job) encapsulate their own state-machine logic and enforce valid transitions
- A **Delivery orchestration layer** wraps cross-aggregate workflows in database transactions to guarantee consistency
- **Repository pattern** with `sqlx.Tx` threading enables atomic multi-table operations
- Infrastructure concerns (caching, auth, rate limiting) live outside the domain layer

## Domain Model

### Order Lifecycle

```
PENDING ──→ ASSIGNED ──→ PICKED_UP ──→ DELIVERED
   │            │             │
   │            └─────────────┤
   ↓                          ↓
WITHDRAWN              AWAITING_HANDOFF ──→ ASSIGNED (new drone)
                              │
                         PICKED_UP ──→ FAILED
```

### Drone States

```
IDLE ──→ EN_ROUTE_PICKUP ──→ EN_ROUTE_DELIVERY ──→ IDLE
  ↑                                                   │
  └─── BROKEN ←──────── (any state) ─────────────────┘
```

### Cross-Aggregate Transactions (Delivery Domain)

| Operation | What happens atomically |
|---|---|
| `CreateOrderAndJob` | Insert order + create open job |
| `CancelOrderAndJob` | Withdraw order + cancel job |
| `ReserveJobAndAssign` | Reserve job + assign order to drone + reserve drone |
| `GrabOrder` | Mark order picked up + transition drone to delivering |
| `CompleteDelivery` | Mark delivered/failed + idle drone + complete job |
| `HandleDroneBroken` | Mark drone broken + await handoff + cancel old job + create new job |

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP Framework | Gin |
| Database | PostgreSQL (sqlx, golang-migrate) |
| Cache | Redis (go-redis) |
| Auth | JWT (HMAC-SHA256) |
| External API | Mapbox Directions (route ETA) |
| Deployment | Docker, Render |

## API Endpoints

### Authentication

```
POST /auth/token          Generate JWT (params: name, role)
```

Roles: `enduser`, `drone`, `admin`

### Enduser — Orders

```
POST   /orders            Place an order (origin + destination coordinates)
GET    /orders            List my orders
GET    /orders/:id        Get order details with ETA
DELETE /orders/:id        Withdraw a pending order
```

### Drone — Jobs & Delivery

```
POST  /drone/me/heartbeat       Report current location
GET   /drone/jobs                List open jobs
POST  /drone/jobs/reserve        Reserve a job
GET   /drone/me/order            Get current assigned order
POST  /drone/orders/:id/grab     Confirm pickup
PATCH /drone/orders/:id/complete Mark delivered or failed
POST  /drone/me/broken           Report drone malfunction
```

### Admin — Management

```
GET   /admin/orders              List all orders (paginated, filterable by status)
PATCH /admin/orders/:id          Update order locations
GET   /admin/drones              List all drones (paginated, filterable by status)
PATCH /admin/drones/:id/status   Mark drone broken or fixed
```

### Health

```
GET /health
```

## Resilience Patterns

| Pattern | Implementation | Purpose |
|---|---|---|
| **Rate Limiting** | Redis-backed token bucket (100 req/60s per IP) | Prevent abuse; fails open if Redis is down |
| **Bulkhead** | Semaphore pools — heartbeat(100), mutation(50), admin(20) | Isolate workloads and bound concurrency |
| **Idempotency** | `Idempotency-Key` header, Redis-cached responses (300s TTL) | Safe retries for all mutation endpoints |
| **Geofencing** | Haversine distance check against Riyadh zone (50 km radius) | Reject out-of-zone orders and heartbeats |
| **ETA Calculation** | Cached drone location + Haversine distance at 50 km/h | Real-time delivery estimates |
| **Graceful Shutdown** | Context cancellation with configurable timeout | Clean connection draining |

## Getting Started

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- (Optional) Mapbox access token for route-based ETA

### Run locally

```bash
# 1. Clone and enter the repo
git clone <repo-url> && cd Drone-Delivery

# 2. Copy env and fill in values
cp .env.example .env

# 3. Start everything (Postgres + Redis + API)
docker compose up --build
```

The API will be available at `http://localhost:8080`.

### Quick smoke test

```bash
# Use the live API or local
BASE=https://drone-delivery-3q0y.onrender.com
# BASE=http://localhost:8080

# Health check
curl $BASE/health

# Get a token
curl -X POST $BASE/auth/token \
  -d "name=alice&role=enduser"

# Place an order (Riyadh coordinates)
curl -X POST $BASE/orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"origin":{"lat":24.7136,"lng":46.6753},"destination":{"lat":24.80,"lng":46.70}}'
```

A full request collection is available in [api.http](api.http) for VS Code REST Client.

## Testing

### Unit tests — domain aggregate state machines, geofencing, ETA

```bash
go test ./tests/unit/...
```

### Integration tests — full API lifecycle against real Postgres & Redis

```bash
# Ensure Postgres and Redis are running
docker compose up -d postgres redis

go test ./tests/integration/...
```

**Coverage includes:**
- Order placement, listing, withdrawal, and detail retrieval
- Full delivery lifecycle (place -> reserve -> grab -> complete)
- Drone broken mid-delivery with automatic handoff to new drone
- Auth flows and role-based access enforcement
- Admin order/drone management with pagination

## Deployment

The project ships with a [render.yaml](render.yaml) for one-click deployment on Render:

1. Connect the repo to Render
2. Set environment variables: `DATABASE_URL`, `REDIS_URL`, `MAPBOX_ACCESS_TOKEN`
3. `JWT_SECRET` is auto-generated

The [Dockerfile](Dockerfile) produces a minimal Alpine-based image via multi-stage build.
