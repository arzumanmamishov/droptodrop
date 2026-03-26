import { useState, useCallback } from 'react';
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
  Thumbnail,
} from '@shopify/polaris';
import { ImageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { SupplierListing } from '../types';

interface MarketplaceResponse {
  listings: SupplierListing[];
  total: number;
}

export default function Marketplace() {
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [importModal, setImportModal] = useState<SupplierListing | null>(null);
  const [markupType, setMarkupType] = useState('percentage');
  const [markupValue, setMarkupValue] = useState('30');
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [importSuccess, setImportSuccess] = useState(false);

  const limit = 20;
  const searchParam = search ? `&search=${encodeURIComponent(search)}` : '';
  const { data, loading, error } = useApi<MarketplaceResponse>(
    `/reseller/marketplace?limit=${limit}&offset=${page * limit}${searchParam}`,
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

  if (loading) {
    return (
      <Page title="Marketplace">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  return (
    <Page title="Marketplace">
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}
        {importSuccess && (
          <Layout.Section>
            <Banner tone="success" onDismiss={() => setImportSuccess(false)}>Product imported successfully!</Banner>
          </Layout.Section>
        )}

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <TextField
                label="Search products"
                value={search}
                onChange={setSearch}
                placeholder="Search by title, type..."
                autoComplete="off"
                clearButton
                onClearButtonClick={() => setSearch('')}
              />
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          {(data?.listings || []).length > 0 ? (
            <InlineGrid columns={3} gap="400">
              {data!.listings.map((listing) => (
                <Card key={listing.id}>
                  <BlockStack gap="300">
                    {(() => {
                      const images = typeof listing.images === 'string' ? JSON.parse(listing.images || '[]') : (listing.images || []);
                      const imgUrl = images[0]?.url || images[0]?.URL;
                      return imgUrl ? (
                        <div style={{ textAlign: 'center' }}>
                          <Thumbnail source={imgUrl} alt={listing.title} size="large" />
                        </div>
                      ) : (
                        <div style={{ textAlign: 'center' }}>
                          <Thumbnail source={ImageIcon} alt={listing.title} size="large" />
                        </div>
                      );
                    })()}
                    <Text as="h3" variant="headingSm">{listing.title}</Text>
                    <InlineStack gap="200">
                      <Badge>{listing.product_type || 'General'}</Badge>
                      <Badge tone="info">{`${listing.processing_days}d processing`}</Badge>
                    </InlineStack>
                    <Text as="p" variant="bodySm" tone="subdued">
                      {listing.vendor || 'Unknown vendor'}
                    </Text>
                    {listing.variants && listing.variants.length > 0 && (
                      <BlockStack gap="100">
                        <Text as="p" variant="bodySm">
                          From ${listing.variants[0].wholesale_price.toFixed(2)} wholesale
                        </Text>
                        {listing.variants[0].suggested_retail_price > 0 && (
                          <Text as="p" variant="bodySm" tone="subdued">
                            Suggested retail: ${listing.variants[0].suggested_retail_price.toFixed(2)}
                          </Text>
                        )}
                        <Text as="p" variant="bodySm">
                          {listing.variants.reduce((sum, v) => sum + v.inventory_quantity, 0)} in stock
                        </Text>
                      </BlockStack>
                    )}
                    <InlineStack align="end">
                      <Button variant="primary" onClick={() => setImportModal(listing)}>
                        Import
                      </Button>
                    </InlineStack>
                  </BlockStack>
                </Card>
              ))}
            </InlineGrid>
          ) : (
            <Card>
              <Text as="p" tone="subdued">
                No products available in the marketplace yet. Check back later.
              </Text>
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
                    <Text as="h3" variant="headingSm">Price Preview</Text>
                    {importModal.variants.slice(0, 5).map((v) => {
                      const markup = parseFloat(markupValue) || 0;
                      const resellerPrice =
                        markupType === 'percentage'
                          ? v.wholesale_price * (1 + markup / 100)
                          : v.wholesale_price + markup;
                      const margin = ((resellerPrice - v.wholesale_price) / resellerPrice) * 100;
                      return (
                        <InlineStack key={v.id} gap="200" align="space-between">
                          <Text as="span" variant="bodySm">{v.title || 'Default'}</Text>
                          <Text as="span" variant="bodySm">
                            ${v.wholesale_price.toFixed(2)} → ${resellerPrice.toFixed(2)} ({margin.toFixed(1)}% margin)
                          </Text>
                        </InlineStack>
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
