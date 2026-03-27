# DropToDrop API Reference

Base URL: `https://droptodrop.osc-fr1.scalingo.io`

All authenticated endpoints require `Authorization: Bearer <token>` header.
Tokens are Shopify App Bridge JWTs or database session tokens.

---

## Public Endpoints

### Health
- `GET /health` — Liveness check. Returns `{"status":"ok"}`
- `GET /health/ready` — Readiness check. Returns `{"status":"healthy","checks":{"database":"ok","redis":"ok"}}`

### OAuth
- `GET /auth/install?shop=<domain>` — Start OAuth install flow
- `GET /auth/callback` — OAuth callback (Shopify redirects here)

### Webhooks
- `POST /webhooks/app/uninstalled` — App uninstalled
- `POST /webhooks/orders/create` — New order created
- `POST /webhooks/fulfillments/create` — Fulfillment created
- `POST /webhooks/products/update` — Product updated
- `POST /webhooks/products/delete` — Product deleted
- `POST /webhooks/inventory/update` — Inventory level changed

### Compliance Webhooks
- `POST /webhooks/compliance/customers-data-request`
- `POST /webhooks/compliance/customers-redact`
- `POST /webhooks/compliance/shop-redact`

---

## Authenticated API (`/api/v1`)

### Shop
| Method | Path | Description |
|--------|------|-------------|
| GET | `/shop` | Get current shop info |
| POST | `/shop/role` | Set role (`{"role":"supplier"}` or `{"role":"reseller"}`) |

### Dashboard
| Method | Path | Description |
|--------|------|-------------|
| GET | `/dashboard` | Role-specific dashboard data with stats and recent orders |

### Settings
| Method | Path | Description |
|--------|------|-------------|
| GET | `/settings` | Get app settings |
| PUT | `/settings` | Update app settings |

### Orders (both roles)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/orders/:id` | Get order detail with fulfillments |

### Audit
| Method | Path | Description |
|--------|------|-------------|
| GET | `/audit?limit=50&offset=0` | List audit log entries |

### Billing
| Method | Path | Description |
|--------|------|-------------|
| GET | `/billing` | Get billing status |

---

## Supplier Endpoints (`/api/v1/supplier`)

Requires role: `supplier`

### Profile
| Method | Path | Description |
|--------|------|-------------|
| GET | `/supplier/profile` | Get supplier profile |
| PUT | `/supplier/profile` | Update supplier profile |

### Listings
| Method | Path | Description |
|--------|------|-------------|
| GET | `/supplier/listings?status=active&limit=20&offset=0` | List supplier listings |
| POST | `/supplier/listings` | Create new listing |
| GET | `/supplier/listings/:id` | Get single listing with variants |
| PUT | `/supplier/listings/:id` | Update listing (title, description, category, prices) |
| PUT | `/supplier/listings/:id/status` | Change listing status (draft/active/paused/archived) |
| DELETE | `/supplier/listings/:id` | Delete a listing |

### Shop Products
| Method | Path | Description |
|--------|------|-------------|
| GET | `/supplier/shop-products?cursor=<cursor>` | Fetch products from Shopify store |

### Orders
| Method | Path | Description |
|--------|------|-------------|
| GET | `/supplier/orders?status=pending&limit=20&offset=0` | List routed orders |
| POST | `/supplier/orders/:id/accept` | Accept a pending order |
| POST | `/supplier/orders/:id/reject` | Reject order (`{"reason":"..."}`) |
| POST | `/supplier/orders/:id/fulfill` | Add fulfillment tracking |

---

## Reseller Endpoints (`/api/v1/reseller`)

Requires role: `reseller`

### Profile
| Method | Path | Description |
|--------|------|-------------|
| GET | `/reseller/profile` | Get reseller profile |

### Marketplace
| Method | Path | Description |
|--------|------|-------------|
| GET | `/reseller/marketplace?category=electronics&search=phone&limit=20&offset=0` | Browse supplier listings |
| GET | `/reseller/marketplace/:id` | Get marketplace listing detail |

### Suppliers
| Method | Path | Description |
|--------|------|-------------|
| GET | `/reseller/suppliers/:id` | Get supplier profile info |

### Imports
| Method | Path | Description |
|--------|------|-------------|
| POST | `/reseller/imports` | Import a supplier listing |
| GET | `/reseller/imports?limit=20&offset=0` | List imported products |
| POST | `/reseller/imports/:id/resync` | Trigger manual re-sync |

### Orders
| Method | Path | Description |
|--------|------|-------------|
| GET | `/reseller/orders?status=pending&limit=20&offset=0` | List routed orders |

---

## Data Models

### Shop
```json
{
  "id": "uuid",
  "shopify_domain": "store.myshopify.com",
  "role": "supplier|reseller|unset",
  "status": "active",
  "currency": "USD",
  "created_at": "2026-01-01T00:00:00Z"
}
```

### SupplierListing
```json
{
  "id": "uuid",
  "title": "Product Name",
  "description": "...",
  "category": "electronics",
  "status": "active",
  "processing_days": 3,
  "variants": [
    {
      "id": "uuid",
      "title": "Default",
      "sku": "SKU-001",
      "wholesale_price": 25.00,
      "suggested_retail_price": 49.99
    }
  ]
}
```

### RoutedOrder
```json
{
  "id": "uuid",
  "reseller_order_number": "#1001",
  "status": "pending|accepted|processing|fulfilled|rejected|cancelled",
  "total_wholesale_amount": 50.00,
  "currency": "USD",
  "items": [...]
}
```

### Categories
Available values: `apparel`, `accessories`, `electronics`, `home_garden`, `beauty`, `sports`, `toys`, `pet`, `food`, `health`, `automotive`, `office`, `other`
