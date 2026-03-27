import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Button, Spinner,
  Banner, InlineStack, InlineGrid, Divider, Icon, Box,
  ProgressBar,
} from '@shopify/polaris';
import { CashDollarIcon, CheckIcon, PackageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';

interface Plan {
  id: string;
  name: string;
  price_monthly: number;
  currency: string;
  max_products: number;
  max_orders_monthly: number;
  max_suppliers: number;
  app_fee_percent: number;
  trial_days: number;
}

interface BillingStatus {
  has_subscription: boolean;
  subscription?: {
    id: string;
    plan_id: string;
    plan_name: string;
    status: string;
    trial_ends_at?: string;
    current_period_end?: string;
  };
  plan?: Plan;
  usage?: {
    order_count: number;
    product_count: number;
    total_fees: number;
    month: string;
  };
  limits?: {
    max_products: number;
    max_orders_monthly: number;
    max_suppliers: number;
    products_used: number;
    orders_this_month: number;
    is_over_limit: boolean;
  };
}

interface PlansResponse {
  plans: Plan[];
}

export default function Billing() {
  const { data: status, loading, error, refetch } = useApi<BillingStatus>('/billing');
  const { data: plansData } = useApi<PlansResponse>('/billing/plans');
  const [subscribing, setSubscribing] = useState<string | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  const handleSubscribe = useCallback(async (planId: string) => {
    setSubscribing(planId);
    setActionError(null);
    try {
      await api.post('/billing/subscribe', { plan_id: planId });
      setActionSuccess(`Subscribed to ${planId} plan!`);
      refetch();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to subscribe');
    } finally {
      setSubscribing(null);
    }
  }, [refetch]);

  const handleCancel = useCallback(async () => {
    setCancelling(true);
    setActionError(null);
    try {
      await api.post('/billing/cancel');
      setActionSuccess('Subscription cancelled.');
      refetch();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed to cancel');
    } finally {
      setCancelling(false);
    }
  }, [refetch]);

  if (loading) {
    return <Page title="Billing"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const plans = plansData?.plans || [];
  const currentPlanId = status?.subscription?.plan_id;

  return (
    <Page title="Billing & Subscription">
      <Layout>
        {actionError && <Layout.Section><Banner tone="critical" onDismiss={() => setActionError(null)}>{actionError}</Banner></Layout.Section>}
        {actionSuccess && <Layout.Section><Banner tone="success" onDismiss={() => setActionSuccess(null)}>{actionSuccess}</Banner></Layout.Section>}
        {error && <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>}

        {status?.has_subscription && status.subscription && (
          <Layout.Section>
            <Card>
              <BlockStack gap="400">
                <InlineStack align="space-between" blockAlign="center">
                  <InlineStack gap="300" blockAlign="center">
                    <div style={{ background: '#e3f1df', borderRadius: '10px', padding: '10px', display: 'flex' }}>
                      <Icon source={CashDollarIcon} />
                    </div>
                    <BlockStack gap="100">
                      <Text as="h2" variant="headingLg">Current Plan: {status.subscription.plan_name}</Text>
                      <Badge tone={status.subscription.status === 'active' ? 'success' : 'attention'}>{status.subscription.status}</Badge>
                    </BlockStack>
                  </InlineStack>
                  <Button tone="critical" onClick={handleCancel} loading={cancelling}>Cancel Plan</Button>
                </InlineStack>
                <Divider />
                {status.subscription.trial_ends_at && new Date(status.subscription.trial_ends_at) > new Date() && (
                  <Banner tone="info">Trial ends {new Date(status.subscription.trial_ends_at).toLocaleDateString()}</Banner>
                )}
                {status.subscription.current_period_end && (
                  <Text as="p" variant="bodySm" tone="subdued">Next billing: {new Date(status.subscription.current_period_end).toLocaleDateString()}</Text>
                )}
              </BlockStack>
            </Card>
          </Layout.Section>
        )}

        {status?.limits && (
          <Layout.Section>
            <InlineGrid columns={{ xs: 1, md: 3 }} gap="400">
              <Card>
                <BlockStack gap="200">
                  <Text as="p" variant="bodySm" tone="subdued">Products</Text>
                  <Text as="p" variant="headingLg">{status.limits.products_used} / {status.limits.max_products === -1 ? 'Unlimited' : status.limits.max_products}</Text>
                  {status.limits.max_products > 0 && (
                    <ProgressBar progress={Math.min((status.limits.products_used / status.limits.max_products) * 100, 100)} tone={status.limits.products_used >= status.limits.max_products ? 'critical' : 'primary'} />
                  )}
                </BlockStack>
              </Card>
              <Card>
                <BlockStack gap="200">
                  <Text as="p" variant="bodySm" tone="subdued">Orders This Month</Text>
                  <Text as="p" variant="headingLg">{status.limits.orders_this_month} / {status.limits.max_orders_monthly === -1 ? 'Unlimited' : status.limits.max_orders_monthly}</Text>
                  {status.limits.max_orders_monthly > 0 && (
                    <ProgressBar progress={Math.min((status.limits.orders_this_month / status.limits.max_orders_monthly) * 100, 100)} tone={status.limits.orders_this_month >= status.limits.max_orders_monthly ? 'critical' : 'primary'} />
                  )}
                </BlockStack>
              </Card>
              <Card>
                <BlockStack gap="200">
                  <Text as="p" variant="bodySm" tone="subdued">Platform Fees ({status.usage?.month})</Text>
                  <Text as="p" variant="headingLg">${(status.usage?.total_fees || 0).toFixed(2)}</Text>
                </BlockStack>
              </Card>
            </InlineGrid>
          </Layout.Section>
        )}

        <Layout.Section>
          <Text as="h2" variant="headingLg">Choose a Plan</Text>
        </Layout.Section>

        <Layout.Section>
          <InlineGrid columns={{ xs: 1, md: 3 }} gap="400">
            {plans.map((plan) => {
              const isCurrent = plan.id === currentPlanId;
              return (
                <Card key={plan.id}>
                  <BlockStack gap="400">
                    <BlockStack gap="100">
                      <InlineStack align="space-between">
                        <Text as="h3" variant="headingMd">{plan.name}</Text>
                        {isCurrent && <Badge tone="success">Current</Badge>}
                      </InlineStack>
                      <InlineStack gap="100" blockAlign="baseline">
                        <Text as="span" variant="heading2xl">{plan.currency === 'EUR' ? '€' : '$'}{plan.price_monthly}</Text>
                        <Text as="span" variant="bodySm" tone="subdued">/month</Text>
                      </InlineStack>
                    </BlockStack>
                    <Divider />
                    <BlockStack gap="200">
                      <InlineStack gap="200" blockAlign="center">
                        <Icon source={CheckIcon} tone="success" />
                        <Text as="span" variant="bodySm">{plan.max_products === -1 ? 'Unlimited' : plan.max_products} products</Text>
                      </InlineStack>
                      <InlineStack gap="200" blockAlign="center">
                        <Icon source={CheckIcon} tone="success" />
                        <Text as="span" variant="bodySm">{plan.max_orders_monthly === -1 ? 'Unlimited' : plan.max_orders_monthly} orders/month</Text>
                      </InlineStack>
                      <InlineStack gap="200" blockAlign="center">
                        <Icon source={CheckIcon} tone="success" />
                        <Text as="span" variant="bodySm">{plan.max_suppliers === -1 ? 'Unlimited' : plan.max_suppliers} suppliers</Text>
                      </InlineStack>
                      <InlineStack gap="200" blockAlign="center">
                        <Icon source={PackageIcon} tone="subdued" />
                        <Text as="span" variant="bodySm">{plan.app_fee_percent}% platform fee</Text>
                      </InlineStack>
                      <InlineStack gap="200" blockAlign="center">
                        <Icon source={CheckIcon} tone="success" />
                        <Text as="span" variant="bodySm">{plan.trial_days}-day free trial</Text>
                      </InlineStack>
                    </BlockStack>
                    <Button
                      variant="primary"
                      fullWidth
                      disabled={isCurrent}
                      loading={subscribing === plan.id}
                      onClick={() => handleSubscribe(plan.id)}
                    >
                      {isCurrent ? 'Current Plan' : `Choose ${plan.name}`}
                    </Button>
                  </BlockStack>
                </Card>
              );
            })}
          </InlineGrid>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
