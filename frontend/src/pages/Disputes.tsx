import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, Button, Spinner, Banner,
  BlockStack, Modal, FormLayout, TextField, Select,
  EmptyState, Text, InlineStack,
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

const ORDER_DISPUTE_TYPES = [
  { label: 'Quality Issue', value: 'quality_issue' },
  { label: 'Wrong Item', value: 'wrong_item' },
  { label: 'Not Received', value: 'not_received' },
  { label: 'Damaged', value: 'damaged' },
  { label: 'Late Delivery', value: 'late_delivery' },
  { label: 'Missing Items', value: 'missing_items' },
  { label: 'Other', value: 'other' },
];

const APP_COMPLAINT_TYPES = [
  { label: 'App Bug / Technical Issue', value: 'app_bug' },
  { label: 'Payment Problem', value: 'payment_problem' },
  { label: 'Account Issue', value: 'account_issue' },
  { label: 'Feature Request', value: 'feature_request' },
  { label: 'Policy Violation by Other Party', value: 'policy_violation' },
  { label: 'Other', value: 'app_other' },
];

const statusConfig: Record<string, { color: string; bg: string }> = {
  open: { color: '#92400e', bg: '#fef3c7' },
  in_review: { color: '#1e40af', bg: '#dbeafe' },
  resolved: { color: '#166534', bg: '#dcfce7' },
  closed: { color: '#64748b', bg: '#f1f5f9' },
};

