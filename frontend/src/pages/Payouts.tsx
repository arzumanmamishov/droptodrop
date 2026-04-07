import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Spinner,
  Banner, InlineStack, Divider, EmptyState, Button,
  Modal, TextField, FormLayout,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';
import ConfirmDialog from '../components/ConfirmDialog';

interface PayoutOrder {
  id: string;
  order_number: string;
  status: string;
  wholesale: number;
  currency: string;
  domain: string;
  pay_status: string;
  platform_fee: number;
  supplier_payout: number;
  products: string;
  created_at: string;
  supplier_paypal: string;
}

interface PayoutsResponse {
  payouts: PayoutOrder[];
  total: number;
  grand_total: number;
  grand_paid: number;
  grand_balance: number;
  grand_fees: number;
  has_paypal: boolean;
}

interface Props { role: string; }

export default function Payouts({ role }: Props) {
  const [page, setPage] = useState(0);
  const limit = 20;
  const { data, loading, refetch } = useApi<PayoutsResponse>(`/payouts?limit=${limit}&offset=${page * limit}`);
  const toast = useToast();
  const [confirmAction, setConfirmAction] = useState<{ order: PayoutOrder; action: string } | null>(null);
  const [acting, setActing] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);
  const [paypalModal, setPaypalModal] = useState(false);
  const [paypalEmail, setPaypalEmail] = useState('');
  const [savingPaypal, setSavingPaypal] = useState(false);

  const isSupplier = role === 'supplier';

  const handleSavePaypal = useCallback(async () => {
    if (!paypalEmail.trim()) return;
    setSavingPaypal(true);
    try {
      const endpoint = isSupplier ? '/supplier/profile' : '/reseller/profile';
      await api.put(endpoint, { paypal_email: paypalEmail.trim() });
      setPaypalModal(false);
      setSuccess('PayPal email saved successfully.');
      toast.success('PayPal email saved');
      refetch();
    } catch { toast.error('Failed to save PayPal email'); }
    finally { setSavingPaypal(false); }
  }, [paypalEmail, isSupplier, refetch, toast]);

  const handleAction = useCallback(async () => {
    if (!confirmAction) return;
    setActing(true);
    try {
      const { order, action } = confirmAction;
      await api.post(`/payouts/${action}/${order.id}`);
      const messages: Record<string, string> = {
        'send-payment': 'Payment sent. Waiting for supplier confirmation.',
        'confirm-received': 'Payment confirmed.',
        'dispute-payment': 'Payment disputed. Reseller notified.',
      };
      setSuccess(messages[action] || 'Done');
      toast.success(messages[action] || 'Done');
      setConfirmAction(null);
      refetch();
    } catch { toast.error('Failed to process payment action'); }
    finally { setActing(false); }
  }, [confirmAction, refetch, toast]);

  if (loading) {
    return <Page title="Payouts"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const payouts = data?.payouts || [];
  const totalPages = Math.ceil((data?.total || 0) / limit);

  const getPayAmount = (p: PayoutOrder) => p.supplier_payout > 0 ? p.supplier_payout : p.wholesale;

  const statusConfig: Record<string, { color: string; bg: string; label: string }> = {
    pending:      { color: '#92400e', bg: '#fef3c7', label: 'Awaiting Payment' },
    unpaid:       { color: '#92400e', bg: '#fef3c7', label: 'Awaiting Payment' },
    payment_sent: { color: '#1e40af', bg: '#dbeafe', label: 'Sent' },
    paid:         { color: '#166534', bg: '#dcfce7', label: 'Paid' },
    disputed:     { color: '#991b1b', bg: '#fee2e2', label: 'Disputed' },
  };

  const getConfirmMessage = () => {
    if (!confirmAction) return '';
    const { order, action } = confirmAction;
    const amt = getPayAmount(order).toFixed(2);
    const orderLabel = order.order_number || order.id.slice(0, 8);
    if (action === 'send-payment') return `Confirm you sent $${amt} to the supplier for order ${orderLabel}?${order.platform_fee > 0 ? ` (Platform fee of $${order.platform_fee.toFixed(2)} is collected separately.)` : ''}`;
    if (action === 'confirm-received') return `Confirm you received $${amt} for order ${orderLabel}?`;
    if (action === 'dispute-payment') return `Report that you have NOT received payment for order ${orderLabel}?`;
    return '';
  };

  return (
    <Page title="Payouts" subtitle={isSupplier ? 'Money owed to you' : 'Money you owe suppliers'}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(null)}>{success}</Banner></Layout.Section>}

        {data && !data.has_paypal && (
          <Layout.Section>
            <Banner
              tone="warning"
              action={{
                content: 'Add PayPal Email',
                onAction: () => setPaypalModal(true),
              }}
            >
              {isSupplier
                ? 'Add your PayPal email so resellers can pay you directly with one click.'
                : 'Add your PayPal email so suppliers can verify your payments.'}
            </Banner>
          </Layout.Section>
        )}

        {/* Summary cards */}
        <Layout.Section>
          <div style={{ display: 'grid', gridTemplateColumns: !isSupplier && (data?.grand_fees || 0) > 0 ? 'repeat(4, 1fr)' : 'repeat(3, 1fr)', gap: '12px' }}>
            <div className="stat-card">
              <div className="stat-card-label">{isSupplier ? 'Total Earned' : 'Total Owed'}</div>
              <div className="stat-card-value">${(data?.grand_total || 0).toFixed(2)}</div>
            </div>
            <div className="stat-card">
              <div className="stat-card-label">Confirmed Paid</div>
              <div className="stat-card-value" style={{ color: '#1e40af' }}>${(data?.grand_paid || 0).toFixed(2)}</div>
            </div>
            <div className="stat-card">
              <div className="stat-card-label">Outstanding</div>
              <div className="stat-card-value" style={{ color: (data?.grand_balance || 0) > 0 ? '#b91c1c' : '#1e40af' }}>
                ${(data?.grand_balance || 0).toFixed(2)}
              </div>
            </div>
            {!isSupplier && (data?.grand_fees || 0) > 0 && (
              <div className="stat-card">
                <div className="stat-card-label">Platform Fees</div>
                <div className="stat-card-value" style={{ color: '#6b7280' }}>${(data?.grand_fees || 0).toFixed(2)}</div>
              </div>
            )}
          </div>
        </Layout.Section>

        {/* Payout list */}
        <Layout.Section>
          {payouts.length > 0 ? (
            <BlockStack gap="300">
              {payouts.map((p) => {
                const amt = getPayAmount(p);
                const cfg = statusConfig[p.pay_status] || statusConfig['unpaid'];
                const needsResellerAction = !isSupplier && (p.pay_status === 'pending' || p.pay_status === 'unpaid' || p.pay_status === 'disputed');
                const needsSupplierAction = isSupplier && p.pay_status === 'payment_sent';

                return (
                  <Card key={p.id}>
                    <div style={{ padding: '4px 0' }}>
                      {/* Header row */}
                      <InlineStack align="space-between" blockAlign="center" wrap={false}>
                        <InlineStack gap="300" blockAlign="center">
                          <div>
                            <Text as="span" variant="bodyMd" fontWeight="semibold">
                              Order {p.order_number || `#${p.id.slice(0, 8)}`}
                            </Text>
                            <div style={{ marginTop: '2px' }}>
                              <Text as="span" variant="bodySm" tone="subdued">
                                {new Date(p.created_at).toLocaleDateString()} &middot; {p.domain}
                              </Text>
                            </div>
                          </div>
                        </InlineStack>
                        <span style={{
                          padding: '4px 12px',
                          borderRadius: '20px',
                          fontSize: '12px',
                          fontWeight: 600,
                          color: cfg.color,
                          background: cfg.bg,
                        }}>
                          {cfg.label}
                        </span>
                      </InlineStack>

                      {/* Products */}
                      {p.products && (
                        <div style={{ margin: '10px 0' }}>
                          <Text as="p" variant="bodySm" tone="subdued">{p.products}</Text>
                        </div>
                      )}

                      <Divider />

                      {/* Amount breakdown + actions */}
                      <div style={{ marginTop: '12px', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', flexWrap: 'wrap', gap: '12px' }}>
                        {/* Left: amounts */}
                        <div style={{ display: 'flex', gap: '24px', alignItems: 'baseline' }}>
                          <div>
                            <Text as="span" variant="bodySm" tone="subdued">
                              {isSupplier ? 'You receive' : 'Pay supplier'}
                            </Text>
                            <div style={{ fontSize: '22px', fontWeight: 700, color: '#111' }}>
                              ${amt.toFixed(2)}
                            </div>
                          </div>
                          {p.platform_fee > 0 && (
                            <div>
                              <Text as="span" variant="bodySm" tone="subdued">Platform fee</Text>
                              <div style={{ fontSize: '14px', fontWeight: 600, color: '#6b7280' }}>
                                ${p.platform_fee.toFixed(2)}
                              </div>
                            </div>
                          )}
                          {p.platform_fee > 0 && (
                            <div>
                              <Text as="span" variant="bodySm" tone="subdued">Wholesale</Text>
                              <div style={{ fontSize: '14px', fontWeight: 500, color: '#9ca3af' }}>
                                ${p.wholesale.toFixed(2)}
                              </div>
                            </div>
                          )}
                        </div>

                        {/* Right: action buttons */}
                        <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
                          {/* RESELLER actions */}
                          {needsResellerAction && (
                            <>
                              {p.supplier_paypal ? (
                                <a
                                  href={`https://paypal.me/${p.supplier_paypal}/${amt.toFixed(2)}${p.currency || 'USD'}`}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  style={{
                                    display: 'inline-flex',
                                    alignItems: 'center',
                                    gap: '6px',
                                    padding: '8px 16px',
                                    borderRadius: '8px',
                                    background: '#0070ba',
                                    color: 'white',
                                    fontWeight: 600,
                                    fontSize: '14px',
                                    textDecoration: 'none',
                                    transition: 'background 0.15s',
                                  }}
                                >
                                  <svg width="18" height="18" viewBox="0 0 24 24" fill="white"><path d="M7.076 21.337H2.47a.641.641 0 0 1-.633-.74L4.944 2.23A.774.774 0 0 1 5.708 1.6h6.627c2.2 0 3.958.654 5.077 1.89.467.516.804 1.104.998 1.744.203.67.263 1.438.178 2.284l-.013.105v.283l.22.125a3.58 3.58 0 0 1 .884.665c.49.56.801 1.266.912 2.084.114.849.037 1.842-.227 2.88a7.937 7.937 0 0 1-.965 2.268 5.28 5.28 0 0 1-1.541 1.574c-.61.405-1.33.71-2.138.9-.787.186-1.671.28-2.628.28h-.623a1.87 1.87 0 0 0-1.847 1.575l-.047.257-.79 5.004-.034.183a.17.17 0 0 1-.167.148H7.076z"/></svg>
                                  Pay ${amt.toFixed(2)}
                                </a>
                              ) : (
                                <button
                                  onClick={() => setPaypalModal(true)}
                                  style={{
                                    padding: '8px 16px', borderRadius: '8px', fontSize: '13px', fontWeight: 600,
                                    background: '#fef3c7', color: '#92400e', border: '1px solid #fcd34d',
                                    cursor: 'pointer', transition: 'background 0.15s',
                                  }}
                                  onMouseOver={(e) => (e.currentTarget.style.background = '#fde68a')}
                                  onMouseOut={(e) => (e.currentTarget.style.background = '#fef3c7')}
                                >
                                  + Add PayPal
                                </button>
                              )}
                              <Button size="slim" variant="secondary" onClick={() => setConfirmAction({ order: p, action: 'send-payment' })}>
                                {p.pay_status === 'disputed' ? 'Retry — Mark Paid' : 'Mark as Paid'}
                              </Button>
                            </>
                          )}

                          {/* Reseller: waiting */}
                          {!isSupplier && p.pay_status === 'payment_sent' && (
                            <Badge tone="info">Waiting for supplier to confirm</Badge>
                          )}

                          {/* SUPPLIER actions */}
                          {needsSupplierAction && (
                            <>
                              <Button variant="primary" onClick={() => setConfirmAction({ order: p, action: 'confirm-received' })}>
                                Confirm Received
                              </Button>
                              <Button tone="critical" onClick={() => setConfirmAction({ order: p, action: 'dispute-payment' })}>
                                Not Received
                              </Button>
                            </>
                          )}

                          {/* Supplier: waiting */}
                          {isSupplier && (p.pay_status === 'pending' || p.pay_status === 'unpaid') && (
                            <Badge tone="attention">Awaiting reseller payment</Badge>
                          )}

                          {/* Both: paid */}
                          {p.pay_status === 'paid' && (
                            <span style={{
                              display: 'inline-flex', alignItems: 'center', gap: '4px',
                              color: '#166534', fontWeight: 600, fontSize: '14px',
                            }}>
                              <svg width="18" height="18" viewBox="0 0 24 24" fill="#166534"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>
                              Payment Complete
                            </span>
                          )}

                          {/* Both: disputed */}
                          {isSupplier && p.pay_status === 'disputed' && (
                            <Badge tone="critical">You disputed this payment</Badge>
                          )}
                        </div>
                      </div>
                    </div>
                  </Card>
                );
              })}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading="No payouts yet" image="">
                <p>{isSupplier ? 'Payments will appear here when resellers order your products.' : 'Payments will appear here when you order from suppliers.'}</p>
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

      <ConfirmDialog
        open={confirmAction !== null}
        title={confirmAction?.action === 'send-payment' ? 'Confirm Payment Sent' : confirmAction?.action === 'confirm-received' ? 'Confirm Payment Received' : 'Dispute Payment'}
        message={getConfirmMessage()}
        confirmLabel={confirmAction?.action === 'send-payment' ? 'Yes, I Paid' : confirmAction?.action === 'confirm-received' ? 'Yes, Received' : 'Not Received'}
        destructive={confirmAction?.action === 'dispute-payment'}
        loading={acting}
        onConfirm={handleAction}
        onCancel={() => setConfirmAction(null)}
      />

      {paypalModal && (
        <Modal
          open
          onClose={() => setPaypalModal(false)}
          title="Add PayPal Email"
          primaryAction={{ content: 'Save', onAction: handleSavePaypal, loading: savingPaypal, disabled: !paypalEmail.trim() }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setPaypalModal(false) }]}
        >
          <Modal.Section>
            <FormLayout>
              <TextField
                label="PayPal Email"
                type="email"
                value={paypalEmail}
                onChange={setPaypalEmail}
                autoComplete="email"
                placeholder="your@email.com"
                helpText={isSupplier
                  ? 'Resellers will use this email to send you payments via PayPal.'
                  : 'This email will be shared with suppliers for payment verification.'}
              />
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
