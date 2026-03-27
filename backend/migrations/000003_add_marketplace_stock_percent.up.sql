ALTER TABLE supplier_listings ADD COLUMN IF NOT EXISTS marketplace_stock_percent INT DEFAULT 100 CHECK (marketplace_stock_percent >= 0 AND marketplace_stock_percent <= 100);
