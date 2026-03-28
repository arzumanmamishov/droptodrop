import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page,
  Layout,
  Card,
  TextField,
  Button,
  Spinner,
  Banner,
  BlockStack,
  Text,
  InlineStack,
  InlineGrid,
  Badge,
  Modal,
  FormLayout,
  Select,
  Tabs,
  Thumbnail,
  Divider,
  EmptyState,
  Box,
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

export default function Marketplace() {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [selectedCategory, setSelectedCategory] = useState(0);
  const [page, setPage] = useState(0);
  const [importModal, setImportModal] = useState<SupplierListing | null>(null);
  const [markupType, setMarkupType] = useState('percentage');
  const [markupValue, setMarkupValue] = useState('30');
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [importSuccess, setImportSuccess] = useState(false);

  const limit = 20;
  const categoryValue = PRODUCT_CATEGORIES[selectedCategory]?.value || 'all';
  const categoryParam = categoryValue !== 'all' ? `&category=${categoryValue}` : '';
  const searchParam = search ? `&search=${encodeURIComponent(search)}` : '';
  const { data, loading, error } = useApi<MarketplaceResponse>(
    `/reseller/marketplace?limit=${limit}&offset=${page * limit}${categoryParam}${searchParam}`,
  );

  const handleImport = useCallback(async () => {
    if (!importModal) return;
    setImporting(true);
    setImportError(null);
    try {
      await api.post('/reseller/imports', {
        supplier_listing_id: importModal.id,
        markup_type: markupType,
        markup_value: parseFloat(markupValue),
        sync_images: true,
        sync_description: true,
        sync_title: false,
      });
      setImportSuccess(true);
      setImportModal(null);
    } catch (err) {
      setImportError(err instanceof Error ? err.message : 'Import failed');
    } finally {
      setImporting(false);
    }
  }, [importModal, markupType, markupValue]);

  const startConversation = useCallback(async (supplierShopId: string) => {
    try {
      await api.post('/conversations', { other_shop_id: supplierShopId, subject: 'Product inquiry' });
      navigate('/messages');
    } catch { /* */ }
  }, [navigate]);

  const requestSample = useCallback(async (listingId: string) => {
    try {
      await api.post('/samples', { listing_id: listingId, quantity: 1, notes: 'Sample request from marketplace' });
      setImportSuccess(true);
    } catch { /* */ }
  }, []);

  const categoryTabs = PRODUCT_CATEGORIES.map((cat) => ({
    id: cat.value,
    content: cat.label,
  }));

  const getProductImage = (listing: SupplierListing): string | null => {
    try {
      const images = typeof listing.images === 'string' ? JSON.parse(listing.images || '[]') : (listing.images || []);
      return images[0]?.url || images[0]?.URL || null;
    } catch {
      return null;
    }
  };

  return (
    <Page title="Marketplace" subtitle={`${data?.total || 0} products available`}
      secondaryActions={[{ content: 'Bulk Import', onAction: () => navigate('/bulk-import') }]}
    >
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}
        {importSuccess && (
          <Layout.Section>
            <Banner tone="success" onDismiss={() => setImportSuccess(false)}>
              Product imported successfully! Check your Imports page.
            </Banner>
          </Layout.Section>
        )}

        <Layout.Section>
          <Card padding="0">
            <Tabs tabs={categoryTabs} selected={selectedCategory} onSelect={(i) => { setSelectedCategory(i); setPage(0); }} />
          </Card>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <InlineStack gap="400" align="space-between" blockAlign="center">
              <div style={{ flex: 1 }}>
                <TextField
                  label=""
                  labelHidden
                  value={search}
                  onChange={setSearch}
                  placeholder="Search products by name..."
                  autoComplete="off"
                  clearButton
                  onClearButtonClick={() => setSearch('')}
                />
              </div>
              <Text as="span" variant="bodySm" tone="subdued">
                {data?.total || 0} results
              </Text>
            </InlineStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
              <Spinner size="large" />
            </div>
          ) : (data?.listings || []).length > 0 ? (
            <InlineGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="400">
              {data!.listings.map((listing) => {
                const imgUrl = getProductImage(listing);
                return (
                  <Card key={listing.id} padding="0">
                    <div style={{ position: 'relative' }}>
                      {imgUrl ? (
                        <img
                          src={imgUrl}
                          alt={listing.title}
                          style={{
                            width: '100%',
                            height: '200px',
                            objectFit: 'cover',
                            borderTopLeftRadius: '12px',
                            borderTopRightRadius: '12px',
                            display: 'block',
                          }}
                        />
                      ) : (
                        <div style={{
                          width: '100%',
                          height: '200px',
                          background: '#f6f6f7',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          borderTopLeftRadius: '12px',
                          borderTopRightRadius: '12px',
                        }}>
                          <Thumbnail source={ImageIcon} alt={listing.title} size="large" />
                        </div>
                      )}
                      <div style={{ position: 'absolute', top: '8px', right: '8px' }}>
                        <Badge tone="info">{getCategoryLabel(listing.category)}</Badge>
                      </div>
                    </div>
                    <Box padding="400">
                      <BlockStack gap="300">
                        <Text as="h3" variant="headingSm" fontWeight="semibold">{listing.title}</Text>
                        <InlineStack gap="200">
                          <Badge>{listing.vendor || 'Unknown vendor'}</Badge>
                          <Badge tone="attention">{`${listing.processing_days}d shipping`}</Badge>
                        </InlineStack>
                        {listing.variants && listing.variants.length > 0 && (
                          <>
                            <Divider />
                            <InlineStack gap="200" align="space-between">
                              <BlockStack gap="100">
                                <Text as="p" variant="headingSm" tone="success">
                                  ${listing.variants[0].wholesale_price.toFixed(2)}
                                </Text>
                                <Text as="p" variant="bodySm" tone="subdued">wholesale</Text>
                              </BlockStack>
                              {listing.variants[0].suggested_retail_price > 0 && (
                                <BlockStack gap="100">
                                  <Text as="p" variant="headingSm">
                                    ${listing.variants[0].suggested_retail_price.toFixed(2)}
                                  </Text>
                                  <Text as="p" variant="bodySm" tone="subdued">retail</Text>
                                </BlockStack>
                              )}
                            </InlineStack>
                          </>
                        )}
                        <InlineStack gap="200">
                          <Button variant="primary" fullWidth onClick={() => setImportModal(listing)}>
                            Import
                          </Button>
                          <Button fullWidth onClick={() => navigate(`/supplier/${listing.supplier_shop_id}`)}>
                            Supplier
                          </Button>
                        </InlineStack>
                        <InlineStack gap="200">
                          <Button fullWidth variant="plain" onClick={() => startConversation(listing.supplier_shop_id)}>
                            Message
                          </Button>
                          <Button fullWidth variant="plain" onClick={() => requestSample(listing.id)}>
                            Sample
                          </Button>
                        </InlineStack>
                      </BlockStack>
                    </Box>
                  </Card>
                );
              })}
            </InlineGrid>
          ) : (
            <Card>
              <EmptyState
                heading="No products found"
                image=""
              >
                <p>
                  {search || categoryValue !== 'all'
                    ? 'Try adjusting your filters or search terms.'
                    : 'No products available in the marketplace yet. Check back later.'}
                </p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>

        <Layout.Section>
          <InlineStack align="center" gap="200">
            <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</Button>
            <Text as="span" variant="bodySm">Page {page + 1}</Text>
            <Button disabled={(data?.listings || []).length < limit} onClick={() => setPage((p) => p + 1)}>Next</Button>
          </InlineStack>
        </Layout.Section>
      </Layout>

      {importModal && (
        <Modal
          open={true}
          onClose={() => setImportModal(null)}
          title={`Import: ${importModal.title}`}
          primaryAction={{ content: 'Import Product', onAction: handleImport, loading: importing }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setImportModal(null) }]}
        >
          <Modal.Section>
            <BlockStack gap="400">
              {importError && <Banner tone="critical">{importError}</Banner>}
              <InlineStack gap="400" blockAlign="center">
                {(() => {
                  const img = getProductImage(importModal);
                  return img ? (
                    <Thumbnail source={img} alt={importModal.title} size="large" />
                  ) : (
                    <Thumbnail source={ImageIcon} alt={importModal.title} size="large" />
                  );
                })()}
                <BlockStack gap="100">
                  <Text as="h3" variant="headingMd">{importModal.title}</Text>
                  <Badge tone="info">{getCategoryLabel(importModal.category)}</Badge>
                </BlockStack>
              </InlineStack>
              <Divider />
              <FormLayout>
                <Select
                  label="Markup type"
                  options={[
                    { label: 'Percentage', value: 'percentage' },
                    { label: 'Fixed amount', value: 'fixed' },
                  ]}
                  value={markupType}
                  onChange={setMarkupType}
                />
                <TextField
                  label={markupType === 'percentage' ? 'Markup percentage' : 'Fixed markup amount'}
                  type="number"
                  value={markupValue}
                  onChange={setMarkupValue}
                  suffix={markupType === 'percentage' ? '%' : '$'}
                  autoComplete="off"
                />
                {importModal.variants && importModal.variants.length > 0 && (
                  <BlockStack gap="200">
                    <Text as="h3" variant="headingSm">Price Preview & Smart Pricing</Text>
                    {importModal.variants.slice(0, 5).map((v) => {
                      const markup = parseFloat(markupValue) || 0;
                      const resellerPrice =
                        markupType === 'percentage'
                          ? v.wholesale_price * (1 + markup / 100)
                          : v.wholesale_price + markup;
                      const margin = ((resellerPrice - v.wholesale_price) / resellerPrice) * 100;
                      // Smart pricing suggestion
                      const w = v.wholesale_price;
                      const aiPrice = w < 20 ? Math.ceil(w * 1.9 * 100) / 100 : w < 100 ? Math.ceil(w * 1.5 * 100) / 100 : Math.ceil(w * 1.3 * 100) / 100;
                      const aiMargin = ((aiPrice - w) / aiPrice) * 100;
                      return (
                        <BlockStack key={v.id} gap="100">
                          <InlineStack gap="200" align="space-between">
                            <Text as="span" variant="bodySm" fontWeight="semibold">{v.title || 'Default'}</Text>
                            <Text as="span" variant="bodySm">
                              ${w.toFixed(2)} → ${resellerPrice.toFixed(2)} ({margin.toFixed(1)}% margin)
                            </Text>
                          </InlineStack>
                          <InlineStack gap="200" align="end">
                            <Badge tone="success">{`AI suggests: $${aiPrice.toFixed(2)} (${aiMargin.toFixed(0)}% margin)`}</Badge>
                          </InlineStack>
                        </BlockStack>
                      );
                    })}
                  </BlockStack>
                )}
              </FormLayout>
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
