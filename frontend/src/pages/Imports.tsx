import { useState, useCallback } from 'react';
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
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { ResellerImport } from '../types';

interface ImportsResponse {
  imports: ResellerImport[];
  total: number;
}

export default function Imports() {
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
    } catch {
      // Error shown on refetch
    } finally {
      setSyncing(null);
    }
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

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      active: 'success',
      pending: 'attention',
      failed: 'critical',
      paused: 'info',
      removed: 'critical',
    };
    return <Badge tone={toneMap[status]}>{status}</Badge>;
  };

  const rows = (data?.imports || []).map((imp) => [
    imp.supplier_title || imp.supplier_listing_id.slice(0, 8),
    statusBadge(imp.status),
    `${imp.markup_type === 'percentage' ? imp.markup_value + '%' : '$' + imp.markup_value.toFixed(2)}`,
    imp.last_sync_at ? new Date(imp.last_sync_at).toLocaleDateString() : 'Never',
    imp.last_sync_error || '-',
    <Button
      key={imp.id}
      size="slim"
      loading={syncing === imp.id}
      onClick={() => handleResync(imp.id)}
    >
      Re-sync
    </Button>,
  ]);

  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page title="Imported Products">
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}
        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              {rows.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'text', 'text', 'text', 'text']}
                  headings={['Source Product', 'Status', 'Markup', 'Last Sync', 'Sync Error', 'Actions']}
                  rows={rows}
                />
              ) : (
                <Text as="p" tone="subdued">
                  No imported products. Browse the Marketplace to import products from suppliers.
                </Text>
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
