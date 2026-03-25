-- ============================================================================
-- DropToDrop: Initial Schema Migration
-- Production-grade PostgreSQL schema for Shopify dropshipping network
-- ============================================================================

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- SHOPS & INSTALLATIONS
-- ============================================================================

CREATE TABLE shops (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shopify_domain  VARCHAR(255) NOT NULL UNIQUE,
    shopify_shop_id BIGINT UNIQUE,
    name            VARCHAR(255),
    email           VARCHAR(255),
    role            VARCHAR(20) NOT NULL DEFAULT 'unset' CHECK (role IN ('unset', 'supplier', 'reseller')),
    status          VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'suspended', 'uninstalled')),
    currency        VARCHAR(10) DEFAULT 'USD',
    timezone        VARCHAR(100),
    country         VARCHAR(10),
    plan_name       VARCHAR(100),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shops_role ON shops(role);
CREATE INDEX idx_shops_status ON shops(status);

CREATE TABLE app_installations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id         UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    access_token    TEXT NOT NULL, -- encrypted at application layer
    scopes          TEXT NOT NULL,
    installed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    uninstalled_at  TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_installations_shop ON app_installations(shop_id);
CREATE INDEX idx_installations_active ON app_installations(is_active);

CREATE TABLE shop_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id         UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    session_token   VARCHAR(512) NOT NULL UNIQUE,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_token ON shop_sessions(session_token);
CREATE INDEX idx_sessions_expires ON shop_sessions(expires_at);

-- ============================================================================
-- SUPPLIER PROFILES & LISTINGS
-- ============================================================================

CREATE TABLE supplier_profiles (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id                 UUID NOT NULL UNIQUE REFERENCES shops(id) ON DELETE CASCADE,
    is_enabled              BOOLEAN NOT NULL DEFAULT FALSE,
    default_processing_days INT NOT NULL DEFAULT 3 CHECK (default_processing_days >= 0),
    shipping_countries      JSONB NOT NULL DEFAULT '[]',
    blind_fulfillment       BOOLEAN NOT NULL DEFAULT FALSE,
    reseller_approval_mode  VARCHAR(20) NOT NULL DEFAULT 'auto' CHECK (reseller_approval_mode IN ('auto', 'manual')),
    min_reseller_rating     DECIMAL(3,2) DEFAULT 0,
    company_name            VARCHAR(255),
    support_email           VARCHAR(255),
    return_policy_url       TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE supplier_listings (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id        UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    shopify_product_id      BIGINT NOT NULL,
    title                   VARCHAR(500) NOT NULL,
    description             TEXT,
    product_type            VARCHAR(255),
    vendor                  VARCHAR(255),
    tags                    TEXT,
    images                  JSONB DEFAULT '[]',
    status                  VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'archived')),
    processing_days         INT NOT NULL DEFAULT 3,
    shipping_countries      JSONB DEFAULT '[]',
    blind_fulfillment       BOOLEAN NOT NULL DEFAULT FALSE,
    eligibility_rules       JSONB DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(supplier_shop_id, shopify_product_id)
);

CREATE INDEX idx_listings_supplier ON supplier_listings(supplier_shop_id);
CREATE INDEX idx_listings_status ON supplier_listings(status);
CREATE INDEX idx_listings_product_id ON supplier_listings(shopify_product_id);

CREATE TABLE supplier_listing_variants (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    listing_id              UUID NOT NULL REFERENCES supplier_listings(id) ON DELETE CASCADE,
    shopify_variant_id      BIGINT NOT NULL,
    title                   VARCHAR(500),
    sku                     VARCHAR(255),
    wholesale_price         DECIMAL(12,2) NOT NULL CHECK (wholesale_price >= 0),
    suggested_retail_price  DECIMAL(12,2),
    inventory_quantity      INT NOT NULL DEFAULT 0,
    weight                  DECIMAL(10,4),
    weight_unit             VARCHAR(10) DEFAULT 'kg',
    requires_shipping       BOOLEAN NOT NULL DEFAULT TRUE,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(listing_id, shopify_variant_id)
);

