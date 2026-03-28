-- =============================================================================
-- REVIEWS & RATINGS
-- =============================================================================

CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id),
    reseller_shop_id UUID NOT NULL REFERENCES shops(id),
    routed_order_id UUID REFERENCES routed_orders(id),
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    title VARCHAR(255),
    comment TEXT,
    is_public BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(reseller_shop_id, routed_order_id)
);
CREATE INDEX IF NOT EXISTS idx_reviews_supplier ON reviews(supplier_shop_id);

-- =============================================================================
-- AUTO PRICE SYNC RULES
-- =============================================================================

ALTER TABLE reseller_imports ADD COLUMN IF NOT EXISTS auto_price_sync BOOLEAN DEFAULT FALSE;
ALTER TABLE reseller_imports ADD COLUMN IF NOT EXISTS price_sync_mode VARCHAR(20) DEFAULT 'maintain_margin';
-- price_sync_mode: 'maintain_margin' (keeps same % margin), 'maintain_markup' (keeps same $ markup), 'manual' (no auto)

-- =============================================================================
-- SHIPPING RULES
-- =============================================================================

CREATE TABLE IF NOT EXISTS shipping_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    country_code VARCHAR(10) NOT NULL,
    shipping_rate DECIMAL(10,2) NOT NULL DEFAULT 0,
    free_shipping_threshold DECIMAL(10,2),
    estimated_days_min INT DEFAULT 3,
    estimated_days_max INT DEFAULT 7,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(supplier_shop_id, country_code)
);

-- =============================================================================
-- SAMPLE ORDERS
-- =============================================================================

CREATE TABLE IF NOT EXISTS sample_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reseller_shop_id UUID NOT NULL REFERENCES shops(id),
    supplier_listing_id UUID NOT NULL REFERENCES supplier_listings(id),
    status VARCHAR(20) DEFAULT 'requested' CHECK (status IN ('requested', 'approved', 'shipped', 'received', 'rejected')),
    quantity INT DEFAULT 1,
    notes TEXT,
    tracking_number VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_samples_reseller ON sample_orders(reseller_shop_id);
CREATE INDEX IF NOT EXISTS idx_samples_listing ON sample_orders(supplier_listing_id);

-- =============================================================================
-- EXCLUSIVE DEALS
-- =============================================================================

CREATE TABLE IF NOT EXISTS deals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    supplier_listing_id UUID REFERENCES supplier_listings(id),
    title VARCHAR(255) NOT NULL,
    discount_type VARCHAR(20) NOT NULL CHECK (discount_type IN ('percentage', 'fixed')),
    discount_value DECIMAL(10,2) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at TIMESTAMPTZ NOT NULL,
    max_uses INT DEFAULT 0,
    current_uses INT DEFAULT 0,
    target_reseller_id UUID REFERENCES shops(id),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_deals_supplier ON deals(supplier_shop_id);
CREATE INDEX IF NOT EXISTS idx_deals_active ON deals(is_active, ends_at);

-- =============================================================================
-- PRODUCT BUNDLES
-- =============================================================================

CREATE TABLE IF NOT EXISTS product_bundles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reseller_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    bundle_price DECIMAL(12,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bundle_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bundle_id UUID NOT NULL REFERENCES product_bundles(id) ON DELETE CASCADE,
    supplier_listing_id UUID NOT NULL REFERENCES supplier_listings(id),
    quantity INT DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_bundle_items_bundle ON bundle_items(bundle_id);

-- =============================================================================
-- WHITE-LABEL INVOICING
-- =============================================================================

ALTER TABLE reseller_profiles ADD COLUMN IF NOT EXISTS invoice_company_name VARCHAR(255);
ALTER TABLE reseller_profiles ADD COLUMN IF NOT EXISTS invoice_address TEXT;
ALTER TABLE reseller_profiles ADD COLUMN IF NOT EXISTS invoice_logo_url TEXT;
ALTER TABLE reseller_profiles ADD COLUMN IF NOT EXISTS invoice_footer TEXT;
