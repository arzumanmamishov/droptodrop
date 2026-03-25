# Shopify Partner Setup — Step by Step

This guide walks you through every step to go from zero to a working DropToDrop installation on real Shopify dev stores.

---

## Prerequisites

- A free [Shopify Partner account](https://partners.shopify.com/signup)
- Docker Desktop running (for PostgreSQL + Redis)
- Node.js 22+ and Go 1.22+ installed
- A tunneling tool: [ngrok](https://ngrok.com/download) (free tier works) or [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)

---

## Part 1: Create the Shopify App

### Step 1 — Log in to Partner Dashboard

Go to https://partners.shopify.com and sign in.

### Step 2 — Create a new app

1. Click **Apps** in the left sidebar
2. Click **Create app**
3. Choose **Create app manually**
4. Set:
   - **App name**: `DropToDrop`
   - **App URL**: `https://YOUR_TUNNEL_DOMAIN` (you'll get this in Part 2)
   - **Allowed redirection URL(s)**: `https://YOUR_TUNNEL_DOMAIN/auth/callback`
5. Click **Create app**

### Step 3 — Copy credentials

On the app's overview page, find:
- **API key** (also called Client ID)
- **API secret key** (click "Show" to reveal)

Save these — you'll need them for `.env`.

### Step 4 — Configure API scopes

Go to **Configuration** → **API access scopes** and ensure these are selected:

```
read_products
write_products
read_orders
write_orders
read_fulfillments
write_fulfillments
read_inventory
write_inventory
read_shipping
```

### Step 5 — Set compliance webhook URLs

Under **Configuration** → **Compliance webhooks**, set:

| Webhook | URL |
|---------|-----|
| Customer data request | `https://YOUR_TUNNEL_DOMAIN/webhooks/compliance/customers-data-request` |
| Customer data erasure | `https://YOUR_TUNNEL_DOMAIN/webhooks/compliance/customers-redact` |
| Shop data erasure | `https://YOUR_TUNNEL_DOMAIN/webhooks/compliance/shop-redact` |

---

## Part 2: Set Up the Tunnel

You need a public HTTPS URL that forwards to your local machine.

### Option A — ngrok (recommended for quick start)

```bash
# Install ngrok (if not already)
# https://ngrok.com/download

# Start a tunnel to port 8080
ngrok http 8080
```

ngrok will display a URL like `https://abc123.ngrok-free.app`. This is your `YOUR_TUNNEL_DOMAIN`.

### Option B — Cloudflare Tunnel

```bash
cloudflared tunnel --url http://localhost:8080
```

### After starting the tunnel

1. Copy the HTTPS URL (e.g. `https://abc123.ngrok-free.app`)
2. Go back to Shopify Partner Dashboard → your app → **Configuration**
3. Update:
   - **App URL**: `https://abc123.ngrok-free.app`
   - **Allowed redirection URL(s)**: `https://abc123.ngrok-free.app/auth/callback`
   - **Compliance webhook URLs** (all three, using the tunnel domain)
4. Click **Save**

> **Important**: Every time you restart ngrok, you get a new URL. You must update the Partner Dashboard each time, or use a paid ngrok plan with a stable domain.

---

## Part 3: Configure Local Environment

### Step 1 — Create .env

```bash
cd /path/to/droptodrop
cp .env.example .env
```

### Step 2 — Fill in .env

Edit `.env` with your real values:

```env
# From Shopify Partner Dashboard
SHOPIFY_API_KEY=your_api_key_from_step_3
SHOPIFY_API_SECRET=your_api_secret_from_step_3
SHOPIFY_SCOPES=read_products,write_products,read_orders,write_orders,read_fulfillments,write_fulfillments,read_inventory,write_inventory,read_shipping
SHOPIFY_APP_URL=https://abc123.ngrok-free.app
SHOPIFY_REDIRECT_URI=https://abc123.ngrok-free.app/auth/callback

# Server
SERVER_PORT=8080
APP_ENV=development

# Database (Docker will use these)
DATABASE_URL=postgres://droptodrop:droptodrop@localhost:5432/droptodrop?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379/0

# Generate these with: openssl rand -hex 32
SESSION_SECRET=PASTE_64_HEX_CHARS_HERE
ENCRYPTION_KEY=PASTE_64_HEX_CHARS_HERE

# Frontend
VITE_SHOPIFY_API_KEY=your_api_key_from_step_3
VITE_APP_URL=https://abc123.ngrok-free.app
```

### Step 3 — Generate secrets

Run this to generate the two secrets:

```bash
echo "SESSION_SECRET=$(openssl rand -hex 32)"
echo "ENCRYPTION_KEY=$(openssl rand -hex 32)"
```

Paste the output values into `.env`.

---

## Part 4: Start Everything

### Step 1 — Start database and Redis

```bash
docker compose up -d postgres redis
```

Wait a few seconds, then verify:

```bash
docker compose ps
# Both should show "healthy"
```

### Step 2 — Run database migrations

```bash
# Install migrate tool if you don't have it
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path backend/migrations -database "postgres://droptodrop:droptodrop@localhost:5432/droptodrop?sslmode=disable" up
```

You should see: `1/u init_schema`

### Step 3 — Start the backend server

```bash
cd backend
go run ./cmd/server
```

You should see: `server starting addr=0.0.0.0:8080`

### Step 4 — Start the background worker

Open a new terminal:

```bash
cd backend
go run ./cmd/worker
```

You should see: `worker started`

### Step 5 — Start the frontend dev server

Open a new terminal:

```bash
cd frontend
npm run dev
```

You should see: `Local: http://localhost:3000`

### Step 6 — Verify health

```bash
curl http://localhost:8080/health
# {"status":"ok"}

curl http://localhost:8080/health/ready
# {"status":"healthy","checks":{"database":"ok","redis":"ok"}}
```

---

## Part 5: Create Development Stores

### Step 1 — Create Supplier Store

1. In Partner Dashboard, click **Stores** → **Add store**
2. Choose **Development store**
3. Set:
   - **Store name**: `droptodrop-supplier`
   - **Purpose**: anything
4. Click **Save**

### Step 2 — Create Reseller Store

Repeat the above with:
   - **Store name**: `droptodrop-reseller`

---

## Part 6: Install and Test

### Test 1 — Install on Supplier Store

1. In your browser, go to:
   ```
   https://abc123.ngrok-free.app/auth/install?shop=droptodrop-supplier.myshopify.com
   ```
2. Shopify will show the OAuth consent screen — click **Install app**
3. You should be redirected to the embedded app inside Shopify Admin
4. The **Onboarding** screen should appear — click **I'm a Supplier**

**What to verify:**
- [ ] OAuth redirects correctly
- [ ] No errors in the backend terminal
- [ ] Onboarding screen renders
- [ ] After choosing "Supplier", Dashboard loads with supplier navigation

### Test 2 — Supplier Setup

1. Click **Supplier Setup** in the nav
2. Enable supplier mode
3. Set processing days to 2
4. Click **Save**
5. Verify "Settings saved" banner appears

### Test 3 — Add Products to Marketplace

1. Click **Listings** in the nav
2. Click **Add Products** (top right)
3. The Product Picker modal should load products from your Shopify store
4. If the store has no products yet, create some in the Shopify Admin first:
   - Go to `https://droptodrop-supplier.myshopify.com/admin/products`
   - Click **Add product** → add title, price, variants
   - Come back to the DropToDrop listings page and click **Add Products** again
5. Select products, set wholesale prices, click **Create Listings**
6. Listings should appear in the table with status "draft"
7. Click **Publish** to make them "active"

### Test 4 — Install on Reseller Store

1. In a **different browser** or incognito window, go to:
   ```
   https://abc123.ngrok-free.app/auth/install?shop=droptodrop-reseller.myshopify.com
   ```
2. Install the app
3. Choose **I'm a Reseller**

### Test 5 — Import Products

1. Click **Marketplace** in the nav
2. The supplier's active listings should appear
3. Click **Import** on a product
4. Set markup (e.g. 30%)
5. Review the price preview
6. Click **Import Product**
7. Check **Imports** page — status should go from "pending" to "active"
8. Check the reseller's Shopify Admin → Products — the imported product should appear

### Test 6 — Order Flow

1. In the reseller's Shopify Admin, go to the imported product
2. Click **More actions** → **Create order** (or use the storefront)
3. Create a test order with a customer address
4. Watch the backend terminal — you should see the `orders/create` webhook arrive
5. The order should appear in:
   - **Reseller** → Orders
   - **Supplier** → Orders (as a routed order)

### Test 7 — Fulfill and Track

1. In the supplier's DropToDrop → **Orders**, click the new order
2. Click **Accept**
3. Click **Add Fulfillment**
4. Enter a tracking number (e.g. `1Z999AA10123456784`)
5. Enter tracking URL and carrier
6. Click **Submit Fulfillment**
7. The worker should sync the fulfillment to the reseller's Shopify store
8. Check:
   - Supplier order detail shows fulfillment event
   - Reseller order detail shows synced tracking
   - Reseller's Shopify Admin → Orders shows the fulfillment

### Test 8 — Uninstall

1. In the supplier store's Shopify Admin → Settings → Apps
2. Click **Delete** on DropToDrop
3. Watch the backend terminal for the `app/uninstalled` webhook
4. Verify the shop status changes to "uninstalled" in the database:
   ```bash
   psql $DATABASE_URL -c "SELECT shopify_domain, status FROM shops;"
   ```

### Test 9 — Reinstall

1. Repeat the install URL for the supplier store
2. Verify the app re-installs, status goes back to "active"
3. Previous data should still exist (soft-delete, not hard-delete)

---

## Part 7: Compliance Webhook Test

Shopify doesn't send compliance webhooks on demand, but you can simulate them:

```bash
# Generate HMAC for test payload
PAYLOAD='{"shop_id":1,"shop_domain":"droptodrop-supplier.myshopify.com","customer":{"id":1,"email":"test@example.com","phone":"+1234567890"},"orders_requested":[1001]}'

HMAC=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "YOUR_API_SECRET_HERE" -binary | base64)

# Test customers/data_request
curl -X POST https://abc123.ngrok-free.app/webhooks/compliance/customers-data-request \
  -H "Content-Type: application/json" \
  -H "X-Shopify-Hmac-Sha256: $HMAC" \
  -d "$PAYLOAD"

# Should return: {"status":"acknowledged"}
```

Repeat for `customers-redact` and `shop-redact` with appropriate payloads.

---

## Troubleshooting

### "invalid signature" on OAuth callback
- Make sure the `SHOPIFY_API_SECRET` in `.env` matches the Partner Dashboard
- Make sure you're using the tunnel URL, not localhost

### "shop not found" after install
- Check the backend logs for the OAuth flow
- Verify the shop was created: `psql $DATABASE_URL -c "SELECT * FROM shops;"`

### Webhooks not arriving
- Verify the tunnel is running and the URL is correct
- Check the Shopify Partner Dashboard → your app → **Webhooks** for delivery errors
- Check the backend logs for HMAC verification failures

### Product not appearing in reseller store
- Check the worker logs for `create_product` job output
- Check `reseller_imports` table: `psql $DATABASE_URL -c "SELECT id, status, last_sync_error FROM reseller_imports;"`
- Check `failed_jobs` table for errors

### "frame refused to connect" in Shopify Admin
- Verify `SHOPIFY_APP_URL` matches your tunnel URL exactly
- Check browser console for CSP errors

---

## Summary Checklist

After completing all tests, check off:

- [ ] Supplier OAuth install works
- [ ] Reseller OAuth install works
- [ ] Supplier can publish products
- [ ] Reseller sees products in marketplace
- [ ] Reseller can import products → product appears in Shopify store
- [ ] Product links exist in database
- [ ] Order webhook triggers routing
- [ ] Supplier sees routed order
- [ ] Supplier can accept and fulfill
- [ ] Tracking syncs to reseller store
- [ ] Uninstall webhook handled
- [ ] Reinstall works
- [ ] Compliance webhooks respond correctly
- [ ] All pages load without errors
- [ ] Audit log shows all events
