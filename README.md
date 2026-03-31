# DropToDrop — Shopify Dropshipping Network

A production-ready Shopify app that connects suppliers with resellers for automated dropshipping. Suppliers list products, resellers import and sell them, orders route automatically, and fulfillment tracking syncs back — all within Shopify.

## Features

### Core Platform
- **Supplier Marketplace** — suppliers list products, resellers browse and import
- **One-Click Import** — import products with images, prices, and descriptions
- **Auto Order Routing** — customer orders automatically route to suppliers
- **Fulfillment Tracking** — tracking syncs from supplier to reseller's customer
- **Hourly Inventory Sync** — stock levels stay in sync automatically

### Business Tools
- **Smart Pricing AI** — suggests optimal retail prices per product tier
- **Bulk Import** — import 50+ products at once with batch markup
- **14 Product Categories** — organized marketplace browsing
- **Shipping Rules** — per-country rates with free shipping thresholds
- **Sample Orders** — try before you commit to importing

### Communication
- **In-App Messaging** — direct chat between supplier and reseller
- **Order Comments** — threaded comments on orders
- **Announcements** — suppliers broadcast to all resellers
- **Notifications** — real-time alerts for orders, messages, disputes

### Trust & Quality
- **Supplier Trust Score** — 0-100 reliability rating
- **Reviews & Ratings** — 1-5 star reviews with summaries
- **Dispute System** — report and resolve order issues
- **Supplier Verification** — verified badge system

### Analytics & Business
- **Dashboard** — real-time stats with auto-refresh
- **Analytics** — revenue, orders, product performance
- **Billing** — Free, Standard (€29), Premium (€79) plans
- **CSV Export** — download order data
- **Audit Logs** — full action history

### Deals & Promotions
- **Exclusive Deals** — time-limited supplier discounts
- **Marketplace Stock Allocation** — control % of inventory for resellers

## Architecture

| Component | Technology |
|-----------|-----------|
| Frontend | React + TypeScript + Shopify Polaris + App Bridge v4 |
| Backend | Go 1.23 + Gin framework |
| Database | PostgreSQL 16 (35+ tables, 10 migrations) |
| Cache/Queue | Redis 7 |
| API | REST (77+ endpoints) + Shopify GraphQL Admin API |
| Deployment | Scalingo (auto-deploy from GitHub) |
| Worker | 7 async job handlers, 6 queues |

## Quick Start

### Prerequisites
- Go 1.23+
- Node.js 22+
- Docker & Docker Compose
- Shopify Partner account

### 1. Clone and Configure
```bash
git clone https://github.com/arzumanmamishov/droptodrop.git
cd droptodrop
cp .env.example .env
# Edit .env with your Shopify app credentials
```

### 2. Start Infrastructure
```bash
docker compose up -d postgres redis
```

### 3. Start Development
```bash
# Backend
cd backend && go run ./cmd/server

# Frontend (separate terminal)
cd frontend && npm install && npm run dev

# Worker (separate terminal)
cd backend && go run ./cmd/worker
```

### 4. Install on Dev Store
```
https://your-app-url/auth/install?shop=your-store.myshopify.com
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SHOPIFY_API_KEY` | Shopify app Client ID |
| `SHOPIFY_API_SECRET` | Shopify app Client Secret |
| `SHOPIFY_APP_URL` | Public app URL |
| `SHOPIFY_REDIRECT_URI` | OAuth callback URL |
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_URL` | Redis connection string |
| `SESSION_SECRET` | Session signing key (32+ chars) |
| `ENCRYPTION_KEY` | AES-256 key for token encryption (32+ hex chars) |
| `SMTP_HOST` | Email server (optional) |
| `SMTP_USER` | Email username (optional) |
| `SMTP_PASS` | Email password (optional) |

## Billing Plans

| Plan | Price | Products | Orders | Fee |
|------|-------|----------|--------|-----|
| Free | €0 | 5 | 10/month | 0% |
| Standard | €29/month | Unlimited | Unlimited | 2% |
| Premium | €79/month | Unlimited | Unlimited | 0% |

## Security & Compliance

- OAuth 2.0 with HMAC verification
- App Bridge v4 JWT session tokens
- AES-256-GCM encrypted access tokens
- GDPR compliance webhooks (data request, redact, shop redact)
- Webhook HMAC-SHA256 verification
- Rate limiting with auto-cleanup
- CSP headers for embedded app security
- Audit logging for all actions

## License

Proprietary — All rights reserved.
