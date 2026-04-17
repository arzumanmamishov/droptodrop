import { useState, useEffect, useCallback } from 'react';
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
  Modal,
} from '@shopify/polaris';
import { OrderIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';
import { COUNTRIES, COUNTRY_NAMES } from '../constants/countries';


interface DashboardData {
  role: string;
  active_listings?: number;
  imported_products?: number;
  order_count?: number;
  paypal_email?: string;
  shipping_countries?: string[];
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
  const toast = useToast();
  const { data, loading, error, refetch } = useApi<DashboardData>('/dashboard');
  const [countryModal, setCountryModal] = useState(false);
  const [selectedCountries, setSelectedCountries] = useState<Set<string>>(new Set());
  const [countrySearch, setCountrySearch] = useState('');
  const [savingCountries, setSavingCountries] = useState(false);

  useEffect(() => {
    if (data?.shipping_countries && data.shipping_countries.length > 0) {
      setSelectedCountries(new Set(data.shipping_countries));
    }
  }, [data?.shipping_countries]);

  const handleSaveCountries = useCallback(async () => {
    setSavingCountries(true);
    try {
      await api.put('/supplier/profile', { shipping_countries: Array.from(selectedCountries) });
      toast.success('Shipping countries saved');
      setCountryModal(false);
      refetch();
    } catch { toast.error('Failed to save'); }
    finally { setSavingCountries(false); }
  }, [selectedCountries, refetch, toast]);


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
        {/* Getting Started Checklist */}
        {isSupplier && ((data?.active_listings ?? 0) === 0 || !data?.paypal_email || !data?.shipping_countries?.length) && (
          <Layout.Section>
            <Card>
              <BlockStack gap="300">
                <Text as="h2" variant="headingMd">🚀 Getting Started</Text>
                <Divider />
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  {[
                    { done: !!data?.paypal_email, label: 'Add your PayPal email', action: () => navigate('/supplier/setup'), actionLabel: 'Go to Settings' },
                    { done: (data?.shipping_countries?.length || 0) > 0, label: 'Select shipping countries', action: () => setCountryModal(true), actionLabel: 'Select' },
                    { done: (data?.active_listings ?? 0) > 0, label: 'List your first product', action: () => navigate('/supplier/listings'), actionLabel: 'Add Product' },
                  ].map((step, i) => (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', borderRadius: '8px', background: step.done ? '#f0fdf4' : '#f8fafc' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                        <span style={{ width: '24px', height: '24px', borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '12px', fontWeight: 700, background: step.done ? '#166534' : '#e2e8f0', color: step.done ? '#fff' : '#94a3b8' }}>
                          {step.done ? '✓' : i + 1}
                        </span>
                        <span style={{ fontSize: '14px', fontWeight: 500, color: step.done ? '#166534' : '#1e293b', textDecoration: step.done ? 'line-through' : 'none' }}>{step.label}</span>
                      </div>
                      {!step.done && <button onClick={step.action} style={{ padding: '4px 12px', fontSize: '12px', fontWeight: 600, background: '#1e40af', color: '#fff', border: 'none', borderRadius: '6px', cursor: 'pointer' }}>{step.actionLabel}</button>}
                    </div>
                  ))}
                </div>
              </BlockStack>
            </Card>
          </Layout.Section>
        )}
        {!isSupplier && ((data?.imported_products ?? 0) === 0 || !data?.paypal_email) && (
          <Layout.Section>
            <Card>
              <BlockStack gap="300">
                <Text as="h2" variant="headingMd">🚀 Getting Started</Text>
                <Divider />
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  {[
                    { done: !!data?.paypal_email, label: 'Add your PayPal email', action: () => navigate('/reseller/settings'), actionLabel: 'Go to Settings' },
                    { done: (data?.imported_products ?? 0) > 0, label: 'Import your first product from Marketplace', action: () => navigate('/marketplace'), actionLabel: 'Browse' },
                  ].map((step, i) => (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', borderRadius: '8px', background: step.done ? '#f0fdf4' : '#f8fafc' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                        <span style={{ width: '24px', height: '24px', borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '12px', fontWeight: 700, background: step.done ? '#166534' : '#e2e8f0', color: step.done ? '#fff' : '#94a3b8' }}>
                          {step.done ? '✓' : i + 1}
                        </span>
                        <span style={{ fontSize: '14px', fontWeight: 500, color: step.done ? '#166534' : '#1e293b', textDecoration: step.done ? 'line-through' : 'none' }}>{step.label}</span>
                      </div>
                      {!step.done && <button onClick={step.action} style={{ padding: '4px 12px', fontSize: '12px', fontWeight: 600, background: '#1e40af', color: '#fff', border: 'none', borderRadius: '6px', cursor: 'pointer' }}>{step.actionLabel}</button>}
                    </div>
                  ))}
                </div>
              </BlockStack>
            </Card>
          </Layout.Section>
        )}
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
        {isSupplier && (!data?.shipping_countries || data.shipping_countries.length === 0) && (
          <Layout.Section>
            <Banner tone="warning" action={{ content: 'Select Countries', onAction: () => setCountryModal(true) }}>
              Select which countries you ship to. This helps resellers know if your products are available in their region.
            </Banner>
          </Layout.Section>
        )}
        {isSupplier && data?.shipping_countries && data.shipping_countries.length > 0 && (
          <Layout.Section>
            <Card>
              <InlineStack align="space-between" blockAlign="center">
                <InlineStack gap="200" blockAlign="center" wrap>
                  <Text as="span" variant="bodyMd" fontWeight="semibold">Ships to:</Text>
                  {data.shipping_countries.map(c => (
                    <span key={c} style={{ padding: '2px 8px', borderRadius: '6px', fontSize: '12px', fontWeight: 600, background: '#dbeafe', color: '#1e40af' }}>
                      {COUNTRY_NAMES[c] || c}
                    </span>
                  ))}
                </InlineStack>
                <Button size="slim" onClick={() => setCountryModal(true)}>Change</Button>
              </InlineStack>
            </Card>
          </Layout.Section>
        )}
        {isSupplier && !data?.paypal_email && (
          <Layout.Section>
            <Banner tone="warning" action={{ content: 'Add PayPal Email', onAction: () => navigate('/supplier/setup') }}>
              Add your PayPal email so resellers can pay you directly. Without it, resellers won't see a PayPal pay button on the Payouts page.
            </Banner>
          </Layout.Section>
        )}
        {!isSupplier && !data?.paypal_email && (
          <Layout.Section>
            <Banner tone="warning" action={{ content: 'Add PayPal Email', onAction: () => navigate('/reseller/settings') }}>
              Add your PayPal email in Settings so suppliers can verify your payments.
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

      {countryModal && (
        <Modal
          open
          onClose={() => setCountryModal(false)}
          title="Select Shipping Countries"
          primaryAction={{ content: selectedCountries.size > 0 ? `Save (${selectedCountries.size} selected)` : 'Ship Worldwide', onAction: handleSaveCountries, loading: savingCountries }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setCountryModal(false) }]}
        >
          <Modal.Section>
            <BlockStack gap="300">
              <input
                type="text" placeholder="Search countries..." value={countrySearch}
                onChange={(e) => setCountrySearch(e.target.value)}
                style={{ width: '100%', padding: '8px 12px', border: '1px solid #e2e8f0', borderRadius: '8px', fontSize: '14px' }}
              />
              <InlineStack gap="200" wrap>
                <Button size="slim" onClick={() => setSelectedCountries(new Set(COUNTRIES))}>Select All</Button>
                <Button size="slim" onClick={() => setSelectedCountries(new Set())}>Clear All</Button>
                <Button size="slim" onClick={() => setSelectedCountries(new Set(['DE','AT','CH','FR','IT','ES','NL','BE','PT','PL','CZ','SE','DK','NO','FI','IE','GB']))}>EU + UK</Button>
                <Button size="slim" onClick={() => setSelectedCountries(new Set(['US','CA','MX']))}>North America</Button>
              </InlineStack>
              <div style={{ maxHeight: '300px', overflowY: 'auto', border: '1px solid #f1f5f9', borderRadius: '8px' }}>
                {COUNTRIES.filter(c => {
                  const name = (COUNTRY_NAMES[c] || c).toLowerCase();
                  return !countrySearch || name.includes(countrySearch.toLowerCase()) || c.toLowerCase().includes(countrySearch.toLowerCase());
                }).map(code => (
                  <label key={code} style={{
                    display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 12px', cursor: 'pointer',
                    background: selectedCountries.has(code) ? '#eff6ff' : 'transparent',
                    borderBottom: '1px solid #f8fafc',
                  }}>
                    <input type="checkbox" checked={selectedCountries.has(code)}
                      onChange={() => {
                        setSelectedCountries(prev => {
                          const next = new Set(prev);
                          if (next.has(code)) next.delete(code); else next.add(code);
                          return next;
                        });
                      }}
                      style={{ width: '16px', height: '16px', accentColor: '#1e40af' }}
                    />
                    <span style={{ fontSize: '13px' }}>{COUNTRY_NAMES[code] || code}</span>
                    <span style={{ fontSize: '11px', color: '#94a3b8' }}>{code}</span>
                  </label>
                ))}
              </div>
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
