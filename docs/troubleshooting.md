# Troubleshooting Guide

## Common Issues

### "HMAC verification failed" on webhooks
- Ensure `SHOPIFY_API_SECRET` matches your app's API secret in Partner Dashboard
- Verify the webhook endpoint is receiving the raw body, not a proxy-modified version
- Check that no middleware is parsing or modifying the body before HMAC verification
- Nginx/load balancer must forward `X-Shopify-Hmac-Sha256` header

### "invalid or expired session" errors
- Session tokens expire after `SESSION_MAX_AGE` seconds (default: 24 hours)
- After uninstall/reinstall, old sessions are deleted
- Ensure the frontend is sending the correct Bearer token in Authorization header
- Check `shop_sessions` table for expired entries

### OAuth callback fails with "state mismatch"
- CSRF nonce is stored in a cookie; ensure cookies are not blocked
- Check that `SHOPIFY_REDIRECT_URI` exactly matches what's configured in Partner Dashboard
- Clear cookies and retry the install

### Database connection errors
- Verify `DATABASE_URL` format: `postgres://user:pass@host:5432/dbname?sslmode=disable`
- Check that PostgreSQL is running and accessible
- Verify connection limits aren't exceeded (check `DATABASE_MAX_OPEN_CONNS`)

### Redis connection errors
- Verify `REDIS_URL` format: `redis://host:6379/0`
- Check that Redis is running and accessible
- If using Redis with auth, set `REDIS_PASSWORD`

### Worker not processing jobs
- Check worker logs for connection errors
- Verify Redis is running
- Check `background_jobs` and `failed_jobs` tables for stuck/failed jobs
- Restart worker if queues are stale

### Products not syncing to reseller store
- Check `reseller_imports` table for `status = 'failed'` and `last_sync_error`
- Verify the reseller's access token is valid
- Check `background_jobs` table for failed import jobs
- Check `failed_jobs` for error details

### Webhooks not being received
- Verify your app URL is accessible over HTTPS from the internet
- Check Shopify Partner Dashboard > Webhooks for delivery failures
- Ensure firewall allows incoming HTTPS traffic
- Check `webhook_events` table for received webhooks

### Embedded app shows blank or "refused to connect"
- Check Content-Security-Policy header allows Shopify frame-ancestors
- Verify `SHOPIFY_APP_URL` is correct and HTTPS
- Ensure the frontend is served at the correct URL
- Check browser console for CSP violation errors

## Debugging Commands

```bash
# Check database state
psql $DATABASE_URL -c "SELECT id, shopify_domain, role, status FROM shops;"
psql $DATABASE_URL -c "SELECT * FROM webhook_events ORDER BY created_at DESC LIMIT 10;"
psql $DATABASE_URL -c "SELECT * FROM background_jobs WHERE status = 'failed';"
psql $DATABASE_URL -c "SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 20;"

# Check Redis queues
redis-cli LLEN queue:imports
redis-cli LLEN queue:orders
redis-cli LLEN queue:fulfillments
redis-cli LLEN queue:imports:dead

# Health check
curl http://localhost:8080/health
curl http://localhost:8080/health/ready
```
