import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Spinner,
  Banner, InlineStack, Divider, TextField, Select,
  Thumbnail, Checkbox, ProgressBar, EmptyState,
} from '@shopify/polaris';
import { ImageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { SupplierListing } from '../types';
import { PRODUCT_CATEGORIES, getCategoryLabel } from '../constants/categories';

interface MarketplaceResponse {
  listings: SupplierListing[];
  total: number;
}

export default function BulkImport() {
  const navigate = useNavigate();
  const [markupType, setMarkupType] = useState('percentage');
  const [markupValue, setMarkupValue] = useState('30');
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [importing, setImporting] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [categoryFilter, setCategoryFilter] = useState('');

  const categoryParam = categoryFilter ? `&category=${categoryFilter}` : '';
  const { data, loading } = useApi<MarketplaceResponse>(
    `/reseller/marketplace?limit=100&offset=0${categoryParam}`,
  );

  const listings = data?.listings || [];

  const getImage = (listing: SupplierListing): string | null => {
    try {
      const imgs = typeof listing.images === 'string' ? JSON.parse(listing.images || '[]') : (listing.images || []);
      return imgs[0]?.url || imgs[0]?.URL || null;
    } catch { return null; }
  };

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const selectAll = () => {
    if (selectedIds.size === listings.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(listings.map(l => l.id)));
    }
  };

  // Smart pricing: suggest retail price based on wholesale + market positioning
  const suggestRetailPrice = (wholesalePrice: number): number => {
    // Tiered markup strategy:
    // Low-cost items (<$20): 80-100% markup for higher margins
    // Mid-range ($20-100): 40-60% markup
    // Premium (>$100): 25-35% markup
    if (wholesalePrice < 20) return Math.ceil(wholesalePrice * 1.9 * 100) / 100;
    if (wholesalePrice < 100) return Math.ceil(wholesalePrice * 1.5 * 100) / 100;
    return Math.ceil(wholesalePrice * 1.3 * 100) / 100;
  };

  const handleBulkImport = useCallback(async () => {
    const selected = listings.filter(l => selectedIds.has(l.id));
    if (selected.length === 0) return;

    setImporting(true);
    setError(null);
    setSuccess(null);
    setProgress(0);

    let completed = 0;
    let failed = 0;

    for (const listing of selected) {
      try {
        await api.post('/reseller/imports', {
          supplier_listing_id: listing.id,
          markup_type: markupType,
          markup_value: parseFloat(markupValue),
          sync_images: true,
          sync_description: true,
          sync_title: false,
        });
        completed++;
      } catch {
        failed++;
      }
      setProgress(Math.round(((completed + failed) / selected.length) * 100));
    }

    setImporting(false);
    if (failed === 0) {
      setSuccess(`Successfully imported ${completed} products!`);
    } else {
      setSuccess(`Imported ${completed} products. ${failed} failed.`);
    }
    setSelectedIds(new Set());
  }, [listings, selectedIds, markupType, markupValue]);

  if (loading) {
    return <Page title="Bulk Import"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  return (
    <Page
      title="Bulk Import"
      subtitle={`${listings.length} products available | ${selectedIds.size} selected`}
      backAction={{ content: 'Marketplace', onAction: () => navigate('/marketplace') }}
      primaryAction={{
        content: `Import ${selectedIds.size} Products`,
        onAction: handleBulkImport,
        disabled: selectedIds.size === 0 || importing,
        loading: importing,
      }}
    >
      <Layout>
        {error && <Layout.Section><Banner tone="critical" onDismiss={() => setError(null)}>{error}</Banner></Layout.Section>}
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(null)}>{success}</Banner></Layout.Section>}

        {importing && (
          <Layout.Section>
            <Card>
              <BlockStack gap="200">
                <Text as="p" variant="bodySm">Importing products... {progress}%</Text>
                <ProgressBar progress={progress} />
              </BlockStack>
            </Card>
          </Layout.Section>
        )}

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Batch Settings</Text>
              <Divider />
              <InlineStack gap="400">
                <div style={{ width: '200px' }}>
                  <Select label="Markup Type" options={[
                    { label: 'Percentage', value: 'percentage' },
                    { label: 'Fixed Amount', value: 'fixed' },
                  ]} value={markupType} onChange={setMarkupType} />
                </div>
                <div style={{ width: '150px' }}>
                  <TextField label="Markup" type="number" value={markupValue} onChange={setMarkupValue}
                    suffix={markupType === 'percentage' ? '%' : '$'} autoComplete="off" />
                </div>
                <div style={{ width: '200px' }}>
                  <Select label="Filter Category" options={[
                    { label: 'All Categories', value: '' },
                    ...PRODUCT_CATEGORIES.filter(c => c.value !== 'all').map(c => ({ label: c.label, value: c.value })),
                  ]} value={categoryFilter} onChange={setCategoryFilter} />
                </div>
              </InlineStack>
              <InlineStack gap="200" blockAlign="center">
                <Checkbox label={`Select all (${listings.length})`} checked={selectedIds.size === listings.length && listings.length > 0} onChange={selectAll} />
              </InlineStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          {listings.length > 0 ? (
            <Card>
              <BlockStack gap="0">
                {listings.map((listing, i) => {
                  const imgUrl = getImage(listing);
                  const wholesale = listing.variants?.[0]?.wholesale_price || 0;
                  const smartPrice = suggestRetailPrice(wholesale);
                  const markup = parseFloat(markupValue) || 0;
                  const yourPrice = markupType === 'percentage'
                    ? wholesale * (1 + markup / 100)
                    : wholesale + markup;

                  return (
                    <div key={listing.id}>
                      <div style={{ padding: '12px 16px', background: selectedIds.has(listing.id) ? '#f0f7ff' : 'transparent' }}>
                        <InlineStack gap="400" blockAlign="center" align="space-between">
                          <InlineStack gap="300" blockAlign="center">
                            <Checkbox label="" checked={selectedIds.has(listing.id)} onChange={() => toggleSelect(listing.id)} />
                            <Thumbnail source={imgUrl || ImageIcon} alt={listing.title} size="small" />
                            <BlockStack gap="050">
                              <Text as="span" variant="bodyMd" fontWeight="semibold">{listing.title}</Text>
                              <InlineStack gap="200">
                                <Badge tone="info">{getCategoryLabel(listing.category)}</Badge>
                                <Text as="span" variant="bodySm" tone="subdued">{listing.vendor}</Text>
                              </InlineStack>
                            </BlockStack>
                          </InlineStack>
                          <InlineStack gap="400" blockAlign="center">
                            <BlockStack gap="050">
                              <Text as="span" variant="bodySm" tone="subdued">Wholesale</Text>
                              <Text as="span" variant="bodyMd">${wholesale.toFixed(2)}</Text>
                            </BlockStack>
                            <BlockStack gap="050">
                              <Text as="span" variant="bodySm" tone="subdued">Your Price</Text>
                              <Text as="span" variant="bodyMd" fontWeight="semibold">${yourPrice.toFixed(2)}</Text>
                            </BlockStack>
                            <BlockStack gap="050">
                              <Text as="span" variant="bodySm" tone="subdued">AI Suggested</Text>
                              <Text as="span" variant="bodyMd" tone="success">${smartPrice.toFixed(2)}</Text>
                            </BlockStack>
                          </InlineStack>
                        </InlineStack>
                      </div>
                      {i < listings.length - 1 && <Divider />}
                    </div>
                  );
                })}
              </BlockStack>
            </Card>
          ) : (
            <Card>
              <EmptyState heading="No products available" image="">
                <p>No products in the marketplace to import.</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>
    </Page>
  );
}
