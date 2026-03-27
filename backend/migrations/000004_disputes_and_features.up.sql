-- Disputes table
CREATE TABLE IF NOT EXISTS disputes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    routed_order_id UUID NOT NULL REFERENCES routed_orders(id),
    reporter_shop_id UUID NOT NULL REFERENCES shops(id),
    reporter_role VARCHAR(20) NOT NULL CHECK (reporter_role IN ('supplier', 'reseller')),
    dispute_type VARCHAR(50) NOT NULL CHECK (dispute_type IN ('quality_issue', 'wrong_item', 'not_received', 'damaged', 'late_delivery', 'missing_items', 'other')),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_review', 'resolved', 'closed')),
    description TEXT NOT NULL,
    resolution TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_disputes_order ON disputes(routed_order_id);
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes(status);

-- Supplier verification
ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

-- Supplier performance stats (cached/computed)
ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS total_orders_fulfilled INT DEFAULT 0;
ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS total_orders_received INT DEFAULT 0;
ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS avg_fulfillment_hours FLOAT DEFAULT 0;
ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS cancellation_count INT DEFAULT 0;
ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS reliability_score DECIMAL(3,1) DEFAULT 0;

-- In-app notifications
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'info' CHECK (type IN ('info', 'success', 'warning', 'error')),
    is_read BOOLEAN DEFAULT FALSE,
    link VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notifications_shop ON notifications(shop_id, is_read);

-- Payout tracking
CREATE TABLE IF NOT EXISTS payout_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    routed_order_id UUID NOT NULL REFERENCES routed_orders(id),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id),
    reseller_shop_id UUID NOT NULL REFERENCES shops(id),
    wholesale_amount DECIMAL(12,2) NOT NULL,
    platform_fee DECIMAL(12,2) DEFAULT 0,
    supplier_payout DECIMAL(12,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'paid', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_payouts_supplier ON payout_records(supplier_shop_id);
CREATE INDEX IF NOT EXISTS idx_payouts_status ON payout_records(status);

-- Order timeline events
CREATE TABLE IF NOT EXISTS order_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    routed_order_id UUID NOT NULL REFERENCES routed_orders(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    actor_type VARCHAR(20) DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_order_events_order ON order_events(routed_order_id);
