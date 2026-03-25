# Architecture Overview

## System Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    Shopify Admin (Embedded)                       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  React + Polaris + App Bridge                             │   │
│  │  ┌──────┐ ┌──────────┐ ┌───────────┐ ┌──────┐ ┌──────┐ │   │
│  │  │Dash  │ │Supplier  │ │Marketplace│ │Orders│ │Audit │ │   │
│  │  │board │ │Setup/List│ │& Imports  │ │      │ │Log   │ │   │
│  │  └──────┘ └──────────┘ └───────────┘ └──────┘ └──────┘ │   │
│  └──────────────────────┬───────────────────────────────────┘   │
└─────────────────────────┼───────────────────────────────────────┘
                          │ REST API (Bearer token)
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Go Backend (Gin)                              │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐ ┌──────────┐   │
│  │  Auth  │ │Webhook │ │Products│ │  Orders  │ │Fulfillmt │   │
│  │  OAuth │ │ HMAC   │ │Listings│ │ Routing  │ │  Sync    │   │
│  └────────┘ └────────┘ └────────┘ └──────────┘ └──────────┘   │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐ ┌──────────┐   │
│  │Imports │ │Compli- │ │ Audit  │ │ Billing  │ │  Health  │   │
│  │  Sync  │ │ ance   │ │  Log   │ │(Placeholder)│ │ Check  │   │
│  └────────┘ └────────┘ └────────┘ └──────────┘ └──────────┘   │
└───────────┬──────────────────────────────────────┬──────────────┘
            │                                      │
            ▼                                      ▼
┌───────────────────┐                  ┌───────────────────┐
│   PostgreSQL 16   │                  │    Redis 7        │
│                   │                  │                   │
│ shops             │                  │ job queues        │
│ installations     │                  │ idempotency keys  │
│ supplier_listings │                  │ cache             │
│ reseller_imports  │                  │ dead letter       │
│ product_links     │                  │                   │
│ routed_orders     │                  │                   │
│ fulfillment_events│                  │                   │
│ webhook_events    │                  │                   │
│ audit_logs        │                  │                   │
└───────────────────┘                  └─────────┬─────────┘
                                                 │
                                                 ▼
                                     ┌───────────────────┐
                                     │  Background Worker │
                                     │                   │
                                     │ product creation  │
                                     │ inventory sync    │
                                     │ fulfillment sync  │
                                     │ notifications     │
                                     │ order routing     │
                                     └───────────────────┘
```

## Data Flow: Order Routing

```
Customer orders on Reseller Store
        │
        ▼
Shopify sends orders/create webhook
        │
        ▼
Backend verifies HMAC (raw body)
        │
        ▼
Records webhook_event (dedup by hash)
        │
        ▼
Looks up product_links for line items
        │
        ▼
Groups items by supplier
        │
        ▼
Creates routed_order with idempotency_key
        │
        ▼
Creates routed_order_items
        │
        ▼
Queues supplier notification
        │
        ▼
Supplier sees order in dashboard
        │
        ▼
Supplier accepts → adds tracking
        │
        ▼
Creates fulfillment_event
        │
        ▼
Worker syncs fulfillment to Reseller's Shopify store
        │
        ▼
Reseller and customer see tracking
```

## Security Model

1. **Authentication**: Shopify OAuth 2.0 with CSRF nonce verification
2. **Session**: Database-backed sessions with expiry, compatible with App Bridge
3. **Webhooks**: HMAC-SHA256 verification using raw request body bytes
4. **Encryption**: AES-256-GCM for access tokens at rest
5. **Rate limiting**: Per-shop token bucket rate limiter
6. **Role-based access**: Middleware enforces supplier/reseller role guards
7. **Input validation**: Gin binding validation on all endpoints
8. **CORS**: Restricted to app URL
9. **CSP**: frame-ancestors restricted to Shopify domains
10. **Idempotency**: SHA-256 keys prevent duplicate order routing

## Webhook Strategy

All webhooks verified with HMAC-SHA256 using the raw request body before JSON parsing.
Each webhook is recorded in webhook_events with payload hash for deduplication.
Heavy processing is offloaded to Redis queues for async handling.

### Registered Webhooks

| Topic | Handler | Purpose |
|-------|---------|---------|
| APP_UNINSTALLED | Deactivate shop, expire sessions | Cleanup |
| ORDERS_CREATE | Route to supplier(s) | Core flow |
| FULFILLMENTS_CREATE | Sync tracking | Core flow |
| PRODUCTS_UPDATE | Update linked listings | Sync |
| PRODUCTS_DELETE | Archive listings | Sync |
| INVENTORY_LEVELS_UPDATE | Update stock counts | Sync |
| CUSTOMERS_DATA_REQUEST | Report stored data | Compliance |
| CUSTOMERS_REDACT | Delete customer PII | Compliance |
| SHOP_REDACT | Delete shop PII | Compliance |
