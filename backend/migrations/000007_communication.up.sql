-- =============================================================================
-- IN-APP MESSAGING
-- =============================================================================

CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    reseller_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    subject VARCHAR(255) DEFAULT '',
    last_message_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(supplier_shop_id, reseller_shop_id)
);
CREATE INDEX IF NOT EXISTS idx_conversations_supplier ON conversations(supplier_shop_id);
CREATE INDEX IF NOT EXISTS idx_conversations_reseller ON conversations(reseller_shop_id);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_shop_id UUID NOT NULL REFERENCES shops(id),
    content TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at);

-- =============================================================================
-- ORDER COMMENTS
-- =============================================================================

CREATE TABLE IF NOT EXISTS order_comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    routed_order_id UUID NOT NULL REFERENCES routed_orders(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id),
    shop_role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_order_comments_order ON order_comments(routed_order_id, created_at);

-- =============================================================================
-- ANNOUNCEMENTS
-- =============================================================================

CREATE TABLE IF NOT EXISTS announcements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    is_pinned BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_announcements_supplier ON announcements(supplier_shop_id, created_at DESC);

CREATE TABLE IF NOT EXISTS announcement_reads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    announcement_id UUID NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(announcement_id, shop_id)
);
