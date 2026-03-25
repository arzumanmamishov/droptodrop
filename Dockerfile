# Stage 1: Build frontend
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# Stage 2: Build backend
FROM golang:1.22-alpine AS backend-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

# Stage 3: Production runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /bin/server /app/server
COPY --from=backend-builder /bin/worker /app/worker
COPY --from=backend-builder /app/migrations /app/migrations
COPY --from=frontend-builder /app/frontend/dist /app/static

EXPOSE 8080

CMD ["/app/server"]
