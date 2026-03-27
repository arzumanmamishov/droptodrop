-- =============================================================================
-- BILLING SYSTEM
-- =============================================================================

-- Subscription plans definition
CREATE TABLE IF NOT EXISTS billing_plans (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price_monthly DECIMAL(10,2) NOT NULL,
    currency VARCHAR(10) DEFAULT 'EUR',
    max_products INT NOT NULL DEFAULT 10,
    max_orders_monthly INT NOT NULL DEFAULT 50,
    max_suppliers INT NOT NULL DEFAULT 3,
    app_fee_percent DECIMAL(5,2) NOT NULL DEFAULT 2.0,
    trial_days INT DEFAULT 14,
    features JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default plans
INSERT INTO billing_plans (id, name, price_monthly, max_products, max_orders_monthly, max_suppliers, app_fee_percent, trial_days, features) VALUES
    ('starter', 'Starter', 29.00, 25, 100, 5, 2.5, 14, '{"analytics": false, "priority_support": false}'),
    ('growth', 'Growth', 49.00, 100, 500, 20, 2.0, 14, '{"analytics": true, "priority_support": false}'),
    ('pro', 'Pro', 99.00, -1, -1, -1, 1.5, 14, '{"analytics": true, "priority_support": true}')
ON CONFLICT (id) DO NOTHING;

-- Shop subscriptions
CREATE TABLE IF NOT EXISTS shop_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    plan_id VARCHAR(50) NOT NULL REFERENCES billing_plans(id),
    shopify_charge_id BIGINT,
    status VARCHAR(30) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'frozen', 'cancelled', 'expired')),
    trial_ends_at TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_shop ON shop_subscriptions(shop_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_active ON shop_subscriptions(shop_id) WHERE status = 'active';

-- Usage tracking per order
CREATE TABLE IF NOT EXISTS usage_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id),
    routed_order_id UUID REFERENCES routed_orders(id),
    order_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    fee_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    fee_percent DECIMAL(5,2) NOT NULL DEFAULT 2.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_usage_shop ON usage_records(shop_id);
CREATE INDEX IF NOT EXISTS idx_usage_created ON usage_records(created_at);

-- Monthly usage summary (cached)
CREATE TABLE IF NOT EXISTS usage_summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id),
    month VARCHAR(7) NOT NULL,
    order_count INT DEFAULT 0,
    product_count INT DEFAULT 0,
    total_fees DECIMAL(12,2) DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(shop_id, month)
);

-- =============================================================================
-- TRUST SYSTEM - Supplier Stats (separate table for clean queries)
-- =============================================================================

CREATE TABLE IF NOT EXISTS supplier_stats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id UUID NOT NULL UNIQUE REFERENCES shops(id) ON DELETE CASCADE,
    total_orders INT DEFAULT 0,
    fulfilled_orders INT DEFAULT 0,
    cancelled_orders INT DEFAULT 0,
    disputed_orders INT DEFAULT 0,
    avg_fulfillment_hours FLOAT DEFAULT 0,
    fulfillment_rate DECIMAL(5,2) DEFAULT 0,
    dispute_rate DECIMAL(5,2) DEFAULT 0,
    score INT DEFAULT 50 CHECK (score >= 0 AND score <= 100),
    label VARCHAR(30) DEFAULT 'New',
    last_updated TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================================================
-- DISPUTE SYSTEM ENHANCEMENTS
-- =============================================================================

-- Add supplier_shop_id and reseller_shop_id to disputes if not exists
DO $$ BEGIN
    ALTER TABLE disputes ADD COLUMN IF NOT EXISTS supplier_shop_id UUID REFERENCES shops(id);
    ALTER TABLE disputes ADD COLUMN IF NOT EXISTS reseller_shop_id UUID REFERENCES shops(id);
EXCEPTION WHEN others THEN NULL;
END $$;

-- Update existing disputes with shop IDs from routed orders
UPDATE disputes d SET
    supplier_shop_id = ro.supplier_shop_id,
    reseller_shop_id = ro.reseller_shop_id
FROM routed_orders ro
WHERE d.routed_order_id = ro.id AND d.supplier_shop_id IS NULL;