CREATE INDEX idx_listing_variants_listing ON supplier_listing_variants(listing_id);

-- ============================================================================
-- RESELLER PROFILES & IMPORTS
-- ============================================================================

CREATE TABLE reseller_profiles (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id                 UUID NOT NULL UNIQUE REFERENCES shops(id) ON DELETE CASCADE,
    is_enabled              BOOLEAN NOT NULL DEFAULT FALSE,
    default_markup_type     VARCHAR(20) NOT NULL DEFAULT 'percentage' CHECK (default_markup_type IN ('percentage', 'fixed')),
    default_markup_value    DECIMAL(12,2) NOT NULL DEFAULT 30,
    min_margin_percentage   DECIMAL(5,2) NOT NULL DEFAULT 10,
    auto_sync_inventory     BOOLEAN NOT NULL DEFAULT TRUE,
    auto_sync_price         BOOLEAN NOT NULL DEFAULT FALSE,
    auto_sync_content       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE reseller_imports (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reseller_shop_id        UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    supplier_listing_id     UUID NOT NULL REFERENCES supplier_listings(id),
    shopify_product_id      BIGINT, -- product ID in reseller's store
    status                  VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'paused', 'failed', 'removed')),
    markup_type             VARCHAR(20) NOT NULL DEFAULT 'percentage',
    markup_value            DECIMAL(12,2) NOT NULL DEFAULT 30,
    sync_images             BOOLEAN NOT NULL DEFAULT TRUE,
    sync_description        BOOLEAN NOT NULL DEFAULT TRUE,
    sync_title              BOOLEAN NOT NULL DEFAULT FALSE,
    last_sync_at            TIMESTAMPTZ,
    last_sync_error         TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(reseller_shop_id, supplier_listing_id)
);

CREATE INDEX idx_imports_reseller ON reseller_imports(reseller_shop_id);
CREATE INDEX idx_imports_listing ON reseller_imports(supplier_listing_id);
CREATE INDEX idx_imports_status ON reseller_imports(status);

CREATE TABLE reseller_import_variants (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    import_id               UUID NOT NULL REFERENCES reseller_imports(id) ON DELETE CASCADE,
    supplier_variant_id     UUID NOT NULL REFERENCES supplier_listing_variants(id),
    shopify_variant_id      BIGINT, -- variant ID in reseller's store
    reseller_price          DECIMAL(12,2) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(import_id, supplier_variant_id)
);

CREATE INDEX idx_import_variants_import ON reseller_import_variants(import_id);

-- ============================================================================
-- PRODUCT LINKS (canonical mapping between supplier and reseller products)
-- ============================================================================

CREATE TABLE product_links (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id        UUID NOT NULL REFERENCES shops(id),
    reseller_shop_id        UUID NOT NULL REFERENCES shops(id),
    supplier_product_id     BIGINT NOT NULL,
    reseller_product_id     BIGINT NOT NULL,
    supplier_variant_id     BIGINT NOT NULL,
    reseller_variant_id     BIGINT NOT NULL,
    import_id               UUID REFERENCES reseller_imports(id) ON DELETE SET NULL,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(reseller_shop_id, reseller_variant_id)
);

CREATE INDEX idx_links_supplier ON product_links(supplier_shop_id, supplier_variant_id);
CREATE INDEX idx_links_reseller ON product_links(reseller_shop_id, reseller_variant_id);

-- ============================================================================
-- INVENTORY SNAPSHOTS
-- ============================================================================

