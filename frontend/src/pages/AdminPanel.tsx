import { useState } from 'react';
import { Page, Layout, Card, BlockStack, Text, InlineStack, Badge, Spinner, InlineGrid, Divider } from '@shopify/polaris';
import { useApi } from '../hooks/useApi';

interface AdminData {
  shops: Array<{
    id: string;
    shopify_domain: string;
    name: string;
    role: string;
    status: string;
    created_at: string;
  }>;
  stats: {
    total_shops: number;
    suppliers: number;
    resellers: number;
    active_listings: number;
    total_imports: number;
    total_orders: number;
    pending_orders: number;
    fulfilled_orders: number;
    total_revenue: number;
    total_payouts: number;
  };
  recent_orders: Array<{
    id: string;
    order_number: string;
    status: string;
    amount: number;
    currency: string;
    reseller: string;
    supplier: string;
    created_at: string;
  }>;
  recent_activity: Array<{
    action: string;
    resource_type: string;
    shop_domain: string;
    created_at: string;
  }>;
}

export default function AdminPanel() {
  const { data, loading } = useApi<AdminData>('/admin/dashboard');
  const [tab, setTab] = useState<'overview' | 'shops' | 'orders' | 'activity'>('overview');

  if (loading) {
    return <Page title="Admin Panel"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  if (!data) {
    return <Page title="Admin Panel"><Text as="p" tone="critical">Access denied or no data available.</Text></Page>;
  }

  const { shops, stats, recent_orders, recent_activity } = data;

  const roleBadge = (role: string) => {
    const tones: Record<string, 'success' | 'info' | 'attention'> = { supplier: 'success', reseller: 'info', unset: 'attention' };
    return <Badge tone={tones[role] || 'attention'}>{role}</Badge>;
  };

  const statusCfg: Record<string, { color: string; bg: string }> = {
    pending: { color: '#92400e', bg: '#fef3c7' },
    accepted: { color: '#1e40af', bg: '#dbeafe' },
    fulfilled: { color: '#166534', bg: '#dcfce7' },
    rejected: { color: '#991b1b', bg: '#fee2e2' },
    cancelled: { color: '#991b1b', bg: '#fee2e2' },
  };

  return (
    <Page title="Admin Panel" subtitle="Platform Overview">
      <Layout>
        {/* Tab navigation */}
        <Layout.Section>
          <div className="tab-pills">
            {(['overview', 'shops', 'orders', 'activity'] as const).map((t) => (
              <div key={t} className={`tab-pill ${tab === t ? 'tab-pill-active' : ''}`} onClick={() => setTab(t)}>
                {t.charAt(0).toUpperCase() + t.slice(1)}
              </div>
            ))}
          </div>
        </Layout.Section>

        {tab === 'overview' && (
          <>
            <Layout.Section>
              <InlineGrid columns={{ xs: 2, md: 4 }} gap="300">
                <div className="stat-card">
                  <div className="stat-card-label">Total Shops</div>
                  <div className="stat-card-value">{stats.total_shops}</div>
                </div>
                <div className="stat-card">
                  <div className="stat-card-label">Suppliers</div>
                  <div className="stat-card-value">{stats.suppliers}</div>
                </div>
                <div className="stat-card">
                  <div className="stat-card-label">Resellers</div>
                  <div className="stat-card-value">{stats.resellers}</div>
                </div>
                <div className="stat-card">
                  <div className="stat-card-label">Active Listings</div>
                  <div className="stat-card-value">{stats.active_listings}</div>
                </div>
              </InlineGrid>
            </Layout.Section>
            <Layout.Section>
              <InlineGrid columns={{ xs: 2, md: 4 }} gap="300">
                <div className="stat-card">
                  <div className="stat-card-label">Total Orders</div>
                  <div className="stat-card-value">{stats.total_orders}</div>
                </div>
                <div className="stat-card">
                  <div className="stat-card-label">Pending</div>
                  <div className="stat-card-value" style={{ color: '#92400e' }}>{stats.pending_orders}</div>
                </div>
                <div className="stat-card">
                  <div className="stat-card-label">Fulfilled</div>
                  <div className="stat-card-value" style={{ color: '#166534' }}>{stats.fulfilled_orders}</div>
                </div>
                <div className="stat-card">
                  <div className="stat-card-label">Total Revenue</div>
                  <div className="stat-card-value">${stats.total_revenue.toFixed(2)}</div>
                </div>
              </InlineGrid>
            </Layout.Section>
          </>
        )}

        {tab === 'shops' && (
          <Layout.Section>
            <Card>
              <BlockStack gap="0">
                <div style={{ padding: '12px 16px', fontWeight: 600, fontSize: '13px', color: '#64748b', display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr', gap: '8px' }}>
                  <span>Shop</span><span>Role</span><span>Status</span><span>Name</span><span>Joined</span>
                </div>
                <Divider />
                {shops.map((shop) => (
                  <div key={shop.id}>
                    <div style={{ padding: '10px 16px', display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr', gap: '8px', alignItems: 'center', fontSize: '13px' }}>
                      <span style={{ fontWeight: 500 }}>{shop.shopify_domain}</span>
                      <span>{roleBadge(shop.role)}</span>
                      <span><Badge tone={shop.status === 'active' ? 'success' : 'attention'}>{shop.status}</Badge></span>
                      <span style={{ color: '#64748b' }}>{shop.name || '-'}</span>
                      <span style={{ color: '#94a3b8', fontSize: '12px' }}>{new Date(shop.created_at).toLocaleDateString()}</span>
                    </div>
                    <Divider />
                  </div>
                ))}
              </BlockStack>
            </Card>
          </Layout.Section>
        )}

        {tab === 'orders' && (
          <Layout.Section>
            <Card>
              <BlockStack gap="0">
                <div style={{ padding: '12px 16px', fontWeight: 600, fontSize: '13px', color: '#64748b', display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr 1fr 1fr', gap: '8px' }}>
                  <span>Order</span><span>Status</span><span>Amount</span><span>Reseller</span><span>Supplier</span><span>Date</span>
                </div>
                <Divider />
                {(recent_orders || []).map((order) => {
                  const cfg = statusCfg[order.status] || statusCfg['pending'];
                  return (
                    <div key={order.id}>
                      <div style={{ padding: '10px 16px', display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr 1fr 1fr', gap: '8px', alignItems: 'center', fontSize: '13px' }}>
                        <span style={{ fontWeight: 600 }}>#{order.order_number}</span>
                        <span><span style={{ padding: '2px 10px', borderRadius: '12px', fontSize: '11px', fontWeight: 600, color: cfg.color, background: cfg.bg }}>{order.status}</span></span>
                        <span style={{ fontWeight: 600 }}>${order.amount.toFixed(2)} {order.currency}</span>
                        <span style={{ color: '#64748b', fontSize: '12px' }}>{order.reseller}</span>
                        <span style={{ color: '#64748b', fontSize: '12px' }}>{order.supplier}</span>
                        <span style={{ color: '#94a3b8', fontSize: '12px' }}>{new Date(order.created_at).toLocaleDateString()}</span>
                      </div>
                      <Divider />
                    </div>
                  );
                })}
                {(!recent_orders || recent_orders.length === 0) && (
                  <div style={{ padding: '24px', textAlign: 'center', color: '#94a3b8' }}>No orders yet</div>
                )}
              </BlockStack>
            </Card>
          </Layout.Section>
        )}

        {tab === 'activity' && (
          <Layout.Section>
            <Card>
              <BlockStack gap="0">
                {(recent_activity || []).map((a, i) => (
                  <div key={i}>
                    <div style={{ padding: '10px 16px', display: 'flex', justifyContent: 'space-between', fontSize: '13px' }}>
                      <InlineStack gap="200" blockAlign="center">
                        <Badge>{a.action}</Badge>
                        <Text as="span" variant="bodySm" tone="subdued">{a.resource_type}</Text>
                        <Text as="span" variant="bodySm">{a.shop_domain}</Text>
                      </InlineStack>
                      <Text as="span" variant="bodySm" tone="subdued">{new Date(a.created_at).toLocaleString()}</Text>
                    </div>
                    <Divider />
                  </div>
                ))}
                {(!recent_activity || recent_activity.length === 0) && (
                  <div style={{ padding: '24px', textAlign: 'center', color: '#94a3b8' }}>No recent activity</div>
                )}
              </BlockStack>
            </Card>
          </Layout.Section>
        )}
      </Layout>
    </Page>
  );
}
