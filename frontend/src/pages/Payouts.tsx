import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Spinner,
  Banner, InlineStack, Divider, EmptyState, Button,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
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
}

interface Props { role: string; }

export default function Payouts({ role }: Props) {
  const [page, setPage] = useState(0);
  const limit = 20;
  const { data, loading, refetch } = useApi<PayoutsResponse>(`/payouts?limit=${limit}&offset=${page * limit}`);
  const [confirmAction, setConfirmAction] = useState<{ order: PayoutOrder; action: string } | null>(null);
  const [acting, setActing] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);

  const isSupplier = role === 'supplier';

  const handleAction = useCallback(async () => {
    if (!confirmAction) return;
    setActing(true);
    try {
      const { order, action } = confirmAction;
      await api.post(`/payouts/${action}/${order.id}`);
      const messages: Record<string, string> = {
        'send-payment': `Payment sent. Waiting for supplier confirmation.`,
        'confirm-received': `Payment confirmed.`,
        'dispute-payment': `Payment disputed. Reseller notified.`,
      };
      setSuccess(messages[action] || 'Done');
      setConfirmAction(null);
      refetch();
    } catch { /* */ }
    finally { setActing(false); }
  }, [confirmAction, refetch]);

  if (loading) {
    return <Page title="Payouts"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const payouts = data?.payouts || [];
  const totalPages = Math.ceil((data?.total || 0) / limit);

  const payStatusBadge = (status: string) => {
    const map: Record<string, { tone: 'success' | 'attention' | 'critical' | 'info'; label: string }> = {
      pending: { tone: 'attention', label: 'Awaiting Payment' },
      payment_sent: { tone: 'info', label: 'Sent — Awaiting Confirmation' },
      paid: { tone: 'success', label: 'Confirmed Paid' },
      disputed: { tone: 'critical', label: 'Disputed' },
      unpaid: { tone: 'attention', label: 'Awaiting Payment' },
    };
    const s = map[status] || map['unpaid'];
    return <Badge tone={s.tone}>{s.label}</Badge>;
  };

  const getConfirmMessage = () => {
    if (!confirmAction) return '';
    const { order, action } = confirmAction;
    const payAmount = order.supplier_payout > 0 ? order.supplier_payout : order.wholesale;
    if (action === 'send-payment') return `Confirm that you have sent $${payAmount.toFixed(2)} to the supplier for order ${order.order_number || order.id.slice(0, 8)}?${order.platform_fee > 0 ? ` (Platform fee of $${order.platform_fee.toFixed(2)} is collected separately via your billing plan.)` : ''} The supplier will be asked to confirm receipt.`;
    if (action === 'confirm-received') return `Confirm you received $${payAmount.toFixed(2)} for order ${order.order_number || order.id.slice(0, 8)}?`;
    if (action === 'dispute-payment') return `Report that you have NOT received payment for order ${order.order_number || order.id.slice(0, 8)}? The reseller will be notified.`;
    return '';
  };

  return (
    <Page title="Payouts" subtitle={isSupplier ? 'Money owed to you' : 'Money you owe'}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(null)}>{success}</Banner></Layout.Section>}

        <Layout.Section>
          <InlineStack gap="300">
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">${(data?.grand_total || 0).toFixed(2)}</div>
              <div className="stat-card-label">{isSupplier ? 'Total Earned' : 'Total Owed'}</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value" style={{ color: '#2d6a4f' }}>${(data?.grand_paid || 0).toFixed(2)}</div>
              <div className="stat-card-label">Confirmed Paid</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value" style={{ color: (data?.grand_balance || 0) > 0 ? '#b91c1c' : '#2d6a4f' }}>
                ${(data?.grand_balance || 0).toFixed(2)}
              </div>
              <div className="stat-card-label">Outstanding</div>
            </div>
            {!isSupplier && (data?.grand_fees || 0) > 0 && (
              <div className="stat-card" style={{ flex: 1 }}>
                <div className="stat-card-value" style={{ color: '#6b7280' }}>
                  ${(data?.grand_fees || 0).toFixed(2)}
                </div>
                <div className="stat-card-label">Platform Fees</div>
              </div>
            )}
          </InlineStack>
        </Layout.Section>

        <Layout.Section>
          {payouts.length > 0 ? (
            <Card>
              <BlockStack gap="0">
                {payouts.map((p, i) => (
                  <div key={p.id}>
                    <div style={{ padding: '14px 16px' }}>
                      <InlineStack align="space-between" blockAlign="center" wrap={false}>
                        <BlockStack gap="100">
                          <InlineStack gap="200" blockAlign="center">
                            <Text as="span" variant="bodyMd" fontWeight="semibold">
                              {p.order_number || p.id.slice(0, 8)}
                            </Text>
                            {payStatusBadge(p.pay_status)}
                          </InlineStack>
                          <Text as="p" variant="bodySm" tone="subdued">
                            {p.products || 'Products'} — {p.domain}
                          </Text>
                          <Text as="p" variant="bodySm" tone="subdued">
                            {new Date(p.created_at).toLocaleDateString()}
                          </Text>
                        </BlockStack>

                        <InlineStack gap="300" blockAlign="center" wrap={false}>
                          <BlockStack gap="050" align="end">
                            <Text as="span" variant="headingSm" fontWeight="bold">
                              ${(p.supplier_payout > 0 ? p.supplier_payout : p.wholesale).toFixed(2)}
                            </Text>
                            <Text as="span" variant="bodySm" tone="subdued">
                              {isSupplier ? 'you receive' : 'to supplier'}
                            </Text>
                            {p.platform_fee > 0 && (
                              <Text as="span" variant="bodySm" tone="subdued">
                                fee: ${p.platform_fee.toFixed(2)}
                              </Text>
                            )}
                          </BlockStack>

                          {/* RESELLER: Pay button */}
                          {!isSupplier && (p.pay_status === 'pending' || p.pay_status === 'unpaid') && (
                            <BlockStack gap="200" align="end">
                              {p.supplier_paypal ? (
                                <Button size="slim" variant="primary" url={`https://paypal.me/${p.supplier_paypal}/${(p.supplier_payout > 0 ? p.supplier_payout : p.wholesale).toFixed(2)}${p.currency || 'USD'}`} external>
                                  Pay ${(p.supplier_payout > 0 ? p.supplier_payout : p.wholesale).toFixed(2)} via PayPal
                                </Button>
                              ) : (
                                <Text as="span" variant="bodySm" tone="caution">Supplier has no PayPal</Text>
                              )}
                              <Button size="slim" variant="secondary" onClick={() => setConfirmAction({ order: p, action: 'send-payment' })}>
                                Mark as Paid
                              </Button>
                            </BlockStack>
                          )}
                          {!isSupplier && p.pay_status === 'payment_sent' && (
                            <Text as="span" variant="bodySm" tone="subdued">Waiting confirmation</Text>
                          )}
                          {!isSupplier && p.pay_status === 'disputed' && (
                            <BlockStack gap="200" align="end">
                              {p.supplier_paypal ? (
                                <Button size="slim" variant="primary" url={`https://paypal.me/${p.supplier_paypal}/${(p.supplier_payout > 0 ? p.supplier_payout : p.wholesale).toFixed(2)}${p.currency || 'USD'}`} external>
                                  Pay ${(p.supplier_payout > 0 ? p.supplier_payout : p.wholesale).toFixed(2)} via PayPal
                                </Button>
                              ) : (
                                <Text as="span" variant="bodySm" tone="caution">Supplier has no PayPal</Text>
                              )}
                              <Button size="slim" variant="secondary" onClick={() => setConfirmAction({ order: p, action: 'send-payment' })}>
                                Mark as Paid
                              </Button>
                            </BlockStack>
                          )}

                          {/* SUPPLIER: Confirm/Dispute */}
                          {isSupplier && p.pay_status === 'payment_sent' && (
                            <InlineStack gap="200">
                              <Button size="slim" variant="primary" onClick={() => setConfirmAction({ order: p, action: 'confirm-received' })}>
                                Confirm
                              </Button>
                              <Button size="slim" tone="critical" onClick={() => setConfirmAction({ order: p, action: 'dispute-payment' })}>
                                Not Received
                              </Button>
                            </InlineStack>
                          )}
                          {isSupplier && (p.pay_status === 'pending' || p.pay_status === 'unpaid') && (
                            <Text as="span" variant="bodySm" tone="subdued">Awaiting reseller</Text>
                          )}

                          {/* Both: Paid confirmation */}
                          {p.pay_status === 'paid' && (
                            <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                          )}
                        </InlineStack>
                      </InlineStack>
                    </div>
                    {i < payouts.length - 1 && <Divider />}
                  </div>
                ))}
              </BlockStack>
            </Card>
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
        title={confirmAction?.action === 'send-payment' ? 'Send Payment' : confirmAction?.action === 'confirm-received' ? 'Confirm Payment Received' : 'Dispute Payment'}
        message={getConfirmMessage()}
        confirmLabel={confirmAction?.action === 'send-payment' ? 'I Have Paid' : confirmAction?.action === 'confirm-received' ? 'Yes, Received' : 'Not Received'}
        destructive={confirmAction?.action === 'dispute-payment'}
        loading={acting}
        onConfirm={handleAction}
        onCancel={() => setConfirmAction(null)}
      />
    </Page>
  );
}
