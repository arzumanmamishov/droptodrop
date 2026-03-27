import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Page,
  Layout,
  Card,
  FormLayout,
  TextField,
  Select,
  Banner,
  Button,
  BlockStack,
  Text,
  Spinner,
} from '@shopify/polaris';
import { CATEGORY_OPTIONS } from '../constants/categories';
import { api } from '../utils/api';
import { SupplierListing } from '../types';

interface VariantPriceEntry {
  id: string;
  title: string;
  wholesalePrice: string;
}

export default function ListingEdit() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState('other');
  const [processingDays, setProcessingDays] = useState('3');
  const [stockPercent, setStockPercent] = useState('100');
  const [variants, setVariants] = useState<VariantPriceEntry[]>([]);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    api
      .get<SupplierListing>(`/supplier/listings/${id}`)
      .then((listing) => {
        setTitle(listing.title);
        setDescription(listing.description || '');
        setCategory(listing.category || 'other');
        setProcessingDays(String(listing.processing_days));
        setStockPercent(String(listing.marketplace_stock_percent || 100));
        setVariants(
          (listing.variants || []).map((v) => ({
            id: v.id,
            title: v.title || v.sku || 'Variant',
            wholesalePrice: String(v.wholesale_price),
          })),
        );
      })
      .catch((err) => setError(err.message || 'Failed to load listing'))
      .finally(() => setLoading(false));
  }, [id]);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      const variantPrices: Record<string, number> = {};
      for (const v of variants) {
        const price = parseFloat(v.wholesalePrice);
        if (!isNaN(price)) {
          variantPrices[v.id] = price;
        }
      }
      await api.put(`/supplier/listings/${id}`, {
        title,
        description,
        category,
        processing_days: parseInt(processingDays, 10) || 3,
        marketplace_stock_percent: parseInt(stockPercent, 10) || 100,
        variant_prices: variantPrices,
      });
      setSuccess('Listing updated successfully.');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to save changes';
      setError(message);
    } finally {
      setSaving(false);
    }
  }, [id, title, description, category, processingDays, stockPercent, variants]);

  const handleVariantPriceChange = useCallback(
    (variantId: string, value: string) => {
      setVariants((prev) =>
        prev.map((v) => (v.id === variantId ? { ...v, wholesalePrice: value } : v)),
      );
    },
    [],
  );

  if (loading) {
    return (
      <Page title="Edit Listing" backAction={{ content: 'Listings', onAction: () => navigate('/supplier/listings') }}>
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  const categoryOptions = CATEGORY_OPTIONS.map((c) => ({
    label: c.label,
    value: c.value,
  }));

  return (
    <Page
      title="Edit Listing"
      backAction={{ content: 'Listings', onAction: () => navigate('/supplier/listings') }}
      primaryAction={{ content: 'Save', onAction: handleSave, loading: saving }}
    >
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical" onDismiss={() => setError('')}>
              {error}
            </Banner>
          </Layout.Section>
        )}
        {success && (
          <Layout.Section>
            <Banner tone="success" onDismiss={() => setSuccess('')}>
              {success}
            </Banner>
          </Layout.Section>
        )}

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">
                Product Details
              </Text>
              <FormLayout>
                <TextField
                  label="Title"
                  value={title}
                  onChange={setTitle}
                  autoComplete="off"
                />
                <TextField
                  label="Description"
                  value={description}
                  onChange={setDescription}
                  multiline={4}
                  autoComplete="off"
                />
                <Select
                  label="Category"
                  options={categoryOptions}
                  value={category}
                  onChange={setCategory}
                />
                <TextField
                  label="Processing Days"
                  type="number"
                  value={processingDays}
                  onChange={setProcessingDays}
                  autoComplete="off"
                  min={1}
                  max={30}
                />
                <TextField
                  label="Marketplace Stock Allocation"
                  type="number"
                  value={stockPercent}
                  onChange={setStockPercent}
                  suffix="%"
                  autoComplete="off"
                  min={1}
                  max={100}
                  helpText="Percentage of your inventory available for resellers. E.g., 50% means if you have 10 items, only 5 are available on the marketplace."
                />
              </FormLayout>
            </BlockStack>
          </Card>
        </Layout.Section>

        {variants.length > 0 && (
          <Layout.Section>
            <Card>
              <BlockStack gap="400">
                <Text as="h2" variant="headingMd">
                  Variant Wholesale Prices
                </Text>
                <FormLayout>
                  {variants.map((v) => (
                    <TextField
                      key={v.id}
                      label={v.title}
                      type="number"
                      value={v.wholesalePrice}
                      onChange={(value) => handleVariantPriceChange(v.id, value)}
                      prefix="$"
                      autoComplete="off"
                      min={0}
                      step={0.01}
                    />
                  ))}
                </FormLayout>
              </BlockStack>
            </Card>
          </Layout.Section>
        )}

        <Layout.Section>
          <Button variant="primary" onClick={handleSave} loading={saving}>
            Save Changes
          </Button>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
