import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import ConfirmDialog from '../components/ConfirmDialog';
import {
  Page, Layout, Card, Badge, Button, Spinner, Banner,
  BlockStack, Text, InlineStack, InlineGrid, Icon,
  EmptyState, Thumbnail,
} from '@shopify/polaris';
import { CheckIcon, ClockIcon, AlertCircleIcon, ImageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { ResellerImport } from '../types';

interface ImportsResponse {
  imports: ResellerImport[];
  total: number;
}

const statusConfig: Record<string, { color: string; bg: string; label: string }> = {
  active:  { color: '#166534', bg: '#dcfce7', label: 'Active' },
  pending: { color: '#92400e', bg: '#fef3c7', label: 'Pending' },
  failed:  { color: '#991b1b', bg: '#fee2e2', label: 'Failed' },
  paused:  { color: '#1e40af', bg: '#dbeafe', label: 'Paused' },
  removed: { color: '#991b1b', bg: '#fee2e2', label: 'Removed' },
};

export default function Imports() {
  const navigate = useNavigate();
  const [page, setPage] = useState(0);
  const [syncing, setSyncing] = useState<string | null>(null);
  const limit = 20;

  const { data, loading, error, refetch } = useApi<ImportsResponse>(
    `/reseller/imports?limit=${limit}&offset=${page * limit}`,
  );

  const handleResync = useCallback(async (importId: string) => {
    setSyncing(importId);
    try {
      await api.post(`/reseller/imports/${importId}/resync`);
      refetch();
    } catch { /* */ }
    finally { setSyncing(null); }
  }, [refetch]);

  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const handleDelete = useCallback(async (importId: string) => {
    try {
      await api.delete(`/reseller/imports/${importId}`);
      refetch();
    } catch { /* */ }
  }, [refetch]);

  const getImportImage = (imp: ResellerImport): string | null => {
    try {
      const imgs = typeof imp.supplier_images === 'string' ? JSON.parse(imp.supplier_images || '[]') : (imp.supplier_images || []);
      return imgs[0]?.url || imgs[0]?.URL || null;
    } catch { return null; }
  };

  if (loading) {
    return (
      <Page title="Imported Products">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  const imports = data?.imports || [];
  const activeCount = imports.filter(i => i.status === 'active').length;
  const pendingCount = imports.filter(i => i.status === 'pending').length;
  const failedCount = imports.filter(i => i.status === 'failed').length;
  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page
      title="Imported Products"
      subtitle={`${data?.total || 0} products`}
      primaryAction={{ content: 'Browse Marketplace', onAction: () => navigate('/marketplace') }}
    >
      <Layout>
        {error && <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>}

        {imports.length > 0 && (
          <Layout.Section>
            <Banner tone="info" title="Publish imported products to your Online Store">
              <p>Go to <strong>Shopify Admin → Products</strong> → click the product → scroll to <strong>"Publishing"</strong> → check <strong>"Online Store"</strong>.</p>
            </Banner>
          </Layout.Section>
        )}

        <Layout.Section>
          <InlineGrid columns={{ xs: 1, md: 3 }} gap="400">
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#dcfce7', borderRadius: '8px', padding: '8px', display: 'flex' }}><Icon source={CheckIcon} /></div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{activeCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Active</Text>
                </BlockStack>
              </InlineStack>
            </Card>
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#fef3c7', borderRadius: '8px', padding: '8px', display: 'flex' }}><Icon source={ClockIcon} /></div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{pendingCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Pending</Text>
                </BlockStack>
              </InlineStack>
            </Card>
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#fee2e2', borderRadius: '8px', padding: '8px', display: 'flex' }}><Icon source={AlertCircleIcon} /></div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{failedCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Failed</Text>
                </BlockStack>
              </InlineStack>
            </Card>
          </InlineGrid>
        </Layout.Section>

        <Layout.Section>
          {imports.length > 0 ? (
            <BlockStack gap="300">
              {imports.map((imp) => {
                const imgUrl = getImportImage(imp);
                const cfg = statusConfig[imp.status] || statusConfig['pending'];

                return (
                  <Card key={imp.id}>
                    <div style={{ padding: '2px 0' }}>
                      <InlineStack align="space-between" blockAlign="start" wrap={false}>
                        {/* Left: image + product info */}
                        <InlineStack gap="400" blockAlign="start" wrap={false}>
                          <div style={{ flexShrink: 0 }}>
                            <Thumbnail source={imgUrl || ImageIcon} alt={imp.supplier_title} size="medium" />
                          </div>
                          <BlockStack gap="100">
                            <Text as="span" variant="bodyMd" fontWeight="semibold">
                              {imp.supplier_title || 'Untitled Product'}
                            </Text>

                            {/* Supplier company name — clickable */}
                            {imp.supplier_shop_id && (
                              <button
                                onClick={() => navigate(`/supplier/${imp.supplier_shop_id}`)}
                                style={{
                                  background: 'none', border: 'none', padding: 0, cursor: 'pointer',
                                  color: '#1e40af', fontSize: '13px', fontWeight: 500, textAlign: 'left',
                                  textDecoration: 'none',
                                }}
                                onMouseOver={(e) => (e.currentTarget.style.textDecoration = 'underline')}
                                onMouseOut={(e) => (e.currentTarget.style.textDecoration = 'none')}
                              >
                                {imp.supplier_company_name || 'View Supplier'}
                              </button>
                            )}

                            <InlineStack gap="200" blockAlign="center">
                              <span style={{
                                padding: '2px 10px', borderRadius: '12px', fontSize: '11px', fontWeight: 600,
                                color: cfg.color, background: cfg.bg,
                              }}>
                                {cfg.label}
                              </span>
                              <Text as="span" variant="bodySm" tone="subdued">
                                Markup: {imp.markup_type === 'percentage' ? `${imp.markup_value}%` : `$${imp.markup_value.toFixed(2)}`}
                              </Text>
                            </InlineStack>

                            <InlineStack gap="300" blockAlign="center">
                              <Text as="span" variant="bodySm" tone="subdued">
                                Synced: {imp.last_sync_at ? new Date(imp.last_sync_at).toLocaleDateString() : 'Never'}
                              </Text>
                              {imp.last_sync_error ? (
                                <Badge tone="critical">Sync Error</Badge>
                              ) : imp.last_sync_at ? (
                                <Badge tone="success">Healthy</Badge>
                              ) : null}
                            </InlineStack>
                          </BlockStack>
                        </InlineStack>

                        {/* Right: actions */}
                        <BlockStack gap="200" align="end">
                          <Button size="slim" loading={syncing === imp.id} onClick={() => handleResync(imp.id)}>
                            Re-sync
                          </Button>
                          <Button size="slim" tone="critical" onClick={() => setConfirmDelete(imp.id)}>
                            Delete
                          </Button>
                          {imp.supplier_shop_id && (
                            <Button size="slim" variant="plain" onClick={() => navigate(`/messages?to=${imp.supplier_shop_id}`)}>
                              Message
                            </Button>
                          )}
                        </BlockStack>
                      </InlineStack>
                    </div>
                  </Card>
                );
              })}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState
                heading="No imported products"
                action={{ content: 'Browse Marketplace', onAction: () => navigate('/marketplace') }}
                image=""
              >
                <p>Import products from suppliers to start selling them in your store.</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>

        {totalPages > 1 && (
          <Layout.Section>
            <InlineStack align="center" gap="200">
              <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</Button>
              <Text as="span" variant="bodySm">Page {page + 1} of {totalPages}</Text>
              <Button disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>Next</Button>
            </InlineStack>
          </Layout.Section>
        )}
      </Layout>

      <ConfirmDialog
        open={confirmDelete !== null}
        title="Delete Import"
        message="Are you sure you want to delete this imported product? The product will be permanently removed from your Shopify store."
        onConfirm={() => { if (confirmDelete) { handleDelete(confirmDelete); setConfirmDelete(null); } }}
        onCancel={() => setConfirmDelete(null)}
      />
    </Page>
  );
}
