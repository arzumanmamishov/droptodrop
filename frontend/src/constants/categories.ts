export const PRODUCT_CATEGORIES = [
  { value: 'all', label: 'All Products' },
  { value: 'apparel', label: 'Apparel & Clothing' },
  { value: 'accessories', label: 'Accessories & Jewelry' },
  { value: 'electronics', label: 'Electronics & Gadgets' },
  { value: 'home_garden', label: 'Home & Garden' },
  { value: 'beauty', label: 'Beauty & Personal Care' },
  { value: 'sports', label: 'Sports & Outdoors' },
  { value: 'toys', label: 'Toys & Games' },
  { value: 'pet', label: 'Pet Supplies' },
  { value: 'food', label: 'Food & Beverages' },
  { value: 'health', label: 'Health & Wellness' },
  { value: 'automotive', label: 'Automotive' },
  { value: 'office', label: 'Office & Stationery' },
  { value: 'other', label: 'Other' },
] as const;

export const CATEGORY_OPTIONS = PRODUCT_CATEGORIES.filter(c => c.value !== 'all').map(c => ({
  label: c.label,
  value: c.value,
}));

export function getCategoryLabel(value: string): string {
  return PRODUCT_CATEGORIES.find(c => c.value === value)?.label || value;
}
