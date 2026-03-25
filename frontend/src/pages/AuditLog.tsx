import { useState } from 'react';
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
import { AuditEntry } from '../types';

interface AuditResponse {
  entries: AuditEntry[];
  total: number;
}

export default function AuditLog() {
  const [page, setPage] = useState(0);
  const limit = 50;

  const { data, loading, error } = useApi<AuditResponse>(
    `/audit?limit=${limit}&offset=${page * limit}`,
  );

  if (loading) {
    return (
      <Page title="Audit Log">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  const outcomeBadge = (outcome: string) => {
    const toneMap: Record<string, 'success' | 'critical' | 'attention'> = {
      success: 'success',
      failure: 'critical',
      error: 'critical',
    };
    return <Badge tone={toneMap[outcome]}>{outcome}</Badge>;
  };

  const rows = (data?.entries || []).map((entry) => [
    entry.actor_type,
    entry.action,
    entry.resource_type || '-',
    entry.resource_id ? entry.resource_id.slice(0, 8) : '-',
    outcomeBadge(entry.outcome),
    entry.error_payload ? entry.error_payload.slice(0, 50) : '-',
    new Date(entry.created_at).toLocaleString(),
  ]);

  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page title="Audit Log">
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
                  columnContentTypes={['text', 'text', 'text', 'text', 'text', 'text', 'text']}
                  headings={['Actor', 'Action', 'Resource', 'ID', 'Outcome', 'Error', 'When']}
                  rows={rows}
                />
              ) : (
                <Text as="p" tone="subdued">No audit entries yet.</Text>
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
