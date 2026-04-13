import { useState, useCallback, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, DataTable, Badge, Button, Spinner,
  Banner, BlockStack, Text, TextField, FormLayout, Modal,
  InlineStack, Divider,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';
import { RoutedOrder, FulfillmentEvent } from '../types';
import ConfirmDialog from '../components/ConfirmDialog';

interface OrderDetailResponse {
  order: RoutedOrder;
  fulfillments: FulfillmentEvent[];
}

interface OrderDetailProps {
  role: string;
}

const statusConfig: Record<string, { color: string; bg: string; label: string }> = {
  pending:    { color: '#92400e', bg: '#fef3c7', label: 'Pending' },
  accepted:   { color: '#1e40af', bg: '#dbeafe', label: 'Accepted' },
  processing: { color: '#6d28d9', bg: '#ede9fe', label: 'Processing' },
  fulfilled:  { color: '#166534', bg: '#dcfce7', label: 'Fulfilled' },
  rejected:   { color: '#991b1b', bg: '#fee2e2', label: 'Rejected' },
  cancelled:  { color: '#991b1b', bg: '#fee2e2', label: 'Cancelled' },
  unfulfilled:{ color: '#92400e', bg: '#fef3c7', label: 'Unfulfilled' },
};

const STEPS = ['pending', 'accepted', 'processing', 'fulfilled'];

function OrderCommentsSection({ orderId }: { orderId: string }) {
  const [comments, setComments] = useState<Array<{ id: string; shop_role: string; content: string; created_at: string }>>([]);
  const [newComment, setNewComment] = useState('');
  const [sending, setSending] = useState(false);

  useEffect(() => {
    api.get<{ comments: typeof comments }>(`/orders/${orderId}/comments`)
      .then(d => setComments(d.comments || []))
      .catch(() => {});
  }, [orderId]);

  const handleSend = async () => {
    if (!newComment.trim()) return;
    setSending(true);
    try {
      await api.post(`/orders/${orderId}/comments`, { content: newComment });
      setNewComment('');
      const d = await api.get<{ comments: typeof comments }>(`/orders/${orderId}/comments`);
      setComments(d.comments || []);
    } catch { /* */ }
    finally { setSending(false); }
  };

  return (
    <BlockStack gap="300">
      {comments.length > 0 ? comments.map(c => (
        <div key={c.id} style={{
          padding: '10px 14px', borderRadius: '10px',
          background: c.shop_role === 'supplier' ? '#f0f9ff' : '#f8fafc',
          border: '1px solid #e2e8f0',
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
            <span style={{
              fontSize: '11px', fontWeight: 600, textTransform: 'uppercase',
              color: c.shop_role === 'supplier' ? '#1e40af' : '#059669',
            }}>
              {c.shop_role}
            </span>
            <span style={{ fontSize: '11px', color: '#94a3b8' }}>
              {new Date(c.created_at).toLocaleString()}
            </span>
          </div>
          <p style={{ margin: 0, fontSize: '14px', color: '#1e293b' }}>{c.content}</p>
        </div>
      )) : (
        <Text as="p" variant="bodySm" tone="subdued">No comments yet</Text>
      )}
      <div style={{ display: 'flex', gap: '8px', alignItems: 'flex-end' }}>
        <div style={{ flex: 1 }}>
          <TextField label="" labelHidden value={newComment} onChange={setNewComment} placeholder="Write a comment..." autoComplete="off" />
        </div>
        <Button onClick={handleSend} loading={sending} disabled={!newComment.trim()}>Send</Button>
      </div>
    </BlockStack>
  );
}

export default function OrderDetail({ role }: OrderDetailProps) {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();
  const { data, loading, error, refetch } = useApi<OrderDetailResponse>(`/orders/${id}`);

  const [fulfillModal, setFulfillModal] = useState(false);
  const [trackingNumber, setTrackingNumber] = useState('');
  const [trackingUrl, setTrackingUrl] = useState('');
  const [trackingCompany, setTrackingCompany] = useState('');
  const [fulfilling, setFulfilling] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [confirmAccept, setConfirmAccept] = useState(false);
  const [confirmReject, setConfirmReject] = useState(false);

  const handleAccept = useCallback(async () => {
    try { await api.post(`/supplier/orders/${id}/accept`); toast.success('Order accepted'); refetch(); }
    catch (err) { toast.error('Failed to accept order'); setActionError(err instanceof Error ? err.message : 'Failed'); }
  }, [id, refetch, toast]);

  const handleReject = useCallback(async () => {
    try { await api.post(`/supplier/orders/${id}/reject`, { reason: 'Rejected by supplier' }); toast.success('Order rejected'); refetch(); }
    catch (err) { toast.error('Failed to reject order'); setActionError(err instanceof Error ? err.message : 'Failed'); }
  }, [id, refetch, toast]);

  const handleFulfill = useCallback(async () => {
    setFulfilling(true); setActionError(null);
    try {
      await api.post(`/supplier/orders/${id}/fulfill`, { routed_order_id: id, tracking_number: trackingNumber, tracking_url: trackingUrl, tracking_company: trackingCompany });
      toast.success('Order fulfilled');
      setFulfillModal(false); refetch();
    } catch (err) { toast.error('Failed to fulfill order'); setActionError(err instanceof Error ? err.message : 'Fulfillment failed'); }
    finally { setFulfilling(false); }
  }, [id, trackingNumber, trackingUrl, trackingCompany, refetch]);

  if (loading) return <Page title="Order Detail"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  if (error || !data) return <Page title="Order Detail"><Banner tone="critical">{error || 'Order not found'}</Banner></Page>;

  const { order, fulfillments } = data;
  const isSupplier = role === 'supplier';
  const canAccept = isSupplier && order.status === 'pending';
  const canFulfill = isSupplier && (order.status === 'accepted' || order.status === 'processing');
  const cfg = statusConfig[order.status] || statusConfig['pending'];
  const currentStep = STEPS.indexOf(order.status);
  const isRejected = order.status === 'rejected' || order.status === 'cancelled';

  return (
    <Page
      title={`Order ${order.reseller_order_number || order.id.slice(0, 8)}`}
      backAction={{ content: 'Orders', onAction: () => navigate('/orders') }}
      primaryAction={canFulfill ? { content: 'Add Fulfillment', onAction: () => setFulfillModal(true) } : undefined}
    >
      <Layout>
        {actionError && <Layout.Section><Banner tone="critical" onDismiss={() => setActionError(null)}>{actionError}</Banner></Layout.Section>}

        {/* Hero card: status + key info */}
        <Layout.Section>
          <div style={{
            background: 'linear-gradient(135deg, #0f172a 0%, #1e3a8a 100%)',
            borderRadius: '14px', padding: '28px 32px', color: '#fff',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '20px' }}>
              {/* Left: order info */}
              <div>
                <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '4px' }}>
                  Order
                </div>
                <div style={{ fontSize: '28px', fontWeight: 700, marginBottom: '8px' }}>
                  #{order.reseller_order_number || order.id.slice(0, 8)}
                </div>
                <div style={{ display: 'flex', gap: '24px', flexWrap: 'wrap' }}>
                  <div>
                    <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.5)' }}>Amount</div>
                    <div style={{ fontSize: '20px', fontWeight: 600 }}>${order.total_wholesale_amount.toFixed(2)} <span style={{ fontSize: '13px', opacity: 0.6 }}>{order.currency}</span></div>
                  </div>
                  <div>
                    <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.5)' }}>{isSupplier ? 'Reseller' : 'Supplier'}</div>
                    <div style={{ fontSize: '14px', fontWeight: 500 }}>{isSupplier ? (order.reseller_shop_name || '-') : (order.supplier_shop_name || '-')}</div>
                  </div>
                  <div>
                    <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.5)' }}>Date</div>
                    <div style={{ fontSize: '14px' }}>{new Date(order.created_at).toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })}</div>
                  </div>
                  <div>
                    <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.5)' }}>Items</div>
                    <div style={{ fontSize: '14px' }}>{order.items?.length || 0} product{(order.items?.length || 0) !== 1 ? 's' : ''}</div>
                  </div>
                </div>
              </div>
              {/* Right: status */}
              <div style={{ textAlign: 'right' }}>
                <span style={{
                  padding: '6px 18px', borderRadius: '24px', fontSize: '13px', fontWeight: 700,
                  color: cfg.color, background: cfg.bg,
                }}>
                  {cfg.label}
                </span>
              </div>
            </div>

            {/* Progress bar */}
            {!isRejected && (
              <div style={{ marginTop: '24px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                  {STEPS.map((step, i) => (
                    <div key={step} style={{ display: 'flex', alignItems: 'center', flex: 1 }}>
                      <div style={{
                        width: '28px', height: '28px', borderRadius: '50%', flexShrink: 0,
                        background: i <= currentStep ? '#3b82f6' : 'rgba(255,255,255,0.15)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontSize: '12px', fontWeight: 700, color: i <= currentStep ? '#fff' : 'rgba(255,255,255,0.4)',
                        border: i === currentStep ? '2px solid #93c5fd' : 'none',
                      }}>
                        {i <= currentStep ? '✓' : i + 1}
                      </div>
                      {i < STEPS.length - 1 && (
                        <div style={{
                          flex: 1, height: '3px', margin: '0 4px',
                          background: i < currentStep ? '#3b82f6' : 'rgba(255,255,255,0.15)',
                          borderRadius: '2px',
                        }} />
                      )}
                    </div>
                  ))}
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '6px' }}>
                  {STEPS.map((step, i) => (
                    <span key={step} style={{
                      fontSize: '11px', textAlign: 'center', flex: 1,
                      color: i <= currentStep ? 'rgba(255,255,255,0.9)' : 'rgba(255,255,255,0.35)',
                      fontWeight: i === currentStep ? 600 : 400,
                    }}>
                      {step.charAt(0).toUpperCase() + step.slice(1)}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </Layout.Section>

        {/* Accept / Reject buttons */}
        {canAccept && (
          <Layout.Section>
            <InlineStack gap="300">
              <button onClick={() => setConfirmAccept(true)} style={{
                padding: '12px 32px', fontSize: '15px', fontWeight: 600,
                background: '#111', color: '#fff', border: 'none', borderRadius: '10px', cursor: 'pointer',
              }}>Accept Order</button>
              <button onClick={() => setConfirmReject(true)} style={{
                padding: '12px 32px', fontSize: '15px', fontWeight: 600,
                background: '#fff', color: '#111', border: '2px solid #111', borderRadius: '10px', cursor: 'pointer',
              }}>Reject Order</button>
            </InlineStack>
          </Layout.Section>
        )}

        {/* Shipping + Customer */}
        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Shipping Address</Text>
              <Divider />
              <Text as="p" variant="bodyMd" fontWeight="semibold">{order.customer_shipping_name || 'N/A'}</Text>
              {order.customer_shipping_address && (
                <Text as="p" variant="bodySm" tone="subdued">
                  {[
                    order.customer_shipping_address.address1,
                    order.customer_shipping_address.address2,
                    order.customer_shipping_address.city,
                    order.customer_shipping_address.province,
                    order.customer_shipping_address.zip,
                    order.customer_shipping_address.country,
                  ].filter(Boolean).join(', ')}
                </Text>
              )}
              {order.customer_email && <Text as="p" variant="bodySm">{order.customer_email}</Text>}
              {order.customer_phone && <Text as="p" variant="bodySm">{order.customer_phone}</Text>}
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Order Summary</Text>
              <Divider />
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div>
                  <Text as="span" variant="bodySm" tone="subdued">Subtotal</Text>
                  <div style={{ fontSize: '16px', fontWeight: 600 }}>${order.total_wholesale_amount.toFixed(2)}</div>
                </div>
                <div>
                  <Text as="span" variant="bodySm" tone="subdued">Currency</Text>
                  <div style={{ fontSize: '16px', fontWeight: 600 }}>{order.currency}</div>
                </div>
                <div>
                  <Text as="span" variant="bodySm" tone="subdued">Items</Text>
                  <div style={{ fontSize: '16px', fontWeight: 600 }}>{order.items?.length || 0}</div>
                </div>
                <div>
                  <Text as="span" variant="bodySm" tone="subdued">Created</Text>
                  <div style={{ fontSize: '13px', fontWeight: 500 }}>{new Date(order.created_at).toLocaleDateString()}</div>
                </div>
              </div>
            </BlockStack>
          </Card>
        </Layout.Section>

        {/* Line Items */}
        <Layout.Section>
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Products</Text>
              <Divider />
              {order.items && order.items.length > 0 ? (
                <BlockStack gap="200">
                  {order.items.map((item) => {
                    const itemCfg = statusConfig[item.fulfillment_status] || statusConfig['unfulfilled'];
                    return (
                      <div key={item.id} style={{
                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                        padding: '12px 14px', borderRadius: '10px', background: '#f8fafc',
                        border: '1px solid #f1f5f9',
                      }}>
                        <div>
                          <div style={{ fontSize: '14px', fontWeight: 600, color: '#1e293b' }}>{item.title}</div>
                          <div style={{ fontSize: '12px', color: '#94a3b8', marginTop: '2px' }}>
                            {item.sku ? `SKU: ${item.sku} \u00b7 ` : ''}Qty: {item.quantity}
                          </div>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                          <span style={{
                            padding: '2px 10px', borderRadius: '12px', fontSize: '11px', fontWeight: 600,
                            color: itemCfg.color, background: itemCfg.bg,
                          }}>
                            {item.fulfillment_status || 'unfulfilled'}
                          </span>
                          <div style={{ fontSize: '15px', fontWeight: 700, color: '#1e293b' }}>
                            ${(item.wholesale_unit_price * item.quantity).toFixed(2)}
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </BlockStack>
              ) : (
                <Text as="p" tone="subdued">No line items</Text>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>

        {/* Fulfillment History */}
        {fulfillments && fulfillments.length > 0 && (
          <Layout.Section>
            <Card>
              <BlockStack gap="300">
                <Text as="h2" variant="headingMd">Fulfillment History</Text>
                <Divider />
                <DataTable
                  columnContentTypes={['text', 'text', 'text', 'text', 'text']}
                  headings={['Tracking', 'Carrier', 'Status', 'Synced', 'Date']}
                  rows={fulfillments.map((f) => [
                    f.tracking_url ? <a href={f.tracking_url} target="_blank" rel="noopener noreferrer" key={f.id}>{f.tracking_number}</a> : f.tracking_number,
                    f.tracking_company || '-',
                    <Badge key={`st-${f.id}`} tone={f.status === 'fulfilled' ? 'success' : 'attention'}>{f.status}</Badge>,
                    f.synced_to_reseller ? <Badge key={`s-${f.id}`} tone="success">Synced</Badge> : <Badge key={`s-${f.id}`}>Pending</Badge>,
                    new Date(f.created_at).toLocaleString(),
                  ])}
                />
              </BlockStack>
            </Card>
          </Layout.Section>
        )}

        {/* Comments */}
        <Layout.Section>
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Comments</Text>
              <Divider />
              <OrderCommentsSection orderId={id!} />
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          <InlineStack gap="200">
            {!isSupplier && order.status === 'fulfilled' && (
              <Button onClick={() => {
                api.post('/returns', { routed_order_id: order.id, reason: 'Customer requested return' })
                  .then(() => { toast.success('Return request sent to supplier'); })
                  .catch(() => { toast.error('Failed to create return request'); });
              }}>Request Return</Button>
            )}
            <Button onClick={() => navigate('/disputes')} tone="critical">Report Issue</Button>
          </InlineStack>
        </Layout.Section>
      </Layout>

      {fulfillModal && (
        <Modal open onClose={() => setFulfillModal(false)} title="Add Fulfillment"
          primaryAction={{ content: 'Submit Fulfillment', onAction: handleFulfill, loading: fulfilling }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setFulfillModal(false) }]}
        >
          <Modal.Section>
            <FormLayout>
              <TextField label="Tracking number" value={trackingNumber} onChange={setTrackingNumber} autoComplete="off" />
              <TextField label="Tracking URL" value={trackingUrl} onChange={setTrackingUrl} autoComplete="url" />
              <TextField label="Carrier/company" value={trackingCompany} onChange={setTrackingCompany} autoComplete="off" />
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}

      <ConfirmDialog
        open={confirmAccept}
        title="Accept Order"
        message={`Are you sure you want to accept order #${order?.reseller_order_number || order?.id?.slice(0, 8) || ''}? You will be responsible for fulfilling this order.`}
        confirmLabel="Yes, Accept"
        onConfirm={() => { setConfirmAccept(false); handleAccept(); }}
        onCancel={() => setConfirmAccept(false)}
      />

      <ConfirmDialog
        open={confirmReject}
        title="Reject Order"
        message={`Are you sure you want to reject order #${order?.reseller_order_number || order?.id?.slice(0, 8) || ''}? The reserved stock will be restored.`}
        confirmLabel="Yes, Reject"
        destructive
        onConfirm={() => { setConfirmReject(false); handleReject(); }}
        onCancel={() => setConfirmReject(false)}
      />
    </Page>
  );
}
