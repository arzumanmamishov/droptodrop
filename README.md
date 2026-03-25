# DropToDrop - Shopify Dropshipping Network

A production-oriented Shopify public app enabling supplier-to-reseller dropshipping with automatic product sync, order routing, fulfillment tracking, pricing controls, and Shopify App Store review-safe compliance.

## Architecture

| Component | Technology |
|-----------|-----------|
| Frontend | React + TypeScript + Shopify Polaris + App Bridge |
| Backend | Go 1.22+ with Gin framework |
| Database | PostgreSQL 16 |
| Cache/Queue | Redis 7 |
| API | REST (internal) + Shopify GraphQL Admin API |
| Deployment | Docker + docker-compose |

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 22+
- Docker & Docker Compose
- A Shopify Partner account with a development app

### 1. Clone and Configure

```bash
git clone <repository-url>
cd droptodrop
cp .env.example .env
# Edit .env with your Shopify app credentials
```

### 2. Start Infrastructure

```bash
# Start PostgreSQL and Redis
docker compose up -d postgres redis

# Run migrations
export DATABASE_URL="postgres://droptodrop:droptodrop@localhost:5432/droptodrop?sslmode=disable"
migrate -path backend/migrations -database "$DATABASE_URL" up

# Seed demo data
psql "$DATABASE_URL" < scripts/seed.sql
```

### 3. Start Development Servers

```bash
# Terminal 1: Backend
cd backend && go run ./cmd/server

# Terminal 2: Worker
cd backend && go run ./cmd/worker

# Terminal 3: Frontend
cd frontend && npm install && npm run dev
```

### 4. Full Docker Setup

```bash
docker compose up --build
```

The app will be available at:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- Health check: http://localhost:8080/health

## Project Structure

```
droptodrop/
├── backend/
│   ├── cmd/
│   │   ├── server/          # HTTP server entry point
│   │   └── worker/          # Background worker entry point
│   ├── internal/
│   │   ├── auth/            # OAuth, session management, encryption
│   │   ├── audit/           # Audit logging service
│   │   ├── billing/         # Billing placeholder (Shopify-ready)
│   │   ├── compliance/      # GDPR/compliance webhook handlers
│   │   ├── config/          # Environment config loading
│   │   ├── database/        # PostgreSQL connection
│   │   ├── fulfillments/    # Fulfillment tracking service
│   │   ├── health/          # Health/readiness endpoints
│   │   ├── imports/         # Reseller product import service
│   │   ├── jobs/            # Background job worker
│   │   ├── logging/         # Structured logging
│   │   ├── middleware/      # Auth, CORS, rate limit, security
│   │   ├── orders/          # Order routing service
│   │   ├── products/        # Supplier listing service
│   │   ├── queue/           # Redis queue client
│   │   ├── shops/           # Shop & profile management
│   │   └── webhooks/        # Shopify webhook handlers
│   ├── migrations/          # PostgreSQL migrations
│   ├── pkg/
│   │   ├── hmac/            # HMAC verification (raw body)
│   │   ├── idempotency/     # Idempotency key management
│   │   ├── retry/           # Exponential backoff retry
│   │   └── shopify/         # Shopify API client & OAuth
│   └── tests/
│       ├── unit/            # Unit tests
│       └── integration/     # Integration tests
├── frontend/
│   └── src/
│       ├── components/      # Shared UI components
│       ├── hooks/           # React hooks (useApi, etc.)
│       ├── pages/           # Page components (9 screens)
│       ├── types/           # TypeScript type definitions
│       └── utils/           # API client, helpers
├── scripts/                 # Seed data, utilities
├── docs/                    # Documentation
├── docker-compose.yml
├── Makefile
└── .github/workflows/ci.yml
```

## API Endpoints

### Public
| Method | Path | Description |
|--------|------|-------------|
| GET | /health | Liveness check |
| GET | /health/ready | Readiness check (DB + Redis) |
| GET | /auth/install | Initiate Shopify OAuth |
| GET | /auth/callback | OAuth callback |

### Webhooks (HMAC verified)
| Method | Path | Topic |
|--------|------|-------|
| POST | /webhooks/app/uninstalled | app/uninstalled |
| POST | /webhooks/orders/create | orders/create |
| POST | /webhooks/fulfillments/create | fulfillments/create |
| POST | /webhooks/products/update | products/update |
| POST | /webhooks/products/delete | products/delete |
| POST | /webhooks/inventory/update | inventory_levels/update |
| POST | /webhooks/compliance/customers-data-request | customers/data_request |
| POST | /webhooks/compliance/customers-redact | customers/redact |
| POST | /webhooks/compliance/shop-redact | shop/redact |

### Authenticated API (Bearer token)
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | /api/v1/shop | Any | Get current shop |
| POST | /api/v1/shop/role | Any | Set shop role |
| GET | /api/v1/dashboard | Any | Dashboard data |
| GET | /api/v1/supplier/profile | Supplier | Get profile |
| PUT | /api/v1/supplier/profile | Supplier | Update profile |
| GET | /api/v1/supplier/listings | Supplier | List listings |
| POST | /api/v1/supplier/listings | Supplier | Create listing |
| PUT | /api/v1/supplier/listings/:id/status | Supplier | Change status |
| GET | /api/v1/supplier/orders | Supplier | List orders |
| POST | /api/v1/supplier/orders/:id/accept | Supplier | Accept order |
| POST | /api/v1/supplier/orders/:id/reject | Supplier | Reject order |
| POST | /api/v1/supplier/orders/:id/fulfill | Supplier | Add fulfillment |
| GET | /api/v1/reseller/marketplace | Reseller | Browse marketplace |
| POST | /api/v1/reseller/imports | Reseller | Import product |
| GET | /api/v1/reseller/imports | Reseller | List imports |
| POST | /api/v1/reseller/imports/:id/resync | Reseller | Re-sync import |
| GET | /api/v1/reseller/orders | Reseller | List orders |
| GET | /api/v1/orders/:id | Any | Order detail |
| GET | /api/v1/settings | Any | Get settings |
| PUT | /api/v1/settings | Any | Update settings |
| GET | /api/v1/billing | Any | Billing status |
| GET | /api/v1/audit | Any | Audit log |

## Testing

```bash
# Unit tests
cd backend && go test ./tests/unit/... -v

# Integration tests (requires DATABASE_URL)
cd backend && go test ./tests/integration/... -v

# Frontend tests
cd frontend && npm test

# All tests
make test
```

## Deployment

See [docs/deployment.md](docs/deployment.md) for production deployment instructions.

## Environment Variables

See [.env.example](.env.example) for all required variables.

## License

Proprietary - All rights reserved.
