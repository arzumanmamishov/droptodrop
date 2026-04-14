import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, Button, Spinner,
  Banner, BlockStack, Text, InlineStack,
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
    setRouting(true); setRouteError(null); setRouteResult(null);
    try {
      const result = await api.post<{ message: string }>('/test/route-order', { order_id: parseInt(routeOrderId) });
      setRouteResult(result.message || 'Order routed!');
      setShowRouteModal(false); setRouteOrderId(''); refetch();
    } catch (err) { setRouteError(err instanceof Error ? err.message : 'Failed to route order'); }
    finally { setRouting(false); }
  };

  const secondaryActions: Array<{ content: string; onAction: () => void }> = [];
  if (role === 'reseller') secondaryActions.push({ content: 'Route Order', onAction: () => setShowRouteModal(true) });
  secondaryActions.push({ content: 'Export CSV', onAction: handleExport });

  if (loading) return <Page title="Orders"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const orders = data?.orders || [];
  const totalPages = Math.ceil((data?.total || 0) / limit);
  const totalCount = data?.total || 0;

  return (
    <Page title="Orders" subtitle={`${totalCount} order${totalCount !== 1 ? 's' : ''}`} secondaryActions={secondaryActions}>
      <Layout>
        {error && <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>}
        {routeResult && <Layout.Section><Banner tone="success" onDismiss={() => setRouteResult(null)}>{routeResult}</Banner></Layout.Section>}
        {routeError && <Layout.Section><Banner tone="critical" onDismiss={() => setRouteError(null)}>{routeError}</Banner></Layout.Section>}

        <Layout.Section>
          <div className="tab-pills" style={{ flexWrap: 'wrap' }}>
            {STATUS_TABS.map((tab) => (
              <div key={tab.value} className={`tab-pill ${statusFilter === tab.value ? 'tab-pill-active' : ''}`}
                onClick={() => { setStatusFilter(tab.value); setPage(0); }}>
                {tab.label}
              </div>
            ))}
          </div>
        </Layout.Section>

        <Layout.Section>
          {orders.length > 0 ? (
            <BlockStack gap="300">
              {orders.map((order) => {
                const cfg = statusConfig[order.status] || statusConfig['pending'];
                const items = order.items || [];
                const firstImage = items.find(i => i.image_url)?.image_url;

                return (
                  <Card key={order.id}>
                    <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-start' }}>
                      {/* Product image thumbnails */}
                      <div style={{ flexShrink: 0, display: 'flex', gap: '4px' }}>
                        {items.length > 0 ? (
                          items.slice(0, 3).map((item, i) => (
                            <div key={i} style={{
                              width: '56px', height: '56px', borderRadius: '10px', overflow: 'hidden',
                              background: '#f1f5f9', border: '1px solid #e2e8f0',
                            }}>
                              {item.image_url ? (
                                <img src={item.image_url} alt={item.title} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
                              ) : (
                                <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '20px', color: '#cbd5e1' }}>
                                  📦
                                </div>
                              )}
                            </div>
                          ))
                        ) : firstImage ? (
                          <div style={{ width: '56px', height: '56px', borderRadius: '10px', overflow: 'hidden', background: '#f1f5f9', border: '1px solid #e2e8f0' }}>
                            <img src={firstImage} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
                          </div>
                        ) : (
                          <div style={{
                            width: '56px', height: '56px', borderRadius: '10px',
                            background: cfg.bg, display: 'flex', alignItems: 'center', justifyContent: 'center',
                            fontSize: '20px',
                          }}>
                            📦
                          </div>
                        )}
                        {items.length > 3 && (
                          <div style={{
                            width: '56px', height: '56px', borderRadius: '10px',
                            background: '#f1f5f9', border: '1px solid #e2e8f0',
                            display: 'flex', alignItems: 'center', justifyContent: 'center',
                            fontSize: '13px', fontWeight: 600, color: '#64748b',
                          }}>
                            +{items.length - 3}
                          </div>
                        )}
                      </div>

                      {/* Order info */}
                      <div style={{ flex: 1, minWidth: 0 }}>
                        {/* Top row */}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '6px' }}>
                          <div>
                            <div style={{ fontSize: '15px', fontWeight: 700, color: '#1e293b' }}>
                              Order #{order.reseller_order_number || order.id.slice(0, 8)}
                            </div>
                            <div style={{ fontSize: '12px', color: '#94a3b8', marginTop: '2px' }}>
                              {new Date(order.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })} {new Date(order.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                              {role === 'supplier' && order.reseller_shop_name ? ` · from ${order.reseller_shop_name}` : ''}
                              {role === 'reseller' && order.supplier_shop_name ? ` · via ${order.supplier_shop_name}` : ''}
                              {order.customer_shipping_name ? ` · ${order.customer_shipping_name}` : ''}
                            </div>
                          </div>
                          <div style={{ textAlign: 'right' }}>
                            <div style={{ fontSize: '16px', fontWeight: 700, color: '#1e293b' }}>
                              ${order.total_wholesale_amount.toFixed(2)}
                            </div>
                            <div style={{ fontSize: '11px', color: '#94a3b8' }}>{order.currency}</div>
                          </div>
                        </div>

                        {/* Status + product names */}
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', marginBottom: '8px' }}>
                          <span style={{
                            padding: '3px 12px', borderRadius: '20px', fontSize: '12px', fontWeight: 700,
                            color: cfg.color, background: cfg.bg,
                          }}>
                            {cfg.label}
                          </span>
                          {(() => {
                            const payMap: Record<string, { color: string; bg: string; label: string }> = {
                              unpaid: { color: '#92400e', bg: '#fef3c7', label: 'Unpaid' },
                              pending: { color: '#92400e', bg: '#fef3c7', label: 'Unpaid' },
                              payment_sent: { color: '#1e40af', bg: '#dbeafe', label: 'Payment Sent' },
                              paid: { color: '#166534', bg: '#dcfce7', label: 'Paid' },
                              disputed: { color: '#991b1b', bg: '#fee2e2', label: 'Disputed' },
                            };
                            const pay = payMap[order.pay_status || 'unpaid'] || payMap['unpaid'];
                            return (
                              <span style={{
                                padding: '3px 10px', borderRadius: '20px', fontSize: '11px', fontWeight: 700,
                                color: pay.color, background: pay.bg,
                              }}>
                                {pay.label}
                              </span>
                            );
                          })()}
                          {items.length > 0 && (
                            <span style={{ fontSize: '12px', color: '#64748b' }}>
                              {items.map(i => i.title).join(', ').slice(0, 60)}{items.map(i => i.title).join(', ').length > 60 ? '...' : ''}
                            </span>
                          )}
                        </div>

                        {/* Product pills */}
                        {items.length > 0 && (
                          <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginBottom: '10px' }}>
                            {items.slice(0, 4).map((item, i) => (
                              <span key={i} style={{
                                padding: '3px 10px', borderRadius: '6px', fontSize: '11px',
                                background: '#f1f5f9', color: '#475569', fontWeight: 500,
                              }}>
                                {item.title} ×{item.quantity} · ${item.wholesale_unit_price.toFixed(2)}
                              </span>
                            ))}
                          </div>
                        )}

                        {/* Details button */}
                        <button
                          onClick={() => navigate(`/orders/${order.id}`)}
                          style={{
                            padding: '6px 20px', fontSize: '13px', fontWeight: 600,
                            background: '#111', color: '#fff', border: 'none', borderRadius: '8px',
                            cursor: 'pointer', transition: 'background 0.15s',
                          }}
                          onMouseOver={(e) => (e.currentTarget.style.background = '#333')}
                          onMouseOut={(e) => (e.currentTarget.style.background = '#111')}
                        >
                          Details
                        </button>
                      </div>
                    </div>
                  </Card>
                );
              })}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading="No orders" image="">
                <p>{statusFilter ? `No ${statusFilter} orders found.` : role === 'supplier' ? 'Orders will appear when resellers sell your products.' : 'Orders will appear when customers buy imported products.'}</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>

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
              <Text as="p" variant="bodyMd">Enter the Shopify order ID to manually route it to the supplier.</Text>
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
