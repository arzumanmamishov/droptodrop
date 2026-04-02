import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, Badge, Button, Spinner,
  Banner, BlockStack, Text, InlineStack, Divider,
  Modal, TextField, EmptyState,
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

const STATUS_TABS = [
  { label: 'All', value: '' },
  { label: 'Pending', value: 'pending' },
  { label: 'Accepted', value: 'accepted' },
  { label: 'Processing', value: 'processing' },
  { label: 'Fulfilled', value: 'fulfilled' },
  { label: 'Cancelled', value: 'cancelled' },
];

const statusConfig: Record<string, { color: string; bg: string; label: string }> = {
  pending:    { color: '#92400e', bg: '#fef3c7', label: 'Pending' },
  accepted:   { color: '#1e40af', bg: '#dbeafe', label: 'Accepted' },
  processing: { color: '#6d28d9', bg: '#ede9fe', label: 'Processing' },
  fulfilled:  { color: '#166534', bg: '#dcfce7', label: 'Fulfilled' },
  rejected:   { color: '#991b1b', bg: '#fee2e2', label: 'Rejected' },
  cancelled:  { color: '#991b1b', bg: '#fee2e2', label: 'Cancelled' },
};

export default function Orders({ role }: OrdersProps) {
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState('');
  const [page, setPage] = useState(0);
  const [routeOrderId, setRouteOrderId] = useState('');
  const [routing, setRouting] = useState(false);
  const [routeResult, setRouteResult] = useState<string | null>(null);
  const [routeError, setRouteError] = useState<string | null>(null);
  const [showRouteModal, setShowRouteModal] = useState(false);
  const limit = 20;

  const endpoint = role === 'supplier' ? '/supplier/orders' : '/reseller/orders';
  const statusQuery = statusFilter ? `&status=${statusFilter}` : '';
  const { data, loading, error, refetch } = useApi<OrdersResponse>(
    `${endpoint}?limit=${limit}&offset=${page * limit}${statusQuery}`,
  );

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

  const orders = data?.orders || [];
  const totalPages = Math.ceil((data?.total || 0) / limit);

  // Count by status for tab badges
  const totalCount = data?.total || 0;

  return (
    <Page title="Orders" subtitle={`${totalCount} order${totalCount !== 1 ? 's' : ''}`} secondaryActions={secondaryActions}>
      <Layout>
        {error && <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>}
        {routeResult && <Layout.Section><Banner tone="success" onDismiss={() => setRouteResult(null)}>{routeResult}</Banner></Layout.Section>}
        {routeError && <Layout.Section><Banner tone="critical" onDismiss={() => setRouteError(null)}>{routeError}</Banner></Layout.Section>}

        {/* Status filter tabs */}
        <Layout.Section>
          <div className="tab-pills" style={{ flexWrap: 'wrap' }}>
            {STATUS_TABS.map((tab) => (
              <div
                key={tab.value}
                className={`tab-pill ${statusFilter === tab.value ? 'tab-pill-active' : ''}`}
                onClick={() => { setStatusFilter(tab.value); setPage(0); }}
              >
                {tab.label}
              </div>
            ))}
          </div>
        </Layout.Section>

        {/* Orders list */}
        <Layout.Section>
          {orders.length > 0 ? (
            <BlockStack gap="300">
              {orders.map((order) => {
                const cfg = statusConfig[order.status] || statusConfig['pending'];
                const itemCount = order.items?.length || 0;
                const itemSummary = order.items?.map(i => `${i.title} x${i.quantity}`).join(', ') || '';

                return (
                  <Card key={order.id}>
                    <div
                      style={{ cursor: 'pointer', padding: '4px 0' }}
                      onClick={() => navigate(`/orders/${order.id}`)}
                    >
                      {/* Top row: order number + status */}
                      <InlineStack align="space-between" blockAlign="center" wrap={false}>
                        <InlineStack gap="300" blockAlign="center">
                          <div style={{
                            padding: '6px 10px', borderRadius: '8px',
                            background: cfg.bg,
                            fontSize: '12px', fontWeight: 700, color: cfg.color, flexShrink: 0,
                            whiteSpace: 'nowrap',
                          }}>
                            #{order.reseller_order_number || order.id.slice(0, 6)}
                          </div>
                          <div>
                            <Text as="span" variant="bodyMd" fontWeight="semibold">
                              Order {order.reseller_order_number || order.id.slice(0, 8)}
                            </Text>
                            <div style={{ marginTop: '2px' }}>
                              <Text as="span" variant="bodySm" tone="subdued">
                                {new Date(order.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                                {order.customer_shipping_name ? ` \u00b7 ${order.customer_shipping_name}` : ''}
                              </Text>
                            </div>
                          </div>
                        </InlineStack>

                        <InlineStack gap="300" blockAlign="center">
                          <span style={{
                            padding: '4px 12px', borderRadius: '20px',
                            fontSize: '12px', fontWeight: 600,
                            color: cfg.color, background: cfg.bg,
                          }}>
                            {cfg.label}
                          </span>
                          <div style={{ textAlign: 'right' }}>
                            <Text as="span" variant="headingSm" fontWeight="bold">
                              ${order.total_wholesale_amount.toFixed(2)}
                            </Text>
                            <div>
                              <Text as="span" variant="bodySm" tone="subdued">{order.currency}</Text>
                            </div>
                          </div>
                        </InlineStack>
                      </InlineStack>

                      {/* Items summary */}
                      {itemCount > 0 && (
                        <>
                          <Divider />
                          <div style={{ marginTop: '10px' }}>
                            <InlineStack gap="200" blockAlign="center" wrap>
                              <Badge tone="info">{`${itemCount} item${itemCount !== 1 ? 's' : ''}`}</Badge>
                              <Text as="span" variant="bodySm" tone="subdued">
                                {itemSummary.length > 80 ? itemSummary.slice(0, 80) + '...' : itemSummary}
                              </Text>
                            </InlineStack>
                          </div>
                        </>
                      )}

                      {/* Customer info row */}
                      {(order.customer_email || order.customer_phone) && (
                        <div style={{ marginTop: '6px' }}>
                          <Text as="span" variant="bodySm" tone="subdued">
                            {[order.customer_email, order.customer_phone].filter(Boolean).join(' \u00b7 ')}
                          </Text>
                        </div>
                      )}
                    </div>
                  </Card>
                );
              })}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading="No orders" image="">
                <p>
                  {statusFilter
                    ? `No ${statusFilter} orders found. Try a different filter.`
                    : role === 'supplier'
                      ? 'Orders will appear when resellers sell your products.'
                      : 'Orders will appear when customers buy imported products.'
                  }
                </p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>

        {/* Pagination */}
        {totalPages > 1 && (
          <Layout.Section>
            <InlineStack align="center" gap="200">
              <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</Button>
              <Text as="span" variant="bodySm">Page {page + 1} of {totalPages}</Text>
              <Button disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>Next</Button>
            </InlineStack>
          </Layout.Section>
        )}
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
