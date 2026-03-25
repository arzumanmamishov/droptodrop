#!/bin/bash
# =============================================================================
# DropToDrop — Start All Development Services
#
# Starts PostgreSQL, Redis, backend server, worker, and frontend dev server.
# Requires .env to be configured and Docker to be running.
#
# Usage: bash scripts/start-dev.sh
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $1"; }

# Check .env
if [ ! -f .env ]; then
    echo "ERROR: .env not found. Run 'bash scripts/setup-local.sh' first."
    exit 1
fi

# Start infrastructure
info "Starting PostgreSQL and Redis..."
docker compose up -d postgres redis
sleep 3

# Verify health
curl -sf http://localhost:8080/health >/dev/null 2>&1 && {
    info "Backend is already running on :8080"
} || {
    info "Starting backend server..."
    cd backend
    go run ./cmd/server &
    SERVER_PID=$!
    cd "$PROJECT_ROOT"
    sleep 2
}

info "Starting background worker..."
cd backend
go run ./cmd/worker &
WORKER_PID=$!
cd "$PROJECT_ROOT"

info "Starting frontend dev server..."
cd frontend
npm run dev &
FRONTEND_PID=$!
cd "$PROJECT_ROOT"

echo ""
info "All services started!"
echo ""
echo "  Backend:  http://localhost:8080"
echo "  Frontend: http://localhost:3000"
echo "  Health:   http://localhost:8080/health/ready"
echo ""
echo "  Supplier demo: http://localhost:3000/?session=dev_supplier_session_token"
echo "  Reseller demo: http://localhost:3000/?session=dev_reseller_session_token"
echo ""
echo "Press Ctrl+C to stop all services."

# Trap Ctrl+C to kill all background processes
cleanup() {
    info "Stopping services..."
    kill $SERVER_PID $WORKER_PID $FRONTEND_PID 2>/dev/null
    docker compose stop postgres redis 2>/dev/null
    info "Done."
}
trap cleanup EXIT INT TERM

# Wait for any background process to exit
wait
