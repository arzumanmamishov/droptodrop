#!/bin/bash
# =============================================================================
# DropToDrop — Webhook Compliance Test Script
#
# Simulates Shopify compliance webhooks with valid HMAC signatures to verify
# that all mandatory endpoints respond correctly.
#
# Usage: bash scripts/test-webhooks.sh [APP_URL] [API_SECRET]
#
# Example: bash scripts/test-webhooks.sh https://abc123.ngrok-free.app my_api_secret
# =============================================================================

set -e

APP_URL="${1:-http://localhost:8080}"
API_SECRET="${2:-}"

if [ -z "$API_SECRET" ]; then
    # Try to read from .env
    if [ -f .env ]; then
        API_SECRET=$(grep SHOPIFY_API_SECRET .env | cut -d= -f2-)
    fi
fi

if [ -z "$API_SECRET" ]; then
    echo "ERROR: API secret required."
    echo "Usage: bash scripts/test-webhooks.sh [APP_URL] [API_SECRET]"
    echo "  or set SHOPIFY_API_SECRET in .env"
    exit 1
fi

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() { echo -e "  ${GREEN}PASS${NC} $1"; }
fail() { echo -e "  ${RED}FAIL${NC} $1 (status: $2)"; }

compute_hmac() {
    echo -n "$1" | openssl dgst -sha256 -hmac "$API_SECRET" -binary | base64
}

test_webhook() {
    local name="$1"
    local endpoint="$2"
    local payload="$3"

    local hmac=$(compute_hmac "$payload")
    local status=$(curl -s -o /dev/null -w "%{http_code}" \
        -X POST "$APP_URL$endpoint" \
        -H "Content-Type: application/json" \
        -H "X-Shopify-Hmac-Sha256: $hmac" \
        -H "X-Shopify-Shop-Domain: test-webhook.myshopify.com" \
        -H "X-Shopify-Webhook-Id: test-$(date +%s)" \
        -d "$payload")

    if [ "$status" = "200" ]; then
        pass "$name"
    else
        fail "$name" "$status"
    fi
}

test_webhook_rejected() {
    local name="$1"
    local endpoint="$2"

    local status=$(curl -s -o /dev/null -w "%{http_code}" \
        -X POST "$APP_URL$endpoint" \
        -H "Content-Type: application/json" \
        -H "X-Shopify-Hmac-Sha256: invalid_hmac_value" \
        -d '{"test":true}')

    if [ "$status" = "401" ]; then
        pass "$name (correctly rejected)"
    else
        fail "$name (should be 401)" "$status"
    fi
}

echo "========================================="
echo "DropToDrop Webhook Compliance Tests"
echo "========================================="
echo ""
echo "Target: $APP_URL"
echo ""

# --- Health check ---
echo "Health Checks:"
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "$APP_URL/health")
if [ "$HEALTH" = "200" ]; then pass "/health"; else fail "/health" "$HEALTH"; fi

READY=$(curl -s -o /dev/null -w "%{http_code}" "$APP_URL/health/ready")
if [ "$READY" = "200" ]; then pass "/health/ready"; else fail "/health/ready" "$READY"; fi
echo ""

# --- HMAC rejection tests ---
echo "HMAC Rejection (all should be 401 with invalid signature):"
test_webhook_rejected "app/uninstalled" "/webhooks/app/uninstalled"
test_webhook_rejected "orders/create" "/webhooks/orders/create"
test_webhook_rejected "products/update" "/webhooks/products/update"
test_webhook_rejected "products/delete" "/webhooks/products/delete"
test_webhook_rejected "inventory/update" "/webhooks/inventory/update"
test_webhook_rejected "fulfillments/create" "/webhooks/fulfillments/create"
test_webhook_rejected "customers/data_request" "/webhooks/compliance/customers-data-request"
test_webhook_rejected "customers/redact" "/webhooks/compliance/customers-redact"
test_webhook_rejected "shop/redact" "/webhooks/compliance/shop-redact"
echo ""

# --- Valid HMAC tests ---
echo "Valid HMAC Compliance Webhooks:"

test_webhook "customers/data_request" "/webhooks/compliance/customers-data-request" \
    '{"shop_id":1,"shop_domain":"test-webhook.myshopify.com","customer":{"id":1,"email":"test@example.com","phone":"+1234567890"},"orders_requested":[1001]}'

test_webhook "customers/redact" "/webhooks/compliance/customers-redact" \
    '{"shop_id":1,"shop_domain":"test-webhook.myshopify.com","customer":{"id":1,"email":"test@example.com","phone":"+1234567890"},"orders_to_redact":[1001]}'

test_webhook "shop/redact" "/webhooks/compliance/shop-redact" \
    '{"shop_id":1,"shop_domain":"test-webhook.myshopify.com"}'

echo ""

# --- Auth tests ---
echo "Auth Tests:"
AUTH_401=$(curl -s -o /dev/null -w "%{http_code}" "$APP_URL/api/v1/shop")
if [ "$AUTH_401" = "401" ]; then pass "API rejects unauthenticated"; else fail "API rejects unauthenticated" "$AUTH_401"; fi

AUTH_OK=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer dev_supplier_session_token" \
    "$APP_URL/api/v1/shop")
if [ "$AUTH_OK" = "200" ]; then pass "API accepts valid session"; else fail "API accepts valid session" "$AUTH_OK"; fi

AUTH_BAD=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer invalid_token" \
    "$APP_URL/api/v1/shop")
if [ "$AUTH_BAD" = "401" ]; then pass "API rejects invalid session"; else fail "API rejects invalid session" "$AUTH_BAD"; fi
echo ""

echo "========================================="
echo "Done."
echo "========================================="
