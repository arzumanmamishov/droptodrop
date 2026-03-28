import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, Badge, Spinner,
  InlineStack, EmptyState, DataTable, Button,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';

interface SampleOrder {
  id: string; status: string; quantity: number;
  notes: string; tracking_number: string; created_at: string; listing_title: string;
}

interface Props { role: string; }

export default function Samples({ role }: Props) {
  const isSupplier = role === 'supplier';
  const { data, loading, refetch } = useApi<{ samples: SampleOrder[] }>('/samples');
  const [updating, setUpdating] = useState<string | null>(null);

  const handleUpdate = useCallback(async (id: string, status: string) => {
    setUpdating(id);
    try {
      await api.put(`/samples/${id}`, { status, tracking: '' });
      refetch();
    } catch { /* */ }
    finally { setUpdating(null); }
  }, [refetch]);

  if (loading) return <Page title="Samples"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const samples = data?.samples || [];
  const statusBadge = (s: string) => {
    const map: Record<string, 'attention' | 'info' | 'success' | 'critical'> = {
      requested: 'attention', approved: 'info', shipped: 'info', received: 'success', rejected: 'critical',
    };
    return <Badge tone={map[s]}>{s}</Badge>;
  };

  return (
    <Page title="Sample Orders" subtitle={isSupplier ? 'Manage sample requests' : 'Your sample requests'}>
      <Layout>
        <Layout.Section>
          <Card>
            {samples.length > 0 ? (
              <DataTable
                columnContentTypes={['text', 'text', 'numeric', 'text', 'text', 'text']}
                headings={['Product', 'Status', 'Qty', 'Notes', 'Date', isSupplier ? 'Actions' : '']}
                rows={samples.map(s => [
                  s.listing_title || s.id.slice(0, 8),
                  statusBadge(s.status),
                  s.quantity,
                  s.notes || '-',
                  new Date(s.created_at).toLocaleDateString(),
                  isSupplier && s.status === 'requested' ? (
                    <InlineStack key={s.id} gap="200">
                      <Button size="slim" loading={updating === s.id} onClick={() => handleUpdate(s.id, 'approved')}>Approve</Button>
                      <Button size="slim" tone="critical" onClick={() => handleUpdate(s.id, 'rejected')}>Reject</Button>
                    </InlineStack>
                  ) : (s.tracking_number || '-'),
                ])}
              />
            ) : (
              <EmptyState heading="No sample orders" image="">
                <p>{isSupplier ? 'Sample requests from resellers will appear here.' : 'Request samples from the marketplace before importing.'}</p>
              </EmptyState>
            )}
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
