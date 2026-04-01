import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, DataTable, Badge, Button, Spinner,
  Banner, BlockStack, Text, InlineStack, Filters, ChoiceList,
  Modal, TextField,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { RoutedOrder } from '../types';

interface OrdersResponse {
  orders: RoutedOrder[];
  total: number;
}

interface OrdersProps {
  role: string;
}

export default function Orders({ role }: OrdersProps) {
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [page, setPage] = useState(0);
  const [routeOrderId, setRouteOrderId] = useState('');
  const [routing, setRouting] = useState(false);
  const [routeResult, setRouteResult] = useState<string | null>(null);
  const [routeError, setRouteError] = useState<string | null>(null);
  const [showRouteModal, setShowRouteModal] = useState(false);
  const limit = 20;

  const endpoint = role === 'supplier' ? '/supplier/orders' : '/reseller/orders';
  const statusQuery = statusFilter.length === 1 ? `&status=${statusFilter[0]}` : '';
  const { data, loading, error, refetch } = useApi<OrdersResponse>(
    `${endpoint}?limit=${limit}&offset=${page * limit}${statusQuery}`,
  );

  // Auto-refresh every 10 seconds
  useEffect(() => {
    const interval = setInterval(() => refetch(), 10000);
    return () => clearInterval(interval);
  }, [refetch]);

  const handleExport = () => {
    (async () => {
      try {
        const token = window.shopify?.idToken ? await window.shopify.idToken() : (localStorage.getItem('droptodrop_session') || '');
        const response = await fetch('/api/v1/export/orders', { headers: { Authorization: `Bearer ${token}` } });
        const blob = await response.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a'); a.href = url; a.download = 'orders.csv'; a.click();
        URL.revokeObjectURL(url);
      } catch { /* */ }
    })();
  };

  const handleRouteOrder = async () => {
    setRouting(true);
    setRouteError(null);
    setRouteResult(null);
    try {
      const result = await api.post<{ message: string }>('/test/route-order', { order_id: parseInt(routeOrderId) });
      setRouteResult(result.message || 'Order routed!');
      setShowRouteModal(false);
      setRouteOrderId('');
      refetch();
    } catch (err) {
      setRouteError(err instanceof Error ? err.message : 'Failed to route order');
    } finally {
      setRouting(false);
    }
  };

  const secondaryActions: Array<{ content: string; onAction: () => void }> = [];
  if (role === 'reseller') {
    secondaryActions.push({ content: 'Route Order', onAction: () => setShowRouteModal(true) });
  }
  secondaryActions.push({ content: 'Export CSV', onAction: handleExport });

  if (loading) {
    return (
      <Page title="Orders">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      pending: 'attention', accepted: 'info', rejected: 'critical', processing: 'info',
      fulfilled: 'success', cancelled: 'critical',
    };
    return <Badge tone={toneMap[status]}>{status}</Badge>;
  };

  const rows = (data?.orders || []).map((order) => [
    <Button key={order.id} variant="plain" onClick={() => navigate(`/orders/${order.id}`)}>
      {order.reseller_order_number || order.id.slice(0, 8)}
    </Button>,
    statusBadge(order.status),
    `$${order.total_wholesale_amount.toFixed(2)}`,
    order.currency,
    order.customer_shipping_name || '-',
    new Date(order.created_at).toLocaleDateString(),
  ]);

  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page title="Orders" secondaryActions={secondaryActions}>
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}
        {routeResult && (
          <Layout.Section>
            <Banner tone="success" onDismiss={() => setRouteResult(null)}>{routeResult}</Banner>
          </Layout.Section>
        )}
        {routeError && (
          <Layout.Section>
            <Banner tone="critical" onDismiss={() => setRouteError(null)}>{routeError}</Banner>
          </Layout.Section>
        )}
        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Filters
                queryValue=""
                filters={[{
                  key: 'status', label: 'Status',
                  filter: (
                    <ChoiceList title="Status" titleHidden
                      choices={[
                        { label: 'Pending', value: 'pending' },
                        { label: 'Accepted', value: 'accepted' },
                        { label: 'Fulfilled', value: 'fulfilled' },
                        { label: 'Rejected', value: 'rejected' },
                        { label: 'Cancelled', value: 'cancelled' },
                      ]}
                      selected={statusFilter} onChange={setStatusFilter}
                    />
                  ),
                  shortcut: true,
                }]}
                onQueryChange={() => {}} onQueryClear={() => {}} onClearAll={() => setStatusFilter([])}
              />

              {rows.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'numeric', 'text', 'text', 'text']}
                  headings={['Order', 'Status', 'Wholesale Total', 'Currency', 'Customer', 'Date']}
                  rows={rows}
                />
              ) : (
                <Text as="p" tone="subdued">
                  No orders yet. {role === 'supplier' ? 'Orders will appear when resellers sell your products.' : 'Orders will appear when customers buy imported products.'}
                </Text>
              )}

              {totalPages > 1 && (
                <InlineStack align="center" gap="200">
                  <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</Button>
                  <Text as="span" variant="bodySm">Page {page + 1} of {totalPages}</Text>
                  <Button disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>Next</Button>
                </InlineStack>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>

      {showRouteModal && (
        <Modal open onClose={() => setShowRouteModal(false)} title="Route a Shopify Order"
          primaryAction={{ content: 'Route Order', onAction: handleRouteOrder, loading: routing, disabled: !routeOrderId }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setShowRouteModal(false) }]}
        >
          <Modal.Section>
            <BlockStack gap="300">
              <Text as="p" variant="bodyMd">
                Enter the Shopify order ID to manually route it to the supplier.
                Find it in Shopify admin → Orders → click the order → copy the number from the URL.
              </Text>
              <TextField label="Shopify Order ID" value={routeOrderId} onChange={setRouteOrderId}
                type="number" autoComplete="off" placeholder="e.g. 6789012345"
                helpText="The numeric order ID from your Shopify admin URL"
              />
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
