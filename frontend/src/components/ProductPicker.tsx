import { useState, useCallback } from 'react';
import {
  Modal,
  BlockStack,
  Text,
  Banner,
  Spinner,
  DataTable,
  Button,
  Checkbox,
  InlineStack,
  TextField,
  Thumbnail,
  Select,
  Divider,
} from '@shopify/polaris';
import { ImageIcon } from '@shopify/polaris-icons';
import { api } from '../utils/api';
import { CATEGORY_OPTIONS } from '../constants/categories';

interface ShopVariant {
  id: number;
  gid: string;
  title: string;
  sku: string;
  price: string;
  inventory_quantity: number;
  weight: number;
  weight_unit: string;
}

interface ShopProductImage {
  url: string;
  alt_text: string;
}

interface ShopProduct {
  id: number;
  gid: string;
  title: string;
  description: string;
  product_type: string;
  vendor: string;
  tags: string;
  status: string;
  images: ShopProductImage[];
  variants: ShopVariant[];
}

interface ProductPickerProps {
  open: boolean;
  onClose: () => void;
  onImport: () => void;
}

/**
 * ProductPicker fetches products from the supplier's Shopify store and
 * lets them select which ones to publish as supplier listings.
 *
 * When running inside Shopify Admin with App Bridge v4, it could use
 * `window.shopify.resourcePicker()` instead. This component provides
 * a fallback that works in all environments.
 */
export default function ProductPicker({ open, onClose, onImport }: ProductPickerProps) {
  const [products, setProducts] = useState<ShopProduct[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [importing, setImporting] = useState(false);
  const [wholesalePrices, setWholesalePrices] = useState<Record<string, string>>({});
  const [categories, setCategories] = useState<Record<number, string>>({});

  const fetchProducts = useCallback(async (cursor?: string) => {
    setLoading(true);
    setError(null);
    try {
      const params = cursor ? `?cursor=${cursor}` : '';
      const data = await api.get<{ products: ShopProduct[]; next_cursor: string }>(
        `/supplier/shop-products${params}`,
      );
      if (cursor) {
        setProducts((prev) => [...prev, ...(data.products || [])]);
      } else {
        setProducts(data.products || []);
      }
      setNextCursor(data.next_cursor || null);
      setLoaded(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load products');
    } finally {
      setLoading(false);
    }
  }, []);

  const handleOpen = useCallback(() => {
    if (!loaded) {
      fetchProducts();
    }
  }, [loaded, fetchProducts]);

  // Trigger fetch when modal opens
  if (open && !loaded && !loading) {
    handleOpen();
  }

  const toggleSelect = useCallback((productId: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(productId)) {
        next.delete(productId);
      } else {
        next.add(productId);
      }
      return next;
    });
  }, []);

  const handleWholesaleChange = useCallback((variantKey: string, value: string) => {
    setWholesalePrices((prev) => ({ ...prev, [variantKey]: value }));
  }, []);

  const handleImport = useCallback(async () => {
    setImporting(true);
    setError(null);
    try {
      for (const product of products.filter((p) => selected.has(p.id))) {
        const variants = product.variants.map((v) => {
          const wholesaleKey = `${product.id}-${v.id}`;
          const wholesale = parseFloat(wholesalePrices[wholesaleKey] || v.price);
          return {
            shopify_variant_id: v.id,
            title: v.title,
            sku: v.sku,
            wholesale_price: isNaN(wholesale) ? parseFloat(v.price) : wholesale,
            suggested_retail_price: parseFloat(v.price),
            inventory_quantity: v.inventory_quantity,
            weight: v.weight,
            weight_unit: v.weight_unit || 'kg',
          };
        });

        const images = (product.images || []).map((img) => ({
          url: img.url,
          altText: img.alt_text,
        }));

        await api.post('/supplier/listings', {
          shopify_product_id: product.id,
          title: product.title,
          description: product.description,
          product_type: product.product_type,
          vendor: product.vendor,
          tags: product.tags,
          images: JSON.stringify(images),
          category: categories[product.id] || 'other',
          processing_days: 3,
          shipping_countries: ['US'],
          blind_fulfillment: false,
          variants,
        });
      }
      onImport();
      onClose();
      setSelected(new Set());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Import failed');
    } finally {
      setImporting(false);
    }
  }, [products, selected, wholesalePrices, categories, onImport, onClose]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Select Products from Your Store"
      primaryAction={{
        content: `Create ${selected.size} Listing${selected.size !== 1 ? 's' : ''}`,
        onAction: handleImport,
        loading: importing,
        disabled: selected.size === 0,
      }}
      secondaryActions={[{ content: 'Cancel', onAction: onClose }]}
    >
      <Modal.Section>
        <BlockStack gap="400">
          {error && <Banner tone="critical">{error}</Banner>}

          {loading && !loaded ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '2rem' }}>
              <Spinner size="large" />
            </div>
          ) : products.length > 0 ? (
            <>
              <Text as="p" variant="bodySm" tone="subdued">
                Select products to list for resellers. Set wholesale prices per variant.
              </Text>
              {products.map((product) => (
                <BlockStack gap="200" key={product.id}>
                  <InlineStack gap="300" align="start" blockAlign="center">
                    <Checkbox
                      label=""
                      checked={selected.has(product.id)}
                      onChange={() => toggleSelect(product.id)}
                    />
                    <Thumbnail
                      source={product.images?.[0]?.url || ImageIcon}
                      alt={product.title}
                      size="small"
                    />
                    <BlockStack gap="100">
                      <Text as="span" variant="headingSm" fontWeight="semibold">
                        {product.title}
                      </Text>
                      <Text as="span" variant="bodySm" tone="subdued">
                        {product.product_type || 'No type'} · {product.vendor || 'No vendor'} · {product.status}
                      </Text>
                    </BlockStack>
                  </InlineStack>
                  {selected.has(product.id) && (
                    <div style={{ paddingLeft: '2rem', maxWidth: '250px' }}>
                      <Select
                        label="Category"
                        options={CATEGORY_OPTIONS}
                        value={categories[product.id] || 'other'}
                        onChange={(val) => setCategories((prev) => ({ ...prev, [product.id]: val }))}
                      />
                    </div>
                  )}
                  <Divider />
                  {selected.has(product.id) && product.variants.length > 0 && (
                    <div style={{ paddingLeft: '2rem' }}>
                      <DataTable
                        columnContentTypes={['text', 'text', 'numeric', 'text']}
                        headings={['Variant', 'SKU', 'Retail Price', 'Wholesale Price']}
                        rows={product.variants.map((v) => {
                          const key = `${product.id}-${v.id}`;
                          return [
                            v.title || 'Default',
                            v.sku || '-',
                            `$${v.price}`,
                            <TextField
                              key={key}
                              label=""
                              labelHidden
                              type="number"
                              value={wholesalePrices[key] ?? v.price}
                              onChange={(val) => handleWholesaleChange(key, val)}
                              prefix="$"
                              autoComplete="off"
                              size="slim"
                            />,
                          ];
                        })}
                      />
                    </div>
                  )}
                </BlockStack>
              ))}
              {nextCursor && (
                <InlineStack align="center">
                  <Button loading={loading} onClick={() => fetchProducts(nextCursor)}>
                    Load More
                  </Button>
                </InlineStack>
              )}
            </>
          ) : (
            <Text as="p" tone="subdued">No products found in your store.</Text>
          )}
        </BlockStack>
      </Modal.Section>
    </Modal>
  );
}
