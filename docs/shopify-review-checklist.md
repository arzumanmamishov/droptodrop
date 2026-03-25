# Shopify App Store Review Readiness Checklist

## Mandatory Requirements

### OAuth & Installation
- [x] Standard Shopify OAuth 2.0 flow implemented
- [x] CSRF state/nonce verification on callback
- [x] HMAC signature validation on callback
- [x] Shop domain validation (*.myshopify.com only)
- [x] Access token encrypted at rest (AES-256-GCM)
- [x] Least-privilege scopes requested
- [x] Re-install flow handles existing shops (upsert)
- [x] Uninstall webhook (app/uninstalled) handled

### Embedded App
- [x] App renders inside Shopify Admin iframe
- [x] Content-Security-Policy allows Shopify frame-ancestors
- [x] Uses Shopify Polaris design system
- [x] App Bridge integration for embedded behavior
- [x] Session-token compatible authentication

### Mandatory Compliance Webhooks
- [x] customers/data_request handler implemented
- [x] customers/redact handler implemented (deletes PII)
- [x] shop/redact handler implemented (deletes shop PII)
- [x] All compliance webhooks HMAC-verified
- [x] Compliance events recorded in database

### Webhook Security
- [x] All webhooks use HMAC-SHA256 verification
- [x] Verification uses raw request body (not re-serialized JSON)
- [x] Invalid signatures rejected with 401
- [x] Webhook deduplication via payload hash
- [x] Idempotent webhook processing

### App Behavior
- [x] No broken pages or dead links
- [x] Loading states on all data-fetching pages
- [x] Error states with meaningful messages
- [x] Empty states with helpful guidance
- [x] Pagination on list views
- [x] Form validation on all inputs

### Privacy & Legal
- [x] Privacy policy URL configurable
- [x] Terms of service URL configurable
- [x] Support email configurable
- [x] Data retention settings available
- [x] Customer data redaction implemented

### Billing
- [x] Billing placeholder structured for Shopify Managed Pricing
- [x] No misleading pricing or fake billing flows
- [ ] Actual billing plan activation (deferred - see gap analysis)

## Pre-Submission Actions Required

1. Create Shopify Partner app listing with accurate description
2. Record demo video showing full flow
3. Provide test store credentials to reviewers
4. Verify all webhook endpoints are accessible over HTTPS
5. Test install/uninstall cycle on fresh dev store
6. Verify embedded app loads correctly in Shopify Admin
7. Ensure privacy policy and support info are real URLs

## Manual QA Checklist

### Install Flow
- [ ] Install from Shopify App Store link
- [ ] OAuth redirects correctly
- [ ] Callback succeeds and creates session
- [ ] Redirects to embedded app in admin
- [ ] Onboarding screen shows role selection

### Supplier Flow
- [ ] Select supplier role
- [ ] Supplier setup page loads with defaults
- [ ] Can enable supplier mode
- [ ] Can create/edit listings
- [ ] Can publish/pause/archive listings
- [ ] Can view incoming orders
- [ ] Can accept/reject orders
- [ ] Can add fulfillment tracking
- [ ] Tracking syncs to reseller

### Reseller Flow
- [ ] Select reseller role
- [ ] Marketplace shows active listings
- [ ] Can search/filter marketplace
- [ ] Import modal shows pricing preview
- [ ] Import creates product in store
- [ ] Can view imported products
- [ ] Can trigger re-sync
- [ ] Orders appear when customers purchase
- [ ] Can view order details and tracking

### General
- [ ] Dashboard shows correct metrics
- [ ] Settings save and persist
- [ ] Audit log shows events
- [ ] Uninstall cleans up properly
- [ ] Re-install restores state
- [ ] No console errors in embedded view
- [ ] Mobile responsive in admin