CREATE TABLE inventory_snapshots (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id        UUID NOT NULL REFERENCES shops(id),
    shopify_variant_id      BIGINT NOT NULL,
    shopify_inventory_item_id BIGINT,
    quantity                INT NOT NULL,
    recorded_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inventory_supplier_variant ON inventory_snapshots(supplier_shop_id, shopify_variant_id);
CREATE INDEX idx_inventory_recorded ON inventory_snapshots(recorded_at);

-- ============================================================================
-- ORDER ROUTING
-- ============================================================================

CREATE TABLE routed_orders (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reseller_shop_id        UUID NOT NULL REFERENCES shops(id),
    supplier_shop_id        UUID NOT NULL REFERENCES shops(id),
    reseller_order_id       BIGINT NOT NULL,
    reseller_order_number   VARCHAR(100),
    status                  VARCHAR(30) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'rejected', 'processing', 'partially_fulfilled', 'fulfilled', 'cancelled')),
    customer_shipping_name  VARCHAR(500),
    customer_shipping_address JSONB,
    customer_email          VARCHAR(255),
    customer_phone          VARCHAR(100),
    total_wholesale_amount  DECIMAL(12,2),
    currency                VARCHAR(10) DEFAULT 'USD',
    idempotency_key         VARCHAR(255) NOT NULL UNIQUE,
    notes                   TEXT,
    supplier_notified_at    TIMESTAMPTZ,
    accepted_at             TIMESTAMPTZ,
    rejected_at             TIMESTAMPTZ,
    fulfilled_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routed_reseller ON routed_orders(reseller_shop_id);
CREATE INDEX idx_routed_supplier ON routed_orders(supplier_shop_id);
CREATE INDEX idx_routed_status ON routed_orders(status);
CREATE INDEX idx_routed_reseller_order ON routed_orders(reseller_order_id);

CREATE TABLE routed_order_items (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    routed_order_id         UUID NOT NULL REFERENCES routed_orders(id) ON DELETE CASCADE,
    reseller_line_item_id   BIGINT NOT NULL,
    supplier_variant_id     BIGINT NOT NULL,
    reseller_variant_id     BIGINT NOT NULL,
    product_link_id         UUID REFERENCES product_links(id),
    title                   VARCHAR(500),
    sku                     VARCHAR(255),
    quantity                INT NOT NULL CHECK (quantity > 0),
    wholesale_unit_price    DECIMAL(12,2) NOT NULL,
    fulfillment_status      VARCHAR(30) NOT NULL DEFAULT 'unfulfilled'
        CHECK (fulfillment_status IN ('unfulfilled', 'fulfilled', 'partially_fulfilled', 'cancelled')),
    fulfilled_quantity      INT NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order ON routed_order_items(routed_order_id);

-- ============================================================================
-- FULFILLMENT EVENTS
-- ============================================================================

CREATE TABLE fulfillment_events (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    routed_order_id         UUID NOT NULL REFERENCES routed_orders(id),
    shopify_fulfillment_id  BIGINT,
    tracking_number         VARCHAR(500),
    tracking_url            TEXT,
    tracking_company        VARCHAR(255),
    status                  VARCHAR(30) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'in_transit', 'delivered', 'failure', 'cancelled')),
    synced_to_reseller      BOOLEAN NOT NULL DEFAULT FALSE,
    synced_at               TIMESTAMPTZ,
    sync_error              TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fulfillment_order ON fulfillment_events(routed_order_id);
CREATE INDEX idx_fulfillment_synced ON fulfillment_events(synced_to_reseller);

-- ============================================================================
-- WEBHOOKS & COMPLIANCE
-- ============================================================================

CREATE TABLE webhook_events (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id                 UUID REFERENCES shops(id),
    topic                   VARCHAR(100) NOT NULL,
    shopify_webhook_id      VARCHAR(100),
    payload_hash            VARCHAR(64),
    status                  VARCHAR(20) NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'processing', 'processed', 'failed', 'skipped')),
    error_message           TEXT,
    attempts                INT NOT NULL DEFAULT 0,
    processed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_topic ON webhook_events(topic);
CREATE INDEX idx_webhook_status ON webhook_events(status);
CREATE INDEX idx_webhook_shop ON webhook_events(shop_id);
CREATE INDEX idx_webhook_hash ON webhook_events(payload_hash);

CREATE TABLE compliance_events (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id                 UUID REFERENCES shops(id),
    event_type              VARCHAR(50) NOT NULL
        CHECK (event_type IN ('customers_data_request', 'customers_redact', 'shop_redact')),
    shopify_request_id      VARCHAR(255),
    payload                 JSONB NOT NULL DEFAULT '{}',
    status                  VARCHAR(20) NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'processing', 'completed', 'failed')),
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_compliance_type ON compliance_events(event_type);
CREATE INDEX idx_compliance_shop ON compliance_events(shop_id);

