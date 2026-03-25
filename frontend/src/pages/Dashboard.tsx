import {
  Page,
  Layout,
  Card,
  Text,
  BlockStack,
  InlineGrid,
  Banner,
  Spinner,
  DataTable,
  Badge,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';

interface DashboardData {
  role: string;
  active_listings?: number;
  imported_products?: number;
  order_count?: number;
  recent_orders?: Array<{
    id: string;
    reseller_order_number: string;
    status: string;
    total_wholesale_amount: number;
    currency: string;
    created_at: string;
  }>;
}

export default function Dashboard() {
  const { data, loading, error } = useApi<DashboardData>('/dashboard');

  if (loading) {
    return (
      <Page title="Dashboard">
        <Layout>
          <Layout.Section>
            <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
              <Spinner size="large" />
            </div>
          </Layout.Section>
        </Layout>
      </Page>
    );
  }

  if (error) {
    return (
      <Page title="Dashboard">
        <Layout>
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        </Layout>
      </Page>
    );
  }

  const isSupplier = data?.role === 'supplier';

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      pending: 'attention',
      accepted: 'info',
      fulfilled: 'success',
      rejected: 'critical',
      processing: 'info',
    };
    return <Badge tone={toneMap[status] || undefined}>{status}</Badge>;
  };

  return (
    <Page title="Dashboard">
      <Layout>
        <Layout.Section>
          <InlineGrid columns={3} gap="400">
            <Card>
              <BlockStack gap="200">
                <Text as="h3" variant="headingSm">
                  {isSupplier ? 'Active Listings' : 'Imported Products'}
                </Text>
                <Text as="p" variant="headingXl">
                  {isSupplier ? (data?.active_listings ?? 0) : (data?.imported_products ?? 0)}
                </Text>
              </BlockStack>
            </Card>
            <Card>
              <BlockStack gap="200">
                <Text as="h3" variant="headingSm">Total Orders</Text>
                <Text as="p" variant="headingXl">{data?.order_count ?? 0}</Text>
              </BlockStack>
            </Card>
            <Card>
              <BlockStack gap="200">
                <Text as="h3" variant="headingSm">Role</Text>
                <Text as="p" variant="headingXl" tone="magic">
                  {data?.role === 'supplier' ? 'Supplier' : 'Reseller'}
                </Text>
              </BlockStack>
            </Card>
          </InlineGrid>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Recent Orders</Text>
              {data?.recent_orders && data.recent_orders.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'text', 'numeric', 'text']}
                  headings={['Order', 'Status', 'Amount', 'Currency', 'Date']}
                  rows={data.recent_orders.map((order) => [
                    order.reseller_order_number || order.id.slice(0, 8),
                    statusBadge(order.status),
                    `$${order.total_wholesale_amount.toFixed(2)}`,
                    order.currency,
                    new Date(order.created_at).toLocaleDateString(),
                  ])}
                />
              ) : (
                <Text as="p" tone="subdued">No orders yet. {isSupplier ? 'Publish listings to start receiving orders.' : 'Import products and start selling.'}</Text>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
