# What Is Still Needed Before Shopify Submission

This is a brutally honest assessment of what is implemented, what is incomplete, and what exact actions are needed.

## Fully Implemented

| Feature | Status | Notes |
|---------|--------|-------|
| Database schema with 17 tables | Complete | Migration up/down, triggers, indexes |
| Shopify OAuth install/callback | Complete | CSRF nonce, HMAC validation, token encryption |
| Shop role selection (supplier/reseller) | Complete | Onboarding screen, DB profile creation |
| Supplier profile management | Complete | Enable/disable, processing time, blind fulfillment, approval mode |
| Supplier listing CRUD | Complete | Create, list, status changes (publish/pause/archive) |
| Marketplace browse/search | Complete | Filtering, pagination, supplier listing view |
| Product import with markup | Complete | Percentage/fixed markup, variant-level pricing preview |
| Order routing from webhook | Complete | Line item splitting by supplier, idempotency keys |
| Supplier order accept/reject | Complete | Status transitions, audit logging |
| Fulfillment tracking | Complete | Add tracking, sync to reseller via queue |
| Webhook HMAC verification | Complete | Raw body verification, deduplication |
| Mandatory compliance webhooks | Complete | customers/data_request, customers/redact, shop/redact |
| Uninstall handling | Complete | Soft-deactivate shop, expire sessions, deactivate installation |
| Audit logging | Complete | Every admin action logged with actor/resource/outcome |
| Rate limiting | Complete | Per-shop token bucket |
| Background job queue | Complete | Redis-based with retry, dead letter, failed_jobs table |
| Session management | Complete | Database-backed, expiry-aware |
| Health endpoints | Complete | Liveness + readiness (DB + Redis checks) |
| 9 embedded admin screens | Complete | Dashboard, Setup, Listings, Marketplace, Imports, Orders, Detail, Settings, Audit |
| Docker + docker-compose | Complete | Multi-stage builds, health checks |
| CI pipeline | Complete | GitHub Actions with test/lint/build/docker |
| Unit tests | Complete | HMAC, OAuth, crypto, pricing, idempotency, retry |
| Integration test scaffolding | Complete | Shop CRUD, uninstall flow |
| Seed data | Complete | Demo supplier/reseller with sample products |

## Recently Completed (moved from partial)

### App Bridge Session Token Verification — DONE
- Full HS256 JWT decode/verify in `pkg/sessiontoken/verify.go`
- Dual-path auth middleware (JWT + DB session) in `internal/middleware/auth.go`
- Frontend `AppBridgeProvider` wires `window.shopify.idToken()` automatically
- 16 unit tests covering valid/expired/tampered/malformed tokens

### Product Creation in Reseller's Shopify Store — DONE
- `handleCreateProduct` calls `productCreate` GraphQL mutation with typed response structs
- Parses product GID and variant GIDs from Shopify response via `ParseGID()`
- Updates `reseller_imports.shopify_product_id` with created product numeric ID
- Updates each `reseller_import_variants.shopify_variant_id` with created variant IDs
- Creates `product_links` entries connecting supplier_variant_id → reseller_variant_id
- Handles image sync when `sync_images` is true (passes image URLs to Shopify)
- Handles description sync when `sync_description` is true

### Fulfillment Sync to Reseller's Shopify Store — DONE
- `handleFulfillmentSync` fetches fulfillment orders via `GetFulfillmentOrders()` GraphQL query
- Selects the correct OPEN/IN_PROGRESS fulfillment order
- Calls `fulfillmentCreateV2` with tracking number, URL, and company
- Parses fulfillment GID from response and stores as `shopify_fulfillment_id`
- Marks `fulfillment_events.synced_to_reseller = TRUE` on success
- Records `sync_error` on failure for visibility

### Product Update Sync Worker — DONE
- `handleProductUpdate` updates `supplier_listings` fields (title, description, type, vendor, tags)
- Updates `supplier_listing_variants` inventory quantities
- Queues re-sync jobs for all active reseller imports of the affected listing
- Re-sync job (`handleSyncProduct`) recalculates reseller prices from current wholesale + markup

### Inventory Sync Worker — DONE
- `handleInventorySync` records inventory snapshots
- Updates `supplier_listing_variants.inventory_quantity` from webhook data

## Partially Implemented (Requires Completion)

### 1. Inventory Propagation to Reseller Stores
**Status**: Supplier inventory is updated from webhooks, but reseller store inventory is not yet updated via Shopify API.
**What's needed**:
- After updating supplier inventory, call Shopify `inventorySetQuantities` mutation on reseller stores
- Handle zero-stock → unpublish behavior
- Add periodic reconciliation job
**Estimated effort**: 3-4 hours

## Not Implemented (Required for Submission)

### 6. Billing Integration
**Status**: Placeholder with plan display, no actual Shopify Billing API calls.
**What's needed**:
- Decide between Shopify Managed Pricing (Partner Dashboard) or Billing API
- If using Billing API: implement `appSubscriptionCreate` mutation, confirmation redirect, charge verification
- If using Managed Pricing: configure plans in Partner Dashboard, verify subscription status
**Note**: App can be submitted with free plan initially.
**Estimated effort**: 4-8 hours (depending on approach)

### 7. E2E Tests with Playwright — DONE
- Playwright project set up in `e2e/` directory
- 3 test suites: `onboarding.spec.ts`, `supplier-flow.spec.ts`, `reseller-flow.spec.ts`
- Tests cover: auth, health check, HMAC rejection, dashboard, setup, listings, marketplace, imports, orders, audit log, settings
- Run with: `cd e2e && npm install && npx playwright test`

### 8. go.sum File — DONE
`go mod tidy` ran and generated go.sum.

### 9. Frontend npm lock file
**Status**: package.json created but package-lock.json not generated.
**What's needed**: Run `cd frontend && npm install` to generate lock file.
**Estimated effort**: 1 minute

## Known Limitations

1. **Single API version**: Hardcoded to Shopify API version `2024-10`. Should be configurable.
2. **No webhook subscription management UI**: Webhooks are registered on install but there's no way to verify or re-register from the admin.
3. **No bulk operations**: Bulk publish/unpublish, bulk import are listed in UI but bulk API calls are not implemented.
4. **Rate limit store is in-memory**: Resets on server restart. Should use Redis for distributed rate limiting.
5. **No email notifications**: Supplier notification job logs but doesn't send email. Needs an email service integration (SendGrid, SES, etc.).
6. **No real-time updates**: Dashboard doesn't auto-refresh. Could add WebSocket or polling.
7. **Supplier approval workflow**: Schema supports it, but no UI for reseller approval requests.

## Critical Path to Submission

In priority order:

1. **Run `go mod tidy` and `npm install`** to generate lock files
2. **Complete App Bridge session token verification** (items 5)
3. **Complete product creation in reseller store** (item 1) - this is the core value proposition
4. **Complete fulfillment sync** (item 2) - required for the end-to-end flow
5. **Add Shopify Resource Picker** for supplier product selection
6. **Test on actual Shopify dev store** with real OAuth credentials
7. **Record demo video** for review submission
8. **Verify all 9 webhook endpoints** are accessible over HTTPS
9. **Submit for review** with free plan, billing can be added later

## Estimated Time to Submission-Ready

With focused effort: **5-8 hours** of additional development and testing.

The biggest remaining gaps are:
- Run `npm install` in frontend/ and e2e/ to generate lock files
- Test the full flow on a real Shopify dev store with real OAuth credentials
- Record a demo video for Shopify review submission
- Set up real email notifications for supplier order alerts
