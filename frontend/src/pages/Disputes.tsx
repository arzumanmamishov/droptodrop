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

const ORDER_TYPES = [
  { label: 'Quality Issue', value: 'quality_issue', icon: '🔍' },
  { label: 'Wrong Item', value: 'wrong_item', icon: '❌' },
  { label: 'Not Received', value: 'not_received', icon: '📭' },
  { label: 'Damaged', value: 'damaged', icon: '💔' },
  { label: 'Late Delivery', value: 'late_delivery', icon: '🕐' },
  { label: 'Missing Items', value: 'missing_items', icon: '📦' },
  { label: 'Other', value: 'other', icon: '📝' },
];

const APP_TYPES = [
  { label: 'App Bug / Technical Issue', value: 'app_bug', icon: '🐛' },
  { label: 'Payment Problem', value: 'payment_problem', icon: '💳' },
  { label: 'Account Issue', value: 'account_issue', icon: '👤' },
  { label: 'Feature Request', value: 'feature_request', icon: '💡' },
  { label: 'Policy Violation', value: 'policy_violation', icon: '⚖️' },
  { label: 'Other', value: 'app_other', icon: '📝' },
];

const ALL_TYPES = [...ORDER_TYPES, ...APP_TYPES];
const APP_TYPE_VALUES = APP_TYPES.map(t => t.value);

