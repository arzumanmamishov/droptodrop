-- Drop order events
DROP INDEX IF EXISTS idx_order_events_order;
DROP TABLE IF EXISTS order_events;

-- Drop payout records
DROP INDEX IF EXISTS idx_payouts_status;
DROP INDEX IF EXISTS idx_payouts_supplier;
DROP TABLE IF EXISTS payout_records;

-- Drop notifications
DROP INDEX IF EXISTS idx_notifications_shop;
DROP TABLE IF EXISTS notifications;

-- Drop supplier performance stats columns
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS reliability_score;
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS cancellation_count;
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS avg_fulfillment_hours;
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS total_orders_received;
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS total_orders_fulfilled;

-- Drop supplier verification columns
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS verified_at;
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS is_verified;

-- Drop disputes
DROP INDEX IF EXISTS idx_disputes_status;
DROP INDEX IF EXISTS idx_disputes_order;
DROP TABLE IF EXISTS disputes;
