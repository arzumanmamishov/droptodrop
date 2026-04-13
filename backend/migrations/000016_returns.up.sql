CREATE TABLE IF NOT EXISTS return_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    routed_order_id UUID NOT NULL REFERENCES routed_orders(id),
    reseller_shop_id UUID NOT NULL REFERENCES shops(id),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id),
    status VARCHAR(20) NOT NULL DEFAULT 'requested',
    reason TEXT NOT NULL,
    customer_name VARCHAR(255),
    return_label_url TEXT,
    supplier_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_returns_reseller ON return_requests(reseller_shop_id);
CREATE INDEX IF NOT EXISTS idx_returns_supplier ON return_requests(supplier_shop_id);
CREATE INDEX IF NOT EXISTS idx_returns_order ON return_requests(routed_order_id);

-- status values: requested, label_uploaded, shipped_back, received, refunded, rejected
