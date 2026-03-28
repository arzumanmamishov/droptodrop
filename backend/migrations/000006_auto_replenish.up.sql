-- Auto-replenishment settings per import
ALTER TABLE reseller_imports ADD COLUMN IF NOT EXISTS auto_replenish BOOLEAN DEFAULT FALSE;
ALTER TABLE reseller_imports ADD COLUMN IF NOT EXISTS replenish_threshold INT DEFAULT 5;
ALTER TABLE reseller_imports ADD COLUMN IF NOT EXISTS replenish_quantity INT DEFAULT 20;

-- Multi-store support: link multiple stores to one supplier account
ALTER TABLE shops ADD COLUMN IF NOT EXISTS parent_shop_id UUID REFERENCES shops(id);
CREATE INDEX IF NOT EXISTS idx_shops_parent ON shops(parent_shop_id);
