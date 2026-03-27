import {
  Page,
  Layout,
  Card,
  BlockStack,
  Text,
  InlineGrid,
  Spinner,
  Banner,
  InlineStack,
  Icon,
  Divider,
  Badge,
  DataTable,
} from '@shopify/polaris';
import {
  OrderIcon,
  CashDollarIcon,
  ProductIcon,
  ChartVerticalFilledIcon,
} from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';

interface AnalyticsData {
  role: string;
  active_listings?: number;
  imported_products?: number;
  order_count?: number;
  pending_order_count?: number;
  recent_orders?: Array<{
    id: string;
    reseller_order_number: string;
    status: string;
    total_wholesale_amount: number;
    currency: string;
    created_at: string;
  }>;
}

export default function Analytics() {
  const { data, loading, error } = useApi<AnalyticsData>('/dashboard');

  if (loading) {
    return (
      <Page title="Analytics">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  if (error) {
    return (
      <Page title="Analytics">
        <Banner tone="critical">{error}</Banner>
      </Page>
    );
  }

  const orders = data?.recent_orders || [];
  const totalRevenue = orders.reduce((sum, o) => sum + o.total_wholesale_amount, 0);
  const avgOrderValue = orders.length > 0 ? totalRevenue / orders.length : 0;
  const fulfilledOrders = orders.filter(o => o.status === 'fulfilled').length;
  const pendingOrders = orders.filter(o => o.status === 'pending').length;
  const fulfillmentRate = orders.length > 0 ? ((fulfilledOrders / orders.length) * 100) : 0;

  const isSupplier = data?.role === 'supplier';

  // Group orders by status for breakdown
  const statusCounts: Record<string, number> = {};
  orders.forEach(o => {
    statusCounts[o.status] = (statusCounts[o.status] || 0) + 1;
  });

  // Simple revenue by day (last 7 days)
  const last7Days: Record<string, number> = {};
  for (let i = 6; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    last7Days[d.toLocaleDateString()] = 0;
  }
  orders.forEach(o => {
    const day = new Date(o.created_at).toLocaleDateString();
    if (day in last7Days) {
      last7Days[day] += o.total_wholesale_amount;
    }
  });

  const StatCard = ({ icon, label, value, color }: { icon: typeof OrderIcon; label: string; value: string | number; color: string }) => (
    <Card>
      <BlockStack gap="300">
        <InlineStack gap="300" blockAlign="center">
          <div style={{ background: color, borderRadius: '10px', padding: '10px', display: 'flex' }}>
            <Icon source={icon} />
          </div>
          <Text as="p" variant="bodySm" tone="subdued">{label}</Text>
        </InlineStack>
        <Text as="p" variant="headingXl">{value}</Text>
      </BlockStack>
    </Card>
  );

  return (
    <Page title="Analytics">
      <Layout>
        <Layout.Section>
          <InlineGrid columns={{ xs: 2, md: 4 }} gap="400">
            <StatCard icon={CashDollarIcon} label="Total Revenue" value={`$${totalRevenue.toFixed(2)}`} color="#e3f1df" />
            <StatCard icon={OrderIcon} label="Total Orders" value={data?.order_count ?? 0} color="#e0f0ff" />
            <StatCard icon={ChartVerticalFilledIcon} label="Avg Order Value" value={`$${avgOrderValue.toFixed(2)}`} color="#f0e6ff" />
            <StatCard icon={ProductIcon} label={isSupplier ? 'Active Listings' : 'Imports'} value={isSupplier ? (data?.active_listings ?? 0) : (data?.imported_products ?? 0)} color="#fef3cd" />
          </InlineGrid>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Order Status Breakdown</Text>
              <Divider />
              <BlockStack gap="200">
                {Object.entries(statusCounts).map(([status, count]) => {
                  const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
                    pending: 'attention', accepted: 'info', fulfilled: 'success',
                    rejected: 'critical', processing: 'info',
                  };
                  return (
                    <InlineStack key={status} align="space-between" blockAlign="center">
                      <Badge tone={toneMap[status]}>{status}</Badge>
                      <Text as="span" variant="headingSm">{count}</Text>
                    </InlineStack>
                  );
                })}
                {Object.keys(statusCounts).length === 0 && (
                  <Text as="p" tone="subdued">No order data yet</Text>
                )}
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Performance</Text>
              <Divider />
              <BlockStack gap="300">
                <InlineStack align="space-between">
                  <Text as="span" variant="bodyMd">Fulfillment Rate</Text>
                  <Text as="span" variant="headingSm" tone={fulfillmentRate > 80 ? 'success' : 'caution'}>
                    {fulfillmentRate.toFixed(1)}%
                  </Text>
                </InlineStack>
                <InlineStack align="space-between">
                  <Text as="span" variant="bodyMd">Pending Orders</Text>
                  <Text as="span" variant="headingSm">{pendingOrders}</Text>
                </InlineStack>
                <InlineStack align="space-between">
                  <Text as="span" variant="bodyMd">Fulfilled Orders</Text>
                  <Text as="span" variant="headingSm" tone="success">{fulfilledOrders}</Text>
                </InlineStack>
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Revenue (Last 7 Days)</Text>
              <Divider />
              <DataTable
                columnContentTypes={['text', 'numeric']}
                headings={['Date', 'Revenue']}
                rows={Object.entries(last7Days).map(([date, amount]) => [
                  date,
                  `$${amount.toFixed(2)}`,
                ])}
              />
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
