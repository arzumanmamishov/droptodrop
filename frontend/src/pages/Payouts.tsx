import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Spinner,
  Banner, InlineStack, Divider, EmptyState, Button,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import ConfirmDialog from '../components/ConfirmDialog';

interface Payout {
  supplier_shop_id: string;
  reseller_shop_id: string;
  domain: string;
  order_count: number;
  total_owed: number;
  total_paid: number;
  balance: number;
}

interface PayoutsResponse {
  payouts: Payout[];
  grand_total: number;
  grand_paid: number;
  grand_balance: number;
}

interface Props {
  role: string;
}

export default function Payouts({ role }: Props) {
  const { data, loading, refetch } = useApi<PayoutsResponse>('/payouts');
  const [confirmPay, setConfirmPay] = useState<Payout | null>(null);
  const [paying, setPaying] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);

  const isSupplier = role === 'supplier';

  const handleMarkPaid = useCallback(async () => {
    if (!confirmPay) return;
    setPaying(true);
    try {
      await api.post('/payouts/mark-paid', {
        supplier_shop_id: confirmPay.supplier_shop_id,
        reseller_shop_id: confirmPay.reseller_shop_id,
        amount: confirmPay.balance,
      });
      setSuccess(`Marked $${confirmPay.balance.toFixed(2)} as paid for ${confirmPay.domain}`);
      setConfirmPay(null);
      refetch();
    } catch { /* */ }
    finally { setPaying(false); }
  }, [confirmPay, refetch]);

  if (loading) {
    return <Page title="Payouts"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const payouts = data?.payouts || [];

  return (
    <Page title="Payouts" subtitle={isSupplier ? 'Money owed to you by resellers' : 'Money you owe to suppliers'}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(null)}>{success}</Banner></Layout.Section>}

        {/* Summary cards */}
        <Layout.Section>
          <InlineStack gap="300">
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">${(data?.grand_total || 0).toFixed(2)}</div>
              <div className="stat-card-label">{isSupplier ? 'Total Earned' : 'Total Owed'}</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">${(data?.grand_paid || 0).toFixed(2)}</div>
              <div className="stat-card-label">Total Paid</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value" style={{ color: (data?.grand_balance || 0) > 0 ? '#b91c1c' : '#2d6a4f' }}>
                ${(data?.grand_balance || 0).toFixed(2)}
              </div>
              <div className="stat-card-label">Outstanding Balance</div>
            </div>
          </InlineStack>
        </Layout.Section>

        {/* Payout list */}
        <Layout.Section>
          {payouts.length > 0 ? (
            <Card>
              <BlockStack gap="0">
                {payouts.map((payout, i) => (
                  <div key={payout.domain}>
                    <div style={{ padding: '16px' }}>
                      <InlineStack align="space-between" blockAlign="center" wrap={false}>
                        <BlockStack gap="100">
                          <Text as="span" variant="bodyMd" fontWeight="semibold">{payout.domain}</Text>
                          <InlineStack gap="200">
                            <Text as="span" variant="bodySm" tone="subdued">{payout.order_count} orders</Text>
                            <Badge tone="success">{`$${payout.total_paid.toFixed(2)} paid`}</Badge>
                            {payout.balance > 0 && (
                              <Badge tone="attention">{`$${payout.balance.toFixed(2)} pending`}</Badge>
                            )}
                          </InlineStack>
                        </BlockStack>

                        <InlineStack gap="300" blockAlign="center" wrap={false}>
                          <BlockStack gap="050" align="end">
                            <Text as="span" variant="headingMd" fontWeight="bold">${payout.total_owed.toFixed(2)}</Text>
                            <Text as="span" variant="bodySm" tone="subdued">total</Text>
                          </BlockStack>
                          {payout.balance > 0 && (
                            <Button
                              size="slim"
                              variant="primary"
                              onClick={() => setConfirmPay(payout)}
                            >
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
                <p>{isSupplier ? 'When resellers order your products, payment tracking will appear here.' : 'When you order from suppliers, payment tracking will appear here.'}</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>

      <ConfirmDialog
        open={confirmPay !== null}
        title="Confirm Payment"
        message={confirmPay ? `Mark $${confirmPay.balance.toFixed(2)} as paid to ${confirmPay.domain}? This records that the payment has been made outside of DropToDrop (e.g. bank transfer, PayPal).` : ''}
        confirmLabel="Confirm Paid"
        destructive={false}
        loading={paying}
        onConfirm={handleMarkPaid}
        onCancel={() => setConfirmPay(null)}
      />
    </Page>
  );
}
