import { useNavigate } from 'react-router-dom';
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
  Button,
  InlineStack,
  Icon,
  Box,
  Divider,
} from '@shopify/polaris';
import {
  ProductIcon,
  OrderIcon,
  CashDollarIcon,
  ClockIcon,
  ImportIcon,
} from '@shopify/polaris-icons';
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
  const navigate = useNavigate();
  const { data, loading, error } = useApi<DashboardData>('/dashboard');

  if (loading) {
    return (
      <Page title="Dashboard">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  if (error) {
    return (
      <Page title="Dashboard">
        <Banner tone="critical">{error}</Banner>
      </Page>
    );
  }

  const isSupplier = data?.role === 'supplier';
  const pendingOrders = (data?.recent_orders || []).filter(o => o.status === 'pending').length;
  const totalRevenue = (data?.recent_orders || []).reduce((sum, o) => sum + o.total_wholesale_amount, 0);

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      pending: 'attention', accepted: 'info', fulfilled: 'success',
      rejected: 'critical', processing: 'info',
    };
    return <Badge tone={toneMap[status]}>{status}</Badge>;
  };

  const StatCard = ({ icon, label, value, color }: { icon: typeof ProductIcon; label: string; value: string | number; color: string }) => (
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
    <Page title="Dashboard">
      <Layout>
        {isSupplier && (data?.active_listings ?? 0) === 0 && (
          <Layout.Section>
            <Banner tone="info" action={{ content: 'Add Products', onAction: () => navigate('/supplier/listings') }}>
              Get started by listing your products for resellers to discover.
            </Banner>
          </Layout.Section>
        )}
        {!isSupplier && (data?.imported_products ?? 0) === 0 && (
          <Layout.Section>
            <Banner tone="info" action={{ content: 'Browse Marketplace', onAction: () => navigate('/marketplace') }}>
              Import products from suppliers to start selling.
            </Banner>
          </Layout.Section>
        )}

        <Layout.Section>
          <InlineGrid columns={{ xs: 2, md: 4 }} gap="400">
            <StatCard icon={isSupplier ? ProductIcon : ImportIcon} label={isSupplier ? 'Active Listings' : 'Imported Products'} value={isSupplier ? (data?.active_listings ?? 0) : (data?.imported_products ?? 0)} color="#e3f1df" />
            <StatCard icon={OrderIcon} label="Total Orders" value={data?.order_count ?? 0} color="#e0f0ff" />
            <StatCard icon={ClockIcon} label="Pending" value={pendingOrders} color="#fef3cd" />
            <StatCard icon={CashDollarIcon} label="Revenue" value={`$${totalRevenue.toFixed(2)}`} color="#f0e6ff" />
          </InlineGrid>
        </Layout.Section>

        <Layout.Section variant="oneThird">
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Quick Links</Text>
              <Divider />
              <BlockStack gap="200">
                {isSupplier ? (
                  <>
                    <Button variant="plain" textAlign="start" onClick={() => navigate('/supplier/listings')}>Manage Listings</Button>
                    <Button variant="plain" textAlign="start" onClick={() => navigate('/supplier/setup')}>Supplier Setup</Button>
                  </>
                ) : (
                  <>
                    <Button variant="plain" textAlign="start" onClick={() => navigate('/marketplace')}>Browse Marketplace</Button>
                    <Button variant="plain" textAlign="start" onClick={() => navigate('/imports')}>Imported Products</Button>
                  </>
                )}
                <Button variant="plain" textAlign="start" onClick={() => navigate('/orders')}>View Orders</Button>
                <Button variant="plain" textAlign="start" onClick={() => navigate('/settings')}>Settings</Button>
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <InlineStack align="space-between" blockAlign="center">
                <Text as="h2" variant="headingMd">Recent Orders</Text>
                <Button variant="plain" onClick={() => navigate('/orders')}>View all</Button>
              </InlineStack>
              <Divider />
              {data?.recent_orders && data.recent_orders.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'numeric', 'text']}
                  headings={['Order', 'Status', 'Amount', 'Date']}
                  rows={data.recent_orders.slice(0, 5).map((order) => [
                    <Button key={order.id} variant="plain" onClick={() => navigate(`/orders/${order.id}`)}>{order.reseller_order_number || order.id.slice(0, 8)}</Button>,
                    statusBadge(order.status),
                    `$${order.total_wholesale_amount.toFixed(2)}`,
                    new Date(order.created_at).toLocaleDateString(),
                  ])}
                />
              ) : (
                <Box padding="400">
                  <BlockStack gap="200" align="center">
                    <Icon source={OrderIcon} tone="subdued" />
                    <Text as="p" tone="subdued" alignment="center">
                      No orders yet. {isSupplier ? 'Publish listings to start receiving orders.' : 'Import products and start selling.'}
                    </Text>
                  </BlockStack>
                </Box>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
