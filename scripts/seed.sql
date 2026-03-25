-- Seed data for local development and demo
-- Run after migrations: psql $DATABASE_URL < scripts/seed.sql

-- Supplier shop
INSERT INTO shops (id, shopify_domain, name, email, role, status, currency)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'supplier-demo.myshopify.com',
    'Demo Supplier Store',
    'supplier@demo.com',
    'supplier',
    'active',
    'USD'
) ON CONFLICT (shopify_domain) DO NOTHING;

-- Reseller shop
INSERT INTO shops (id, shopify_domain, name, email, role, status, currency)
VALUES (
    'b0000000-0000-0000-0000-000000000002',
    'reseller-demo.myshopify.com',
    'Demo Reseller Store',
    'reseller@demo.com',
    'reseller',
    'active',
    'USD'
) ON CONFLICT (shopify_domain) DO NOTHING;

-- Supplier profile
INSERT INTO supplier_profiles (shop_id, is_enabled, default_processing_days, blind_fulfillment, reseller_approval_mode, company_name, support_email)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    TRUE, 3, FALSE, 'auto', 'Demo Supplier Co', 'support@demosupplier.com'
) ON CONFLICT (shop_id) DO NOTHING;

-- Reseller profile
INSERT INTO reseller_profiles (shop_id, is_enabled, default_markup_type, default_markup_value, min_margin_percentage)
VALUES (
    'b0000000-0000-0000-0000-000000000002',
    TRUE, 'percentage', 30, 10
) ON CONFLICT (shop_id) DO NOTHING;

-- App installations (tokens are encrypted placeholders - not functional)
INSERT INTO app_installations (shop_id, access_token, scopes, is_active)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'encrypted_placeholder_supplier', 'read_products,write_products,read_orders,write_orders', TRUE),
    ('b0000000-0000-0000-0000-000000000002', 'encrypted_placeholder_reseller', 'read_products,write_products,read_orders,write_orders', TRUE)
ON CONFLICT DO NOTHING;

-- App settings
INSERT INTO app_settings (shop_id, support_email, privacy_policy_url)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'support@demosupplier.com', 'https://demosupplier.com/privacy'),
    ('b0000000-0000-0000-0000-000000000002', 'support@demoreseller.com', 'https://demoreseller.com/privacy')
ON CONFLICT DO NOTHING;

-- Session tokens for testing (1 year expiry)
INSERT INTO shop_sessions (shop_id, session_token, expires_at)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'dev_supplier_session_token', NOW() + INTERVAL '1 year'),
    ('b0000000-0000-0000-0000-000000000002', 'dev_reseller_session_token', NOW() + INTERVAL '1 year')
ON CONFLICT DO NOTHING;

-- Sample supplier listings
INSERT INTO supplier_listings (id, supplier_shop_id, shopify_product_id, title, description, product_type, vendor, status, processing_days, shipping_countries)
VALUES
    ('c0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 1001, 'Premium Wireless Headphones', 'High-quality bluetooth headphones with noise cancellation.', 'Electronics', 'Demo Supplier', 'active', 2, '["US","CA","GB"]'),
    ('c0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 1002, 'Organic Cotton T-Shirt', 'Sustainably sourced 100% organic cotton tee.', 'Apparel', 'Demo Supplier', 'active', 3, '["US","CA"]'),
    ('c0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001', 1003, 'Stainless Steel Water Bottle', 'Double-wall insulated 750ml water bottle.', 'Accessories', 'Demo Supplier', 'draft', 1, '["US"]')
ON CONFLICT (supplier_shop_id, shopify_product_id) DO NOTHING;

-- Sample listing variants
INSERT INTO supplier_listing_variants (listing_id, shopify_variant_id, title, sku, wholesale_price, suggested_retail_price, inventory_quantity)
VALUES
    ('c0000000-0000-0000-0000-000000000001', 2001, 'Black', 'WH-BLK-001', 45.00, 89.99, 150),
    ('c0000000-0000-0000-0000-000000000001', 2002, 'White', 'WH-WHT-001', 45.00, 89.99, 120),
    ('c0000000-0000-0000-0000-000000000002', 2003, 'S', 'TS-S-001', 12.00, 29.99, 200),
    ('c0000000-0000-0000-0000-000000000002', 2004, 'M', 'TS-M-001', 12.00, 29.99, 350),
    ('c0000000-0000-0000-0000-000000000002', 2005, 'L', 'TS-L-001', 12.00, 29.99, 180),
    ('c0000000-0000-0000-0000-000000000003', 2006, 'Default', 'WB-DEF-001', 8.50, 24.99, 500)
ON CONFLICT (listing_id, shopify_variant_id) DO NOTHING;

-- Sample audit logs
INSERT INTO audit_logs (shop_id, actor_type, action, resource_type, resource_id, outcome)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'system', 'oauth_complete', 'shop', 'a0000000-0000-0000-0000-000000000001', 'success'),
    ('a0000000-0000-0000-0000-000000000001', 'merchant', 'role_set', 'shop', 'a0000000-0000-0000-0000-000000000001', 'success'),
    ('a0000000-0000-0000-0000-000000000001', 'merchant', 'listing_created', 'supplier_listing', 'c0000000-0000-0000-0000-000000000001', 'success'),
    ('b0000000-0000-0000-0000-000000000002', 'system', 'oauth_complete', 'shop', 'b0000000-0000-0000-0000-000000000002', 'success'),
    ('b0000000-0000-0000-0000-000000000002', 'merchant', 'role_set', 'shop', 'b0000000-0000-0000-0000-000000000002', 'success');