const statusStyles: Record<string, { color: string; bg: string; label: string; icon: string }> = {
  open:      { color: '#92400e', bg: '#fef3c7', label: 'Open', icon: '🟡' },
  in_review: { color: '#1e40af', bg: '#dbeafe', label: 'In Review', icon: '🔵' },
  resolved:  { color: '#166534', bg: '#dcfce7', label: 'Resolved', icon: '🟢' },
  closed:    { color: '#64748b', bg: '#f1f5f9', label: 'Closed', icon: '⚫' },
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
  const { data, loading, refetch } = useApi<DisputesResponse>(`/disputes?limit=${limit}&offset=${page * limit}`);
  const totalPages = Math.ceil((data?.total || 0) / limit);

  const handleCreate = useCallback(async () => {
    setCreating(true); setError(null);
    try {
      if (createModal === 'order') {
        await api.post('/disputes', { routed_order_id: orderId.replace('#', '').trim(), dispute_type: disputeType, description });
      } else {
        await api.post('/disputes', { routed_order_id: '', dispute_type: disputeType, description: `[APP COMPLAINT] ${description}` });
      }
      toast.success(createModal === 'order' ? 'Dispute filed' : 'Complaint submitted');
      setCreateModal(null); setOrderId(''); setDescription('');
      setDisputeType(createModal === 'order' ? 'quality_issue' : 'app_bug');
      refetch();
    } catch (err) {
      toast.error('Failed to submit');
      setError(err instanceof Error ? err.message : 'Failed');
    } finally { setCreating(false); }
  }, [orderId, disputeType, description, createModal, refetch, toast]);

  if (loading) return <Page title="Disputes"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const allDisputes = data?.disputes || [];
  const appComplaints = allDisputes.filter(d => d.description.startsWith('[APP COMPLAINT]') || APP_TYPE_VALUES.includes(d.dispute_type));
  const orderDisputes = allDisputes.filter(d => !appComplaints.includes(d));
  const currentList = activeTab === 'orders' ? orderDisputes : appComplaints;

  const openCount = currentList.filter(d => d.status === 'open').length;
  const resolvedCount = currentList.filter(d => d.status === 'resolved').length;

  return (
    <Page title="Disputes & Complaints">
      <Layout>
        {/* Tab bar */}
        <Layout.Section>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '12px' }}>
            <div style={{ display: 'flex', gap: '0', background: '#f1f5f9', borderRadius: '12px', padding: '4px' }}>
              <button
                onClick={() => setActiveTab('orders')}
                style={{
                  padding: '10px 24px', fontSize: '13px', fontWeight: 600, border: 'none', borderRadius: '10px', cursor: 'pointer',
                  background: activeTab === 'orders' ? '#fff' : 'transparent',
                  color: activeTab === 'orders' ? '#1e293b' : '#64748b',
                  boxShadow: activeTab === 'orders' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                }}
              >
                📦 Order Disputes
                {orderDisputes.length > 0 && <span style={{ marginLeft: '6px', fontSize: '11px', padding: '1px 7px', borderRadius: '10px', background: '#fee2e2', color: '#991b1b' }}>{orderDisputes.length}</span>}
              </button>
              <button
                onClick={() => setActiveTab('app')}
                style={{
                  padding: '10px 24px', fontSize: '13px', fontWeight: 600, border: 'none', borderRadius: '10px', cursor: 'pointer',
                  background: activeTab === 'app' ? '#fff' : 'transparent',
                  color: activeTab === 'app' ? '#1e293b' : '#64748b',
                  boxShadow: activeTab === 'app' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                }}
              >
                🛠️ App Complaints
                {appComplaints.length > 0 && <span style={{ marginLeft: '6px', fontSize: '11px', padding: '1px 7px', borderRadius: '10px', background: '#dbeafe', color: '#1e40af' }}>{appComplaints.length}</span>}
              </button>
            </div>
            <button
              onClick={() => { setCreateModal(activeTab === 'orders' ? 'order' : 'app'); setDisputeType(activeTab === 'orders' ? 'quality_issue' : 'app_bug'); }}
              style={{
                padding: '10px 24px', fontSize: '13px', fontWeight: 600,
                background: '#111', color: '#fff', border: 'none', borderRadius: '10px', cursor: 'pointer',
              }}
            >
              {activeTab === 'orders' ? '+ Report Issue' : '+ Submit Complaint'}
            </button>
          </div>
        </Layout.Section>

        {/* Stats */}
        {currentList.length > 0 && (
          <Layout.Section>
            <div style={{ display: 'flex', gap: '12px' }}>
              <div style={{ flex: 1, background: '#fff', borderRadius: '14px', padding: '16px 20px', border: '1px solid #f1f5f9', display: 'flex', alignItems: 'center', gap: '12px' }}>
                <div style={{ width: '40px', height: '40px', borderRadius: '10px', background: '#fef3c7', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '18px' }}>🟡</div>
                <div><div style={{ fontSize: '22px', fontWeight: 700 }}>{openCount}</div><div style={{ fontSize: '12px', color: '#94a3b8' }}>Open</div></div>
              </div>
              <div style={{ flex: 1, background: '#fff', borderRadius: '14px', padding: '16px 20px', border: '1px solid #f1f5f9', display: 'flex', alignItems: 'center', gap: '12px' }}>
                <div style={{ width: '40px', height: '40px', borderRadius: '10px', background: '#dcfce7', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '18px' }}>🟢</div>
                <div><div style={{ fontSize: '22px', fontWeight: 700 }}>{resolvedCount}</div><div style={{ fontSize: '12px', color: '#94a3b8' }}>Resolved</div></div>
              </div>
              <div style={{ flex: 1, background: '#fff', borderRadius: '14px', padding: '16px 20px', border: '1px solid #f1f5f9', display: 'flex', alignItems: 'center', gap: '12px' }}>
                <div style={{ width: '40px', height: '40px', borderRadius: '10px', background: '#f1f5f9', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '18px' }}>📊</div>
                <div><div style={{ fontSize: '22px', fontWeight: 700 }}>{currentList.length}</div><div style={{ fontSize: '12px', color: '#94a3b8' }}>Total</div></div>
              </div>
            </div>
          </Layout.Section>
        )}

        {/* Disputes list */}
        <Layout.Section>
          {currentList.length > 0 ? (
            <BlockStack gap="300">
              {currentList.map((d) => {
                const sCfg = statusStyles[d.status] || statusStyles['open'];
                const typeInfo = ALL_TYPES.find(t => t.value === d.dispute_type);
                const desc = d.description.replace('[APP COMPLAINT] ', '');
                return (
                  <Card key={d.id}>
                    <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-start' }}>
                      {/* Icon */}
                      <div style={{
                        width: '44px', height: '44px', borderRadius: '12px', flexShrink: 0,
                        background: sCfg.bg, display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontSize: '20px',
                      }}>
                        {typeInfo?.icon || '📝'}
                      </div>

                      {/* Content */}
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '6px' }}>
                          <div>
                            <div style={{ fontSize: '14px', fontWeight: 700, color: '#1e293b' }}>
                              {typeInfo?.label || d.dispute_type}
                            </div>
                            <div style={{ fontSize: '12px', color: '#94a3b8', marginTop: '2px' }}>
                              {new Date(d.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })} {new Date(d.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                              {' · '}{d.reporter_role}
                              {d.routed_order_id && d.routed_order_id !== '00000000-0000-0000-0000-000000000000' && (
                                <span> · <a onClick={() => navigate(`/orders/${d.routed_order_id}`)} style={{ color: '#1e40af', cursor: 'pointer', textDecoration: 'underline' }}>View Order</a></span>
                              )}
                            </div>
                          </div>
                          <span style={{
                            padding: '4px 14px', borderRadius: '20px', fontSize: '12px', fontWeight: 700,
                            color: sCfg.color, background: sCfg.bg, whiteSpace: 'nowrap',
                          }}>
                            {sCfg.icon} {sCfg.label}
                          </span>
                        </div>

                        <div style={{
                          fontSize: '13px', color: '#475569', lineHeight: '1.5',
                          background: '#f8fafc', borderRadius: '10px', padding: '10px 14px', marginBottom: '6px',
                        }}>
                          {desc}
                        </div>

                        {d.resolution && (
                          <div style={{
                            fontSize: '13px', color: '#166534', lineHeight: '1.5',
                            background: '#f0fdf4', borderRadius: '10px', padding: '10px 14px',
                            border: '1px solid #dcfce7',
                          }}>
                            <strong>Resolution:</strong> {d.resolution}
                          </div>
                        )}
                      </div>
                    </div>
                  </Card>
                );
              })}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading={activeTab === 'orders' ? 'No order disputes' : 'No app complaints'} image="">
                <p>{activeTab === 'orders'
                  ? 'File a dispute if you have a problem with a specific order.'
                  : 'Submit a complaint about app issues, payments, or feature requests.'
                }</p>
              </EmptyState>
            </Card>
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
          title={createModal === 'order' ? '📦 Report Order Issue' : '🛠️ Submit App Complaint'}
          primaryAction={{ content: 'Submit', onAction: handleCreate, loading: creating, disabled: (createModal === 'order' && !orderId) || !description }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setCreateModal(null) }]}
        >
          <Modal.Section>
            <BlockStack gap="400">
              {error && <Banner tone="critical">{error}</Banner>}
              <FormLayout>
                {createModal === 'order' && (
                  <TextField label="Order ID or Number" value={orderId} onChange={setOrderId} autoComplete="off"
                    helpText="Enter the order number (e.g. 1005)" placeholder="#1005" />
                )}
                <Select label="Issue Type"
                  options={createModal === 'order' ? ORDER_TYPES.map(t => ({ label: `${t.icon} ${t.label}`, value: t.value })) : APP_TYPES.map(t => ({ label: `${t.icon} ${t.label}`, value: t.value }))}
                  value={disputeType} onChange={setDisputeType} />
                <TextField label="Description" value={description} onChange={setDescription}
                  multiline={4} autoComplete="off"
                  helpText={createModal === 'order' ? 'Describe the problem with this order in detail' : 'Describe your issue, suggestion, or feedback'} />
              </FormLayout>
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
