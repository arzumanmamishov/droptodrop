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
}

interface PayoutsResponse {
  payouts: PayoutOrder[];
  total: number;
  grand_total: number;
  grand_paid: number;
  grand_balance: number;
}

interface Props {
  role: string;
}

export default function Payouts({ role }: Props) {
  const [page, setPage] = useState(0);
  const limit = 20;
  const { data, loading, refetch } = useApi<PayoutsResponse>(`/payouts?limit=${limit}&offset=${page * limit}`);
  const [confirmPay, setConfirmPay] = useState<PayoutOrder | null>(null);
  const [paying, setPaying] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);

  const isSupplier = role === 'supplier';

  const handleMarkPaid = useCallback(async () => {
    if (!confirmPay) return;
    setPaying(true);
    try {
      await api.post(`/payouts/mark-paid/${confirmPay.id}`);
      setSuccess(`Marked order ${confirmPay.order_number || confirmPay.id.slice(0, 8)} as paid`);
      setConfirmPay(null);
      refetch();
    } catch { /* */ }
    finally { setPaying(false); }
  }, [confirmPay, refetch]);

  if (loading) {
    return <Page title="Payouts"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const payouts = data?.payouts || [];
  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page title="Payouts" subtitle={isSupplier ? 'Money owed to you by resellers' : 'Money you owe to suppliers'}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(null)}>{success}</Banner></Layout.Section>}

        <Layout.Section>
          <InlineStack gap="300">
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">${(data?.grand_total || 0).toFixed(2)}</div>
              <div className="stat-card-label">{isSupplier ? 'Total Earned' : 'Total Owed'}</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">${(data?.grand_paid || 0).toFixed(2)}</div>
              <div className="stat-card-label">Paid</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value" style={{ color: (data?.grand_balance || 0) > 0 ? '#b91c1c' : '#2d6a4f' }}>
                ${(data?.grand_balance || 0).toFixed(2)}
              </div>
              <div className="stat-card-label">Outstanding</div>
            </div>
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
                        <BlockStack gap="100" >
                          <InlineStack gap="200" blockAlign="center">
                            <Text as="span" variant="bodyMd" fontWeight="semibold">
                              {p.order_number || p.id.slice(0, 8)}
                            </Text>
                            <Badge tone={p.status === 'fulfilled' ? 'success' : p.status === 'pending' ? 'attention' : 'info'}>
                              {p.status}
                            </Badge>
                            {p.pay_status === 'paid' ? (
                              <Badge tone="success">Paid</Badge>
                            ) : p.pay_status === 'pending' ? (
                              <Badge tone="attention">Payment Pending</Badge>
                            ) : (
                              <Badge>Unpaid</Badge>
                            )}
                          </InlineStack>
                          <Text as="p" variant="bodySm" tone="subdued">
                            {p.products || 'No products'} — {p.domain}
                          </Text>
                          <Text as="p" variant="bodySm" tone="subdued">
                            {new Date(p.created_at).toLocaleDateString()}
                          </Text>
                        </BlockStack>

                        <InlineStack gap="400" blockAlign="center" wrap={false}>
                          <BlockStack gap="050" align="end">
                            <Text as="span" variant="headingSm">${p.wholesale.toFixed(2)}</Text>
                            <Text as="span" variant="bodySm" tone="subdued">wholesale</Text>
                          </BlockStack>
                          {p.platform_fee > 0 && (
                            <BlockStack gap="050" align="end">
                              <Text as="span" variant="bodySm">-${p.platform_fee.toFixed(2)}</Text>
                              <Text as="span" variant="bodySm" tone="subdued">fee</Text>
                            </BlockStack>
                          )}
                          {p.supplier_payout > 0 && (
                            <BlockStack gap="050" align="end">
                              <Text as="span" variant="headingSm" fontWeight="bold">${p.supplier_payout.toFixed(2)}</Text>
                              <Text as="span" variant="bodySm" tone="subdued">payout</Text>
                            </BlockStack>
                          )}
                          {p.pay_status !== 'paid' && (
                            <Button size="slim" variant="primary" onClick={() => setConfirmPay(p)}>
                              Mark Paid
                            </Button>
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
                <p>{isSupplier ? 'When resellers order your products, payments will be tracked here.' : 'When you order from suppliers, payments will be tracked here.'}</p>
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
        open={confirmPay !== null}
        title="Confirm Payment"
        message={confirmPay ? `Mark payment for order ${confirmPay.order_number || confirmPay.id.slice(0, 8)} ($${confirmPay.wholesale.toFixed(2)}) as paid?` : ''}
        confirmLabel="Confirm Paid"
        destructive={false}
        loading={paying}
        onConfirm={handleMarkPaid}
        onCancel={() => setConfirmPay(null)}
      />
    </Page>
  );
}
