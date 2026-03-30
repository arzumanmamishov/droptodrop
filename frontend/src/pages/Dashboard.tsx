import { useState, useEffect } from 'react';
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
import { OrderIcon } from '@shopify/polaris-icons';
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
  const { data, loading, error, refetch } = useApi<DashboardData>('/dashboard');

  // Auto-refresh every 30 seconds
  useEffect(() => {
    const interval = setInterval(() => refetch(), 30000);
    return () => clearInterval(interval);
  }, [refetch]);

  // Platform stats
  const [platformStats, setPlatformStats] = useState<{ total_products: number; total_orders: number; total_suppliers: number; total_resellers: number } | null>(null);
  useEffect(() => {
    fetch('/public/stats').then(r => r.json()).then(setPlatformStats).catch(() => {});
  }, []);

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

  const StatCard = ({ label, value, sublabel }: { label: string; value: string | number; sublabel: string }) => (
    <div className="stat-card">
      <div style={{ position: 'relative', zIndex: 1 }}>
        <div className="stat-card-label">{label}</div>
        <div className="stat-card-value">{value}</div>
        <div className="stat-card-sublabel">{sublabel}</div>
      </div>
    </div>
  );

  return (
    <Page title="Dashboard">
      <Layout>
        <Layout.Section>
          <div className="page-header-accent" />
        </Layout.Section>
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
            <StatCard label={isSupplier ? 'Active Listings' : 'Imported Products'} value={isSupplier ? (data?.active_listings ?? 0) : (data?.imported_products ?? 0)} sublabel={`Total ${isSupplier ? 'listings' : 'imports'}`} />
            <StatCard label="Total Orders" value={data?.order_count ?? 0} sublabel="All time orders" />
            <StatCard label="Pending Orders" value={pendingOrders} sublabel="Awaiting action" />
            <StatCard label="Revenue" value={`$${totalRevenue.toFixed(2)}`} sublabel="From recent orders" />
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
        {platformStats && (
          <Layout.Section>
            <div className="platform-banner">
              <BlockStack gap="200">
                <Text as="p" variant="bodySm">
                  <span style={{ color: 'rgba(255,255,255,0.6)' }}>DropToDrop Network</span>
                </Text>
                <InlineStack gap="600" align="center">
                  <BlockStack gap="050" align="center">
                    <span style={{ fontSize: '24px', fontWeight: 700, color: 'white' }}>{platformStats.total_products}</span>
                    <span style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)' }}>Products</span>
                  </BlockStack>
                  <BlockStack gap="050" align="center">
                    <span style={{ fontSize: '24px', fontWeight: 700, color: 'white' }}>{platformStats.total_orders}</span>
                    <span style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)' }}>Orders</span>
                  </BlockStack>
                  <BlockStack gap="050" align="center">
                    <span style={{ fontSize: '24px', fontWeight: 700, color: 'white' }}>{platformStats.total_suppliers}</span>
                    <span style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)' }}>Suppliers</span>
                  </BlockStack>
                  <BlockStack gap="050" align="center">
                    <span style={{ fontSize: '24px', fontWeight: 700, color: 'white' }}>{platformStats.total_resellers}</span>
                    <span style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)' }}>Resellers</span>
                  </BlockStack>
                </InlineStack>
              </BlockStack>
            </div>
          </Layout.Section>
        )}
      </Layout>
    </Page>
  );
}
