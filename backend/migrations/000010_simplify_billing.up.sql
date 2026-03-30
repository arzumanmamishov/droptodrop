DELETE FROM billing_plans;
INSERT INTO billing_plans (id, name, price_monthly, currency, max_products, max_orders_monthly, max_suppliers, app_fee_percent, trial_days, features) VALUES
('free', 'Free', 0, 'EUR', 5, 10, 2, 0, 0, '{}'),
('standard', 'Standard', 29, 'EUR', -1, -1, -1, 2.0, 14, '{"analytics": true}'),
('premium', 'Premium', 79, 'EUR', -1, -1, -1, 0, 14, '{"analytics": true, "priority_support": true}');
