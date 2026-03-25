# Deployment Guide

## Supported Platforms

This app is containerized and deployable to:
- Railway
- Render
- Fly.io
- AWS (ECS/Fargate/EKS)
- GCP (Cloud Run/GKE)

## Prerequisites

1. PostgreSQL 16+ instance
2. Redis 7+ instance
3. HTTPS domain with valid SSL certificate
4. Shopify Partner account with app configured

## Environment Setup

1. Copy `.env.example` to your deployment platform's environment variable configuration
2. Set all required variables (see .env.example for full list)
3. Ensure `SHOPIFY_APP_URL` matches your HTTPS domain
4. Generate secure values for `SESSION_SECRET`, `ENCRYPTION_KEY`

## Database Migration

```bash
# Using golang-migrate
migrate -path backend/migrations -database "$DATABASE_URL" up
```

## Docker Build

```bash
# Build backend
docker build -t droptodrop-backend ./backend

# Build frontend
docker build -t droptodrop-frontend ./frontend
```

## Railway Deployment

1. Connect your repository
2. Add PostgreSQL and Redis services
3. Set environment variables
4. Deploy backend with `./backend` as root directory
5. Deploy frontend with `./frontend` as root directory
6. Run migration as a one-off job

## Fly.io Deployment

```bash
# Backend
cd backend
fly launch
fly secrets set SHOPIFY_API_KEY=... SHOPIFY_API_SECRET=...
fly deploy

# Frontend
cd frontend
fly launch
fly deploy
```

## Health Checks

Configure your platform to use:
- Liveness: `GET /health` (returns 200)
- Readiness: `GET /health/ready` (checks DB + Redis)

## Scaling

- **Backend**: Stateless, scale horizontally
- **Worker**: Scale workers by increasing `WORKER_CONCURRENCY` or running multiple instances
- **Frontend**: Static files, serve from CDN or any web server
- **Database**: Use connection pooling (PgBouncer) for high concurrency

## Rollback

```bash
# Database: rollback one migration
migrate -path backend/migrations -database "$DATABASE_URL" down 1

# Application: redeploy previous container image
```

## Monitoring

- Structured JSON logs to stdout (production mode)
- Health endpoints for monitoring integration
- Audit logs in database for business event tracking
- Recommended: Connect to Datadog, New Relic, or Grafana for observability