export default function Disputes() {
  const navigate = useNavigate();
  const toast = useToast();
  const [activeTab, setActiveTab] = useState<'orders' | 'app'>('orders');
  const [createModal, setCreateModal] = useState<'order' | 'app' | null>(null);
  const [orderId, setOrderId] = useState('');
  const [disputeType, setDisputeType] = useState('quality_issue');
  const [description, setDescription] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(0);

  const limit = 20;
  const { data, loading, refetch } = useApi<DisputesResponse>(
    `/disputes?limit=${limit}&offset=${page * limit}`,
  );
  const totalPages = Math.ceil((data?.total || 0) / limit);

  const handleCreate = useCallback(async () => {
    setCreating(true);
    setError(null);
    try {
      if (createModal === 'order') {
        await api.post('/disputes', {
          routed_order_id: orderId.replace('#', '').trim(),
          dispute_type: disputeType,
          description,
        });
      } else {
        await api.post('/disputes', {
          routed_order_id: orderId.replace('#', '').trim() || '00000000-0000-0000-0000-000000000000',
          dispute_type: disputeType,
          description: `[APP COMPLAINT] ${description}`,
        });
      }
      toast.success(createModal === 'order' ? 'Dispute filed' : 'Complaint submitted');
      setCreateModal(null);
      setOrderId(''); setDescription('');
      setDisputeType(createModal === 'order' ? 'quality_issue' : 'app_bug');
      refetch();
    } catch (err) {
      toast.error('Failed to submit');
      setError(err instanceof Error ? err.message : 'Failed');
    } finally { setCreating(false); }
  }, [orderId, disputeType, description, createModal, refetch, toast]);

  if (loading) return <Page title="Disputes"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const allDisputes = data?.disputes || [];
  const appComplaints = allDisputes.filter(d => d.description.startsWith('[APP COMPLAINT]') || ['app_bug','payment_problem','account_issue','feature_request','policy_violation','app_other'].includes(d.dispute_type));
  const orderDisputes = allDisputes.filter(d => !appComplaints.includes(d));

  const renderDispute = (d: Dispute) => {
    const cfg = statusConfig[d.status] || statusConfig['open'];
    const typeLabel = [...ORDER_DISPUTE_TYPES, ...APP_COMPLAINT_TYPES].find(t => t.value === d.dispute_type)?.label || d.dispute_type;
    const desc = d.description.replace('[APP COMPLAINT] ', '');
    return (
      <Card key={d.id}>
        <div style={{ padding: '2px 0' }}>
          <InlineStack align="space-between" blockAlign="start" wrap={false}>
            <BlockStack gap="100">
              <InlineStack gap="200" blockAlign="center">
                {d.routed_order_id && d.routed_order_id !== '00000000-0000-0000-0000-000000000000' && (
                  <Button variant="plain" onClick={() => navigate(`/orders/${d.routed_order_id}`)}>
                    Order {d.routed_order_id.slice(0, 8)}
                  </Button>
                )}
                <span style={{ padding: '3px 10px', borderRadius: '6px', fontSize: '11px', fontWeight: 600, background: '#f1f5f9', color: '#475569' }}>
                  {typeLabel}
                </span>
                <span style={{ padding: '3px 10px', borderRadius: '20px', fontSize: '11px', fontWeight: 700, color: cfg.color, background: cfg.bg }}>
                  {d.status.replace('_', ' ')}
                </span>
              </InlineStack>
              <Text as="p" variant="bodySm">{desc.length > 120 ? desc.slice(0, 120) + '...' : desc}</Text>
              {d.resolution && (
                <Text as="p" variant="bodySm" tone="success">Resolution: {d.resolution}</Text>
              )}
              <Text as="span" variant="bodySm" tone="subdued">
                {new Date(d.created_at).toLocaleDateString()} {new Date(d.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                {' · '}{d.reporter_role}
              </Text>
            </BlockStack>
          </InlineStack>
        </div>
      </Card>
    );
  };

  return (
    <Page title="Disputes & Complaints">
      <Layout>
        <Layout.Section>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <div className="tab-pills">
              <div className={`tab-pill ${activeTab === 'orders' ? 'tab-pill-active' : ''}`} onClick={() => setActiveTab('orders')}>
                Order Disputes {orderDisputes.length > 0 && <span style={{ marginLeft: '4px', fontSize: '11px' }}>({orderDisputes.length})</span>}
              </div>
              <div className={`tab-pill ${activeTab === 'app' ? 'tab-pill-active' : ''}`} onClick={() => setActiveTab('app')}>
                App Complaints {appComplaints.length > 0 && <span style={{ marginLeft: '4px', fontSize: '11px' }}>({appComplaints.length})</span>}
              </div>
            </div>
            <button
              onClick={() => { setCreateModal(activeTab === 'orders' ? 'order' : 'app'); setDisputeType(activeTab === 'orders' ? 'quality_issue' : 'app_bug'); }}
              style={{
                padding: '8px 20px', fontSize: '13px', fontWeight: 600,
                background: '#111', color: '#fff', border: 'none', borderRadius: '8px', cursor: 'pointer',
              }}
            >
              {activeTab === 'orders' ? 'Report Order Issue' : 'Submit Complaint'}
            </button>
          </div>
        </Layout.Section>

        <Layout.Section>
          {activeTab === 'orders' ? (
            orderDisputes.length > 0 ? (
              <BlockStack gap="300">{orderDisputes.map(renderDispute)}</BlockStack>
            ) : (
              <Card><EmptyState heading="No order disputes" image="">
                <p>File a dispute if you have a problem with a specific order — quality, delivery, or payment issues.</p>
              </EmptyState></Card>
            )
          ) : (
            appComplaints.length > 0 ? (
              <BlockStack gap="300">{appComplaints.map(renderDispute)}</BlockStack>
            ) : (
              <Card><EmptyState heading="No app complaints" image="">
                <p>Submit a complaint about app bugs, payment problems, policy violations, or feature requests.</p>
              </EmptyState></Card>
            )
          )}
        </Layout.Section>

        {totalPages > 1 && (
          <Layout.Section>
            <InlineStack align="center" gap="200">
              <Button disabled={page === 0} onClick={() => setPage(p => p - 1)}>Previous</Button>
              <Text as="span" variant="bodySm">Page {page + 1} of {totalPages}</Text>
              <Button disabled={page >= totalPages - 1} onClick={() => setPage(p => p + 1)}>Next</Button>
            </InlineStack>
          </Layout.Section>
        )}
      </Layout>

      {createModal && (
        <Modal open onClose={() => setCreateModal(null)}
          title={createModal === 'order' ? 'Report Order Issue' : 'Submit App Complaint'}
          primaryAction={{ content: 'Submit', onAction: handleCreate, loading: creating, disabled: (createModal === 'order' && !orderId) || !description }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setCreateModal(null) }]}
        >
          <Modal.Section>
            <BlockStack gap="400">
              {error && <Banner tone="critical">{error}</Banner>}
              <FormLayout>
                {createModal === 'order' && (
                  <TextField label="Order ID or Number" value={orderId} onChange={setOrderId} autoComplete="off"
                    helpText="Enter the order number (e.g. 1005)" />
                )}
                <Select label="Issue Type"
                  options={createModal === 'order' ? ORDER_DISPUTE_TYPES : APP_COMPLAINT_TYPES}
                  value={disputeType} onChange={setDisputeType} />
                <TextField label="Description" value={description} onChange={setDescription}
                  multiline={4} autoComplete="off"
                  helpText={createModal === 'order' ? 'Describe the problem with this order' : 'Describe your issue or suggestion'} />
              </FormLayout>
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
