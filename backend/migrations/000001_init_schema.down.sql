-- Reverse migration: drop all tables in dependency order
DROP TRIGGER IF EXISTS update_background_jobs_updated_at ON background_jobs;
DROP TRIGGER IF EXISTS update_app_settings_updated_at ON app_settings;
DROP TRIGGER IF EXISTS update_compliance_events_updated_at ON compliance_events;
DROP TRIGGER IF EXISTS update_fulfillment_events_updated_at ON fulfillment_events;
DROP TRIGGER IF EXISTS update_routed_order_items_updated_at ON routed_order_items;
DROP TRIGGER IF EXISTS update_routed_orders_updated_at ON routed_orders;
DROP TRIGGER IF EXISTS update_product_links_updated_at ON product_links;
DROP TRIGGER IF EXISTS update_reseller_import_variants_updated_at ON reseller_import_variants;
DROP TRIGGER IF EXISTS update_reseller_imports_updated_at ON reseller_imports;
DROP TRIGGER IF EXISTS update_reseller_profiles_updated_at ON reseller_profiles;
DROP TRIGGER IF EXISTS update_supplier_listing_variants_updated_at ON supplier_listing_variants;
DROP TRIGGER IF EXISTS update_supplier_listings_updated_at ON supplier_listings;
DROP TRIGGER IF EXISTS update_supplier_profiles_updated_at ON supplier_profiles;
DROP TRIGGER IF EXISTS update_installations_updated_at ON app_installations;
DROP TRIGGER IF EXISTS update_shops_updated_at ON shops;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS failed_jobs;
DROP TABLE IF EXISTS background_jobs;
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS compliance_events;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS fulfillment_events;
DROP TABLE IF EXISTS routed_order_items;
DROP TABLE IF EXISTS routed_orders;
DROP TABLE IF EXISTS inventory_snapshots;
DROP TABLE IF EXISTS product_links;
DROP TABLE IF EXISTS reseller_import_variants;
DROP TABLE IF EXISTS reseller_imports;
DROP TABLE IF EXISTS reseller_profiles;
DROP TABLE IF EXISTS supplier_listing_variants;
DROP TABLE IF EXISTS supplier_listings;
DROP TABLE IF EXISTS supplier_profiles;
DROP TABLE IF EXISTS shop_sessions;
DROP TABLE IF EXISTS app_installations;
DROP TABLE IF EXISTS shops;
