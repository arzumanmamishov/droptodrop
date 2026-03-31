-- Add stock validation tracking to routed orders
ALTER TABLE routed_orders ADD COLUMN IF NOT EXISTS stock_validated BOOLEAN DEFAULT FALSE;
ALTER TABLE routed_orders ADD COLUMN IF NOT EXISTS stock_failure_reason TEXT;
