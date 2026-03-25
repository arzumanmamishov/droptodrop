# Environment Variables Reference

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| SHOPIFY_API_KEY | Yes | Shopify app API key from Partner Dashboard | `abc123def456` |
| SHOPIFY_API_SECRET | Yes | Shopify app API secret | `shpss_xxxxx` |
| SHOPIFY_SCOPES | No | OAuth scopes (defaults provided) | `read_products,write_products,...` |
| SHOPIFY_APP_URL | Yes | Public HTTPS URL of the app | `https://droptodrop.example.com` |
| SHOPIFY_REDIRECT_URI | Yes | OAuth callback URL | `https://droptodrop.example.com/auth/callback` |
| SERVER_PORT | No | HTTP port (default: 8080) | `8080` |
| SERVER_HOST | No | Bind address (default: 0.0.0.0) | `0.0.0.0` |
| APP_ENV | No | Environment (default: development) | `production` |
| LOG_LEVEL | No | Log level (default: debug) | `info` |
| DATABASE_URL | Yes | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=require` |
| DATABASE_MAX_OPEN_CONNS | No | Max open connections (default: 25) | `25` |
| DATABASE_MAX_IDLE_CONNS | No | Max idle connections (default: 5) | `5` |
| DATABASE_CONN_MAX_LIFETIME | No | Connection max lifetime (default: 5m) | `5m` |
| REDIS_URL | No | Redis connection URL (default: localhost) | `redis://host:6379/0` |
| REDIS_PASSWORD | No | Redis password | `secret` |
| SESSION_SECRET | Yes | Session signing secret (32+ chars) | `random_hex_string_64_chars` |
| ENCRYPTION_KEY | Yes | AES-256 key for token encryption (64 hex chars = 32 bytes) | `0123456789abcdef...` |
| RATE_LIMIT_RPS | No | Requests per second limit (default: 100) | `100` |
| RATE_LIMIT_BURST | No | Rate limit burst (default: 200) | `200` |
| WORKER_CONCURRENCY | No | Worker goroutines per queue (default: 5) | `5` |
| WORKER_RETRY_MAX | No | Max job retries (default: 3) | `3` |
| WORKER_RETRY_DELAY | No | Delay between retries (default: 5s) | `5s` |
| VITE_SHOPIFY_API_KEY | Yes (frontend) | Same as SHOPIFY_API_KEY for frontend | `abc123def456` |
| VITE_APP_URL | Yes (frontend) | Same as SHOPIFY_APP_URL for frontend | `https://droptodrop.example.com` |
