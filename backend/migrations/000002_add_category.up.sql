ALTER TABLE supplier_listings ADD COLUMN IF NOT EXISTS category VARCHAR(100) DEFAULT 'other';
CREATE INDEX IF NOT EXISTS idx_listings_category ON supplier_listings(category);
