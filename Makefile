.PHONY: dev dev-backend dev-frontend build test lint migrate seed docker-up docker-down clean

# ===== Development =====
dev: docker-infra dev-backend dev-frontend

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

dev-worker:
	cd backend && go run ./cmd/worker

# ===== Infrastructure =====
docker-infra:
	docker compose up -d postgres redis

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# ===== Database =====
migrate-up:
	migrate -path backend/migrations -database "$${DATABASE_URL}" up

migrate-down:
	migrate -path backend/migrations -database "$${DATABASE_URL}" down 1

migrate-create:
	migrate create -ext sql -dir backend/migrations -seq $(name)

seed:
	cd backend && go run ./cmd/seed

# ===== Build =====
build-backend:
	cd backend && CGO_ENABLED=0 go build -o bin/server ./cmd/server
	cd backend && CGO_ENABLED=0 go build -o bin/worker ./cmd/worker

build-frontend:
	cd frontend && npm ci && npm run build

build: build-backend build-frontend

# ===== Test =====
test-backend:
	cd backend && go test ./... -v -count=1

test-frontend:
	cd frontend && npm run test

test: test-backend test-frontend

# ===== Quality =====
lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && npm run lint

lint: lint-backend lint-frontend

typecheck:
	cd frontend && npm run typecheck

fmt:
	cd backend && gofmt -w .
	cd frontend && npx prettier --write src/

# ===== Cleanup =====
clean:
	rm -rf backend/bin backend/tmp frontend/dist frontend/node_modules
