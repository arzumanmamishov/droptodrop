import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, DataTable, Badge, Button, Spinner, Banner,
  BlockStack, Modal, FormLayout, TextField, Select,
  EmptyState,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';

interface Dispute {
  id: string;
  routed_order_id: string;
  reporter_role: string;
  dispute_type: string;
  status: string;
  description: string;
  resolution: string;
  created_at: string;
}

interface DisputesResponse {
  disputes: Dispute[];
  total: number;
}

const DISPUTE_TYPES = [
  { label: 'Quality Issue', value: 'quality_issue' },
  { label: 'Wrong Item', value: 'wrong_item' },
  { label: 'Not Received', value: 'not_received' },
  { label: 'Damaged', value: 'damaged' },
  { label: 'Late Delivery', value: 'late_delivery' },
  { label: 'Missing Items', value: 'missing_items' },
  { label: 'Other', value: 'other' },
];

export default function Disputes() {
  const navigate = useNavigate();
  const toast = useToast();
  const [createModal, setCreateModal] = useState(false);
  const [orderId, setOrderId] = useState('');
  const [disputeType, setDisputeType] = useState('quality_issue');
  const [description, setDescription] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const limit = 20;
  const { data, loading, refetch } = useApi<DisputesResponse>(
    `/disputes?limit=${limit}&offset=0`,
  );

  const handleCreate = useCallback(async () => {
    setCreating(true);
    setError(null);
    try {
      await api.post('/disputes', {
        routed_order_id: orderId,
        dispute_type: disputeType,
        description,
      });
      toast.success('Dispute created');
      setSuccess(true);
      setCreateModal(false);
      setOrderId('');
      setDescription('');
      refetch();
    } catch (err) {
      toast.error('Failed to create dispute');
      setError(err instanceof Error ? err.message : 'Failed to create dispute');
    } finally {
      setCreating(false);
    }
  }, [orderId, disputeType, description, refetch]);

  const statusBadge = (status: string) => {
    const map: Record<string, 'attention' | 'info' | 'success' | 'critical'> = {
      open: 'attention', in_review: 'info', resolved: 'success', closed: 'critical',
    };
    return <Badge tone={map[status]}>{status.replace('_', ' ')}</Badge>;
  };

  const typeBadge = (type: string) => (
    <Badge>{DISPUTE_TYPES.find(t => t.value === type)?.label || type}</Badge>
  );

  if (loading) {
    return <Page title="Disputes"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const disputes = data?.disputes || [];
  const rows = disputes.map((d) => [
    <Button key={d.id} variant="plain" onClick={() => navigate(`/orders/${d.routed_order_id}`)}>{d.routed_order_id.slice(0, 8)}</Button>,
    typeBadge(d.dispute_type),
    statusBadge(d.status),
    d.description.slice(0, 60) + (d.description.length > 60 ? '...' : ''),
    d.resolution || '-',
    new Date(d.created_at).toLocaleDateString(),
  ]);

  return (
    <Page title="Disputes" primaryAction={{ content: 'Report Issue', onAction: () => setCreateModal(true) }}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(false)}>Dispute created.</Banner></Layout.Section>}
        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              {rows.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'text', 'text', 'text', 'text']}
                  headings={['Order', 'Type', 'Status', 'Description', 'Resolution', 'Date']}
                  rows={rows}
                />
              ) : (
                <EmptyState heading="No disputes" image="">
                  <p>No disputes have been filed. Use "Report Issue" if you have a problem with an order.</p>
                </EmptyState>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>

      {createModal && (
        <Modal open onClose={() => setCreateModal(false)} title="Report an Issue"
          primaryAction={{ content: 'Submit', onAction: handleCreate, loading: creating, disabled: !orderId || !description }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setCreateModal(false) }]}>
          <Modal.Section>
            <BlockStack gap="400">
              {error && <Banner tone="critical">{error}</Banner>}
              <FormLayout>
                <TextField label="Order ID or Order Number" value={orderId} onChange={setOrderId} autoComplete="off" helpText="Enter the order number (e.g. 1005) or order ID from the Orders page" />
                <Select label="Issue Type" options={DISPUTE_TYPES} value={disputeType} onChange={setDisputeType} />
                <TextField label="Description" value={description} onChange={setDescription} multiline={4} autoComplete="off" helpText="Describe the issue in detail" />
              </FormLayout>
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
