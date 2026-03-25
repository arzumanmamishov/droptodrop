#!/bin/bash
# =============================================================================
# DropToDrop — Local Development Setup Script
#
# This script:
# 1. Checks prerequisites
# 2. Generates secrets if .env doesn't exist
# 3. Starts PostgreSQL + Redis via Docker
# 4. Runs database migrations
# 5. Seeds demo data
# 6. Installs frontend dependencies
# 7. Prints next steps
#
# Usage: bash scripts/setup-local.sh
# =============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Navigate to project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

info "DropToDrop Local Setup"
echo "========================="
echo ""

# --- Check prerequisites ---
info "Checking prerequisites..."

command -v go >/dev/null 2>&1 || error "Go is not installed. Install from https://go.dev/dl/"
command -v node >/dev/null 2>&1 || error "Node.js is not installed. Install from https://nodejs.org/"
command -v docker >/dev/null 2>&1 || error "Docker is not installed. Install from https://docker.com/"
command -v npm >/dev/null 2>&1 || error "npm is not installed (comes with Node.js)."

GO_VERSION=$(go version | awk '{print $3}')
NODE_VERSION=$(node --version)
info "Go: $GO_VERSION"
info "Node: $NODE_VERSION"

# --- .env file ---
if [ ! -f .env ]; then
    info "Creating .env from .env.example..."
    cp .env.example .env

    # Generate secrets
    SESSION_SECRET=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 64)
    ENCRYPTION_KEY=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 64)

    # Replace placeholders
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/generate_a_32_byte_random_hex_string/$SESSION_SECRET/" .env
        sed -i '' "s/generate_a_32_byte_hex_key_for_aes256/$ENCRYPTION_KEY/" .env
    else
        sed -i "s/generate_a_32_byte_random_hex_string/$SESSION_SECRET/" .env
        sed -i "s/generate_a_32_byte_hex_key_for_aes256/$ENCRYPTION_KEY/" .env
    fi

    warn ".env created with generated secrets."
    warn "You MUST edit .env to add your Shopify API key and secret."
    echo ""
    echo "  Required fields to fill in:"
    echo "    SHOPIFY_API_KEY"
    echo "    SHOPIFY_API_SECRET"
    echo "    SHOPIFY_APP_URL       (your ngrok/tunnel URL)"
    echo "    SHOPIFY_REDIRECT_URI  (your ngrok/tunnel URL + /auth/callback)"
    echo "    VITE_SHOPIFY_API_KEY  (same as SHOPIFY_API_KEY)"
    echo "    VITE_APP_URL          (same as SHOPIFY_APP_URL)"
    echo ""
else
    info ".env already exists, skipping."
fi

# --- Docker infrastructure ---
info "Starting PostgreSQL and Redis..."
docker compose up -d postgres redis

# Wait for health checks
info "Waiting for services to be healthy..."
for i in $(seq 1 30); do
    PG_HEALTHY=$(docker compose ps postgres --format json 2>/dev/null | grep -c '"healthy"' || echo "0")
    REDIS_HEALTHY=$(docker compose ps redis --format json 2>/dev/null | grep -c '"healthy"' || echo "0")
    if [ "$PG_HEALTHY" -ge 1 ] && [ "$REDIS_HEALTHY" -ge 1 ]; then
        info "PostgreSQL and Redis are healthy."
        break
    fi
    if [ "$i" -eq 30 ]; then
        warn "Timed out waiting for services. They may still be starting."
    fi
    sleep 1
done

# --- Database migrations ---
info "Running database migrations..."
DATABASE_URL="postgres://droptodrop:droptodrop@localhost:5432/droptodrop?sslmode=disable"

# Install migrate if not present
if ! command -v migrate >/dev/null 2>&1; then
    info "Installing golang-migrate..."
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
fi

migrate -path backend/migrations -database "$DATABASE_URL" up 2>&1 || warn "Migrations may have already been applied."

# --- Seed data ---
info "Seeding demo data..."
docker compose exec -T postgres psql -U droptodrop -d droptodrop < scripts/seed.sql 2>&1 || warn "Seed data may already exist."

# --- Go dependencies ---
info "Downloading Go dependencies..."
cd backend
go mod download
cd ..

# --- Frontend dependencies ---
info "Installing frontend dependencies..."
cd frontend
npm install --silent
cd ..

# --- E2E dependencies ---
info "Installing E2E test dependencies..."
cd e2e
npm install --silent
cd ..

# --- Verify build ---
info "Verifying backend build..."
cd backend
go build -o /dev/null ./cmd/server 2>&1 && info "Backend server: OK"
go build -o /dev/null ./cmd/worker 2>&1 && info "Backend worker: OK"
cd ..

info "Verifying frontend build..."
cd frontend
VITE_SHOPIFY_API_KEY=test VITE_APP_URL=https://test.example.com npx tsc --noEmit 2>&1 && info "Frontend typecheck: OK"
cd ..

# --- Done ---
echo ""
echo "========================================="
info "Setup complete!"
echo "========================================="
echo ""
echo "Next steps:"
echo ""
echo "  1. Edit .env with your Shopify app credentials"
echo "     (see docs/shopify-partner-setup.md for full guide)"
echo ""
echo "  2. Start your tunnel:"
echo "     ngrok http 8080"
echo ""
echo "  3. Update SHOPIFY_APP_URL and SHOPIFY_REDIRECT_URI in .env"
echo "     with the tunnel URL"
echo ""
echo "  4. Start the services (3 terminals):"
echo ""
echo "     Terminal 1:  cd backend && go run ./cmd/server"
echo "     Terminal 2:  cd backend && go run ./cmd/worker"
echo "     Terminal 3:  cd frontend && npm run dev"
echo ""
echo "  5. Install on a dev store:"
echo "     https://YOUR_TUNNEL_URL/auth/install?shop=YOUR_STORE.myshopify.com"
echo ""
echo "  6. For testing without Shopify (using seed data):"
echo "     Open http://localhost:3000/?session=dev_supplier_session_token"
echo "     Open http://localhost:3000/?session=dev_reseller_session_token"
echo ""
