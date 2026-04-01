-- Reseller approval status per supplier
CREATE TABLE IF NOT EXISTS supplier_reseller_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    reseller_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'blocked')),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(supplier_shop_id, reseller_shop_id)
);
CREATE INDEX IF NOT EXISTS idx_supplier_reseller_supplier ON supplier_reseller_links(supplier_shop_id);
CREATE INDEX IF NOT EXISTS idx_supplier_reseller_reseller ON supplier_reseller_links(reseller_shop_id);