-- ============================================================================
-- AUDIT LOGS
-- ============================================================================

CREATE TABLE audit_logs (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id                 UUID REFERENCES shops(id),
    actor_type              VARCHAR(30) NOT NULL DEFAULT 'system'
        CHECK (actor_type IN ('system', 'merchant', 'webhook', 'worker', 'admin')),
    actor_id                VARCHAR(255),
    action                  VARCHAR(100) NOT NULL,
    resource_type           VARCHAR(100),
    resource_id             VARCHAR(255),
    details                 JSONB DEFAULT '{}',
    outcome                 VARCHAR(20) NOT NULL DEFAULT 'success'
        CHECK (outcome IN ('success', 'failure', 'error')),
    error_payload           TEXT,
    ip_address              VARCHAR(45),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_shop ON audit_logs(shop_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_created ON audit_logs(created_at);

-- ============================================================================
-- APP SETTINGS
-- ============================================================================

CREATE TABLE app_settings (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id                 UUID NOT NULL UNIQUE REFERENCES shops(id) ON DELETE CASCADE,
    notifications_enabled   BOOLEAN NOT NULL DEFAULT TRUE,
    notification_email      VARCHAR(255),
    support_email           VARCHAR(255),
    privacy_policy_url      TEXT,
    terms_url               TEXT,
    data_retention_days     INT NOT NULL DEFAULT 365,
    billing_plan            VARCHAR(50) DEFAULT 'free',
    billing_status          VARCHAR(20) DEFAULT 'inactive',
    preferences             JSONB DEFAULT '{}',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- BACKGROUND JOBS / FAILED JOBS
-- ============================================================================

CREATE TABLE background_jobs (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    queue                   VARCHAR(100) NOT NULL DEFAULT 'default',
    job_type                VARCHAR(100) NOT NULL,
    payload                 JSONB NOT NULL DEFAULT '{}',
    status                  VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'failed', 'dead')),
    attempts                INT NOT NULL DEFAULT 0,
    max_attempts            INT NOT NULL DEFAULT 3,
    last_error              TEXT,
    run_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jobs_queue_status ON background_jobs(queue, status);
CREATE INDEX idx_jobs_run_at ON background_jobs(run_at);
CREATE INDEX idx_jobs_type ON background_jobs(job_type);

CREATE TABLE failed_jobs (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    original_job_id         UUID,
    queue                   VARCHAR(100) NOT NULL,
    job_type                VARCHAR(100) NOT NULL,
    payload                 JSONB NOT NULL DEFAULT '{}',
    error                   TEXT NOT NULL,
    failed_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_failed_jobs_type ON failed_jobs(job_type);

-- ============================================================================
-- Updated-at trigger function
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply updated_at triggers to all mutable tables
CREATE TRIGGER update_shops_updated_at BEFORE UPDATE ON shops FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_installations_updated_at BEFORE UPDATE ON app_installations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_supplier_profiles_updated_at BEFORE UPDATE ON supplier_profiles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_supplier_listings_updated_at BEFORE UPDATE ON supplier_listings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_supplier_listing_variants_updated_at BEFORE UPDATE ON supplier_listing_variants FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_reseller_profiles_updated_at BEFORE UPDATE ON reseller_profiles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_reseller_imports_updated_at BEFORE UPDATE ON reseller_imports FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_reseller_import_variants_updated_at BEFORE UPDATE ON reseller_import_variants FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_product_links_updated_at BEFORE UPDATE ON product_links FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_routed_orders_updated_at BEFORE UPDATE ON routed_orders FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_routed_order_items_updated_at BEFORE UPDATE ON routed_order_items FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_fulfillment_events_updated_at BEFORE UPDATE ON fulfillment_events FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_compliance_events_updated_at BEFORE UPDATE ON compliance_events FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_app_settings_updated_at BEFORE UPDATE ON app_settings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_background_jobs_updated_at BEFORE UPDATE ON background_jobs FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
