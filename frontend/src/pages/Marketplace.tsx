import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, TextField, Button, Spinner, Banner,
  BlockStack, Text, InlineStack, InlineGrid, Badge, Modal,
  FormLayout, Select, Tabs, Thumbnail, Divider, EmptyState, Box, Icon,
} from '@shopify/polaris';
import { ImageIcon, StarIcon, DeliveryIcon } from '@shopify/polaris-icons';
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
        sync_images: true, sync_description: true, sync_title: false,
      });
      setImportSuccess(true);
      setImportModal(null);
    } catch (err) {
      setImportError(err instanceof Error ? err.message : 'Import failed');
    } finally { setImporting(false); }
  }, [importModal, markupType, markupValue]);

  const startConversation = useCallback(async (supplierShopId: string) => {
    try {
      await api.post('/conversations', { other_shop_id: supplierShopId, subject: 'Product inquiry' });
      navigate('/messages');
    } catch { /* */ }
  }, [navigate]);

  const requestSample = useCallback(async (listingId: string) => {
    try {
      await api.post('/samples', { listing_id: listingId, quantity: 1, notes: 'Sample request' });
      setImportSuccess(true);
    } catch { /* */ }
  }, []);

  const categoryTabs = PRODUCT_CATEGORIES.map((cat) => ({ id: cat.value, content: cat.label }));

  const getProductImage = (listing: SupplierListing): string | null => {
    try {
      const images = typeof listing.images === 'string' ? JSON.parse(listing.images || '[]') : (listing.images || []);
      return images[0]?.url || images[0]?.URL || null;
    } catch { return null; }
  };

  const getSmartPrice = (wholesale: number) => {
    if (wholesale < 20) return Math.ceil(wholesale * 1.9 * 100) / 100;
    if (wholesale < 100) return Math.ceil(wholesale * 1.5 * 100) / 100;
    return Math.ceil(wholesale * 1.3 * 100) / 100;
  };

  return (
    <Page title="Marketplace" subtitle={`${data?.total || 0} products available`}
      secondaryActions={[{ content: 'Bulk Import', onAction: () => navigate('/bulk-import') }]}>
      <Layout>
        {error && <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>}
        {importSuccess && (
          <Layout.Section>
            <Banner tone="success" onDismiss={() => setImportSuccess(false)}>Done! Check your Imports page.</Banner>
          </Layout.Section>
        )}

        {/* Hero search bar */}
        <Layout.Section>
          <div style={{ background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', borderRadius: '16px', padding: '32px 24px' }}>
            <BlockStack gap="300">
              <Text as="p" variant="headingLg" alignment="center">
                <span style={{ color: 'white' }}>Find products to sell in your store</span>
              </Text>
              <div style={{ maxWidth: '600px', margin: '0 auto', width: '100%' }}>
                <TextField
                  label="" labelHidden value={search} onChange={setSearch}
                  placeholder="Search by product name, type, vendor..."
                  autoComplete="off" clearButton onClearButtonClick={() => setSearch('')}
                />
              </div>
              <Text as="p" variant="bodySm" alignment="center">
                <span style={{ color: 'rgba(255,255,255,0.7)' }}>{data?.total || 0} products from verified suppliers</span>
              </Text>
            </BlockStack>
          </div>
        </Layout.Section>

        {/* Category tabs */}
        <Layout.Section>
          <Card padding="0">
            <Tabs tabs={categoryTabs} selected={selectedCategory} onSelect={(i) => { setSelectedCategory(i); setPage(0); }} fitted />
          </Card>
        </Layout.Section>

        {/* Product grid */}
        <Layout.Section>
          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div>
          ) : (data?.listings || []).length > 0 ? (
            <InlineGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="400">
              {data!.listings.map((listing) => {
                const imgUrl = getProductImage(listing);
                const wholesale = listing.variants?.[0]?.wholesale_price || 0;
                const smartPrice = getSmartPrice(wholesale);

                return (
                  <div key={listing.id} className="product-card" style={{
                    background: 'white', borderRadius: '16px',
                    boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
                    overflow: 'hidden',
                  }}>
                    {/* Image */}
                    <div style={{ position: 'relative', overflow: 'hidden' }}>
                      {imgUrl ? (
                        <img src={imgUrl} alt={listing.title} style={{
                          width: '100%', height: '220px', objectFit: 'cover', display: 'block',
                          transition: 'transform 0.3s ease',
                        }}
                        onMouseOver={(e) => (e.currentTarget.style.transform = 'scale(1.05)')}
                        onMouseOut={(e) => (e.currentTarget.style.transform = 'scale(1)')}
                        />
                      ) : (
                        <div style={{
                          width: '100%', height: '220px',
                          background: 'linear-gradient(135deg, #f0f4ff 0%, #e8eaf6 100%)',
                          display: 'flex', alignItems: 'center', justifyContent: 'center',
                        }}>
                          <Thumbnail source={ImageIcon} alt={listing.title} size="large" />
                        </div>
                      )}
                      {/* Category badge overlay */}
                      <div style={{ position: 'absolute', top: '12px', left: '12px' }}>
                        <div style={{ background: 'rgba(0,0,0,0.6)', color: 'white', padding: '4px 10px', borderRadius: '20px', fontSize: '12px', fontWeight: 500 }}>
                          {getCategoryLabel(listing.category)}
                        </div>
                      </div>
                      {/* Shipping badge */}
                      <div style={{ position: 'absolute', top: '12px', right: '12px' }}>
                        <div style={{ background: 'rgba(255,255,255,0.9)', padding: '4px 8px', borderRadius: '20px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}>
                          <Icon source={DeliveryIcon} tone="subdued" />
                          <span>{listing.processing_days}d</span>
                        </div>
                      </div>
                    </div>

                    {/* Content */}
                    <div style={{ padding: '16px' }}>
                      <BlockStack gap="300">
                        {/* Title & vendor */}
                        <BlockStack gap="100">
                          <Text as="h3" variant="headingSm" fontWeight="bold">{listing.title}</Text>
                          <Text as="p" variant="bodySm" tone="subdued">{listing.vendor || 'Unknown vendor'}</Text>
                        </BlockStack>

                        {/* Pricing */}
                        {wholesale > 0 && (
                          <div style={{ background: '#f8fafb', borderRadius: '10px', padding: '12px' }}>
                            <InlineStack align="space-between" blockAlign="center">
                              <BlockStack gap="050">
                                <Text as="p" variant="bodySm" tone="subdued">Wholesale</Text>
                                <span className="price-tag">${wholesale.toFixed(2)}</span>
                              </BlockStack>
                              <BlockStack gap="050">
                                <Text as="p" variant="bodySm" tone="subdued">Sell for</Text>
                                <Text as="p" variant="headingSm" tone="success">${smartPrice.toFixed(2)}</Text>
                              </BlockStack>
                              <BlockStack gap="050">
                                <Text as="p" variant="bodySm" tone="subdued">Profit</Text>
                                <Text as="p" variant="headingSm" fontWeight="bold">
                                  ${(smartPrice - wholesale).toFixed(2)}
                                </Text>
                              </BlockStack>
                            </InlineStack>
                          </div>
                        )}

                        {/* Actions */}
                        <Button variant="primary" fullWidth onClick={() => setImportModal(listing)}>
                          Import to My Store
                        </Button>
                        <InlineStack gap="100">
                          <Button fullWidth size="slim" onClick={() => navigate(`/supplier/${listing.supplier_shop_id}`)}>
                            View Supplier
                          </Button>
                          <Button fullWidth size="slim" onClick={() => startConversation(listing.supplier_shop_id)}>
                            Message
                          </Button>
                          <Button fullWidth size="slim" onClick={() => requestSample(listing.id)}>
                            Sample
                          </Button>
                        </InlineStack>
                      </BlockStack>
                    </div>
                  </div>
                );
              })}
            </InlineGrid>
          ) : (
            <Card>
              <EmptyState heading="No products found" image="">
                <p>{search || categoryValue !== 'all' ? 'Try adjusting your filters.' : 'No products available yet.'}</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>

        {/* Pagination */}
        <Layout.Section>
          <InlineStack align="center" gap="400">
            <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</Button>
            <div style={{ background: '#f6f6f7', padding: '6px 16px', borderRadius: '20px' }}>
              <Text as="span" variant="bodySm" fontWeight="semibold">Page {page + 1}</Text>
            </div>
            <Button disabled={(data?.listings || []).length < limit} onClick={() => setPage((p) => p + 1)}>Next</Button>
          </InlineStack>
        </Layout.Section>
      </Layout>

      {/* Import Modal */}
      {importModal && (
        <Modal open onClose={() => setImportModal(null)} title={`Import: ${importModal.title}`}
          primaryAction={{ content: 'Import Product', onAction: handleImport, loading: importing }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setImportModal(null) }]}>
          <Modal.Section>
            <BlockStack gap="400">
              {importError && <Banner tone="critical">{importError}</Banner>}
              <InlineStack gap="400" blockAlign="center">
                {(() => {
                  const img = getProductImage(importModal);
                  return <Thumbnail source={img || ImageIcon} alt={importModal.title} size="large" />;
                })()}
                <BlockStack gap="100">
                  <Text as="h3" variant="headingMd">{importModal.title}</Text>
                  <InlineStack gap="200">
                    <Badge tone="info">{getCategoryLabel(importModal.category)}</Badge>
                    <Badge>{importModal.vendor}</Badge>
                  </InlineStack>
                </BlockStack>
              </InlineStack>
              <Divider />
              <FormLayout>
                <Select label="Markup type" options={[
                  { label: 'Percentage', value: 'percentage' },
                  { label: 'Fixed amount', value: 'fixed' },
                ]} value={markupType} onChange={setMarkupType} />
                <TextField
                  label={markupType === 'percentage' ? 'Markup percentage' : 'Fixed markup amount'}
                  type="number" value={markupValue} onChange={setMarkupValue}
                  suffix={markupType === 'percentage' ? '%' : '$'} autoComplete="off"
                />
                {importModal.variants && importModal.variants.length > 0 && (
                  <BlockStack gap="200">
                    <Text as="h3" variant="headingSm">Price Preview</Text>
                    {importModal.variants.slice(0, 5).map((v) => {
                      const markup = parseFloat(markupValue) || 0;
                      const price = markupType === 'percentage' ? v.wholesale_price * (1 + markup / 100) : v.wholesale_price + markup;
                      const margin = ((price - v.wholesale_price) / price) * 100;
                      const ai = getSmartPrice(v.wholesale_price);
                      const aiMargin = ((ai - v.wholesale_price) / ai) * 100;
                      return (
                        <div key={v.id} style={{ background: '#f8fafb', borderRadius: '8px', padding: '12px' }}>
                          <InlineStack align="space-between" blockAlign="center">
                            <Text as="span" variant="bodySm" fontWeight="semibold">{v.title || 'Default'}</Text>
                            <InlineStack gap="200">
                              <Text as="span" variant="bodySm">${v.wholesale_price.toFixed(2)} → <strong>${price.toFixed(2)}</strong> ({margin.toFixed(0)}%)</Text>
                            </InlineStack>
                          </InlineStack>
                          <InlineStack align="end" gap="200">
                            <Badge tone="success">{`AI: $${ai.toFixed(2)} (${aiMargin.toFixed(0)}% margin)`}</Badge>
                          </InlineStack>
                        </div>
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
