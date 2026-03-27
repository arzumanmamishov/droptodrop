import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page,
  Layout,
  Card,
  DataTable,
  Badge,
  Button,
  Spinner,
  Banner,
  BlockStack,
  Text,
  InlineStack,
  InlineGrid,
  Icon,
  EmptyState,
} from '@shopify/polaris';
import { CheckIcon, ClockIcon, AlertCircleIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { ResellerImport } from '../types';

interface ImportsResponse {
  imports: ResellerImport[];
  total: number;
}

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

  const handleDelete = useCallback(async (importId: string) => {
    try {
      await api.delete(`/reseller/imports/${importId}`);
      refetch();
    } catch { /* */ }
  }, [refetch]);

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

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      active: 'success', pending: 'attention', failed: 'critical', paused: 'info', removed: 'critical',
    };
    return <Badge tone={toneMap[status]}>{status}</Badge>;
  };

  const rows = imports.map((imp) => [
    imp.supplier_title || imp.supplier_listing_id.slice(0, 8),
    statusBadge(imp.status),
    `${imp.markup_type === 'percentage' ? imp.markup_value + '%' : '$' + imp.markup_value.toFixed(2)}`,
    imp.last_sync_at ? new Date(imp.last_sync_at).toLocaleDateString() : 'Never',
    imp.last_sync_error
      ? <Badge key={`err-${imp.id}`} tone="critical">Error</Badge>
      : <Badge key={`ok-${imp.id}`} tone="success">OK</Badge>,
    <InlineStack key={imp.id} gap="200">
      <Button size="slim" loading={syncing === imp.id} onClick={() => handleResync(imp.id)}>
        Re-sync
      </Button>
      <Button size="slim" tone="critical" onClick={() => handleDelete(imp.id)}>
        Delete
      </Button>
    </InlineStack>,
  ]);

  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page
      title="Imported Products"
      subtitle={`${data?.total || 0} products`}
      primaryAction={{ content: 'Browse Marketplace', onAction: () => navigate('/marketplace') }}
    >
      <Layout>
        {error && (
          <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>
        )}

        <Layout.Section>
          <InlineGrid columns={{ xs: 1, md: 3 }} gap="400">
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#e3f1df', borderRadius: '8px', padding: '8px', display: 'flex' }}><Icon source={CheckIcon} /></div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{activeCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Active</Text>
                </BlockStack>
              </InlineStack>
            </Card>
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#fef3cd', borderRadius: '8px', padding: '8px', display: 'flex' }}><Icon source={ClockIcon} /></div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{pendingCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Pending</Text>
                </BlockStack>
              </InlineStack>
            </Card>
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#fde8e8', borderRadius: '8px', padding: '8px', display: 'flex' }}><Icon source={AlertCircleIcon} /></div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{failedCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Failed</Text>
                </BlockStack>
              </InlineStack>
            </Card>
          </InlineGrid>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              {rows.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'text', 'text', 'text', 'text']}
                  headings={['Source Product', 'Status', 'Markup', 'Last Sync', 'Health', 'Actions']}
                  rows={rows}
                />
              ) : (
                <EmptyState
                  heading="No imported products"
                  action={{ content: 'Browse Marketplace', onAction: () => navigate('/marketplace') }}
                  image=""
                >
                  <p>Import products from suppliers to start selling them in your store.</p>
                </EmptyState>
              )}

              {totalPages > 1 && (
                <InlineStack align="center" gap="200">
                  <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</Button>
                  <Text as="span" variant="bodySm">Page {page + 1} of {totalPages}</Text>
                  <Button disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>Next</Button>
                </InlineStack>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
