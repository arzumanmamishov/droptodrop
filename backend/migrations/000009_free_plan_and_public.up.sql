-- Free plan
INSERT INTO billing_plans (id, name, price_monthly, max_products, max_orders_monthly, max_suppliers, app_fee_percent, trial_days, features)
VALUES ('free', 'Free', 0, 5, 10, 2, 3.0, 0, '{"analytics": false, "priority_support": false, "bulk_import": false}')
ON CONFLICT (id) DO NOTHING;

-- Platform stats cache
CREATE TABLE IF NOT EXISTS platform_stats (
    id INT PRIMARY KEY DEFAULT 1,
    total_products INT DEFAULT 0,
    total_orders INT DEFAULT 0,
    total_suppliers INT DEFAULT 0,
    total_resellers INT DEFAULT 0,
    total_revenue DECIMAL(12,2) DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
INSERT INTO platform_stats (id) VALUES (1) ON CONFLICT DO NOTHING;
