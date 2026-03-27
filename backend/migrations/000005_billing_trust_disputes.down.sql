DROP TABLE IF EXISTS usage_summaries;
DROP TABLE IF EXISTS usage_records;
DROP TABLE IF EXISTS shop_subscriptions;
DROP TABLE IF EXISTS billing_plans;
DROP TABLE IF EXISTS supplier_stats;
ALTER TABLE disputes DROP COLUMN IF EXISTS supplier_shop_id;
ALTER TABLE disputes DROP COLUMN IF EXISTS reseller_shop_id;
