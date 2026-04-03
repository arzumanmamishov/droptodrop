export interface Shop {
  id: string;
  shopify_domain: string;
  name: string;
  email: string;
  role: 'unset' | 'supplier' | 'reseller';
  status: string;
  currency: string;
  created_at: string;
  updated_at: string;
}

export interface SupplierProfile {
  id: string;
  shop_id: string;
  is_enabled: boolean;
  default_processing_days: number;
  shipping_countries: string[];
  blind_fulfillment: boolean;
  reseller_approval_mode: 'auto' | 'manual';
  company_name: string;
  support_email: string;
  return_policy_url: string;
  paypal_email: string;
}

export interface ResellerProfile {
  id: string;
  shop_id: string;
  is_enabled: boolean;
  default_markup_type: 'percentage' | 'fixed';
  default_markup_value: number;
  min_margin_percentage: number;
  auto_sync_inventory: boolean;
  auto_sync_price: boolean;
  auto_sync_content: boolean;
}

export interface ListingVariant {
  id: string;
  listing_id: string;
  shopify_variant_id: number;
  title: string;
  sku: string;
  wholesale_price: number;
  suggested_retail_price: number;
  inventory_quantity: number;
  weight: number;
  weight_unit: string;
  is_active: boolean;
}

export interface SupplierListing {
  id: string;
  supplier_shop_id: string;
  shopify_product_id: number;
  title: string;
  description: string;
  product_type: string;
  vendor: string;
  tags: string;
  images: string[];
  category: string;
  status: 'draft' | 'active' | 'paused' | 'archived';
  processing_days: number;
  marketplace_stock_percent: number;
  shipping_countries: string[];
  blind_fulfillment: boolean;
  variants: ListingVariant[];
  created_at: string;
  updated_at: string;
}

export interface ResellerImport {
  id: string;
  reseller_shop_id: string;
  supplier_listing_id: string;
  shopify_product_id: number | null;
  status: 'pending' | 'active' | 'paused' | 'failed' | 'removed';
  markup_type: string;
  markup_value: number;
  sync_images: boolean;
  sync_description: boolean;
  sync_title: boolean;
  last_sync_at: string | null;
  last_sync_error: string | null;
  supplier_title: string;
  supplier_images: string | Array<{ url?: string; URL?: string }>;
  supplier_shop_id: string;
  supplier_company_name: string;
  supplier_stock: number;
  supplier_price: number;
  created_at: string;
  updated_at: string;
}

export interface RoutedOrderItem {
  id: string;
  routed_order_id: string;
  reseller_line_item_id: number;
  supplier_variant_id: number;
  reseller_variant_id: number;
  title: string;
  sku: string;
  quantity: number;
  wholesale_unit_price: number;
  fulfillment_status: string;
  fulfilled_quantity: number;
}

export interface RoutedOrder {
  id: string;
  reseller_shop_id: string;
  supplier_shop_id: string;
  reseller_order_id: number;
  reseller_order_number: string;
  status: string;
  customer_shipping_name: string;
  customer_shipping_address: Record<string, string>;
  customer_email: string;
  customer_phone: string;
  total_wholesale_amount: number;
  currency: string;
  notes: string;
  items: RoutedOrderItem[];
  created_at: string;
  updated_at: string;
}

export interface FulfillmentEvent {
  id: string;
  routed_order_id: string;
  tracking_number: string;
  tracking_url: string;
  tracking_company: string;
  status: string;
  synced_to_reseller: boolean;
  synced_at: string;
  created_at: string;
  updated_at: string;
}

export interface AuditEntry {
  id: string;
  shop_id: string;
  actor_type: string;
  actor_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: Record<string, unknown>;
  outcome: string;
  error_payload: string;
  created_at: string;
}

export interface AppSettings {
  notifications_enabled: boolean;
  notification_email: string;
  support_email: string;
  privacy_policy_url: string;
  terms_url: string;
  data_retention_days: number;
  billing_plan: string;
}

export interface PaginatedResponse<T> {
  total: number;
  [key: string]: T[] | number;
}
