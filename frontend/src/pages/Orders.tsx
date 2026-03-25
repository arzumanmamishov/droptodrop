import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page,
  Layout,
  Card,
  DataTable,
  Badge,
  Button,
  Spinner,
  Banner,
  BlockStack,
  Text,
  InlineStack,
  Filters,
  ChoiceList,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
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
  const limit = 20;

  const endpoint = role === 'supplier' ? '/supplier/orders' : '/reseller/orders';
  const statusQuery = statusFilter.length === 1 ? `&status=${statusFilter[0]}` : '';
  const { data, loading, error } = useApi<OrdersResponse>(
    `${endpoint}?limit=${limit}&offset=${page * limit}${statusQuery}`,
  );

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
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info' | 'warning'> = {
      pending: 'attention',
      accepted: 'info',
      rejected: 'critical',
      processing: 'info',
      fulfilled: 'success',
      partially_fulfilled: 'warning',
      cancelled: 'critical',
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
    <Page title="Orders">
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}
        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Filters
                queryValue=""
                filters={[
                  {
                    key: 'status',
                    label: 'Status',
                    filter: (
                      <ChoiceList
                        title="Status"
                        titleHidden
                        choices={[
                          { label: 'Pending', value: 'pending' },
                          { label: 'Accepted', value: 'accepted' },
                          { label: 'Fulfilled', value: 'fulfilled' },
                          { label: 'Rejected', value: 'rejected' },
                        ]}
                        selected={statusFilter}
                        onChange={setStatusFilter}
                      />
                    ),
                    shortcut: true,
                  },
                ]}
                onQueryChange={() => {}}
                onQueryClear={() => {}}
                onClearAll={() => setStatusFilter([])}
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
    </Page>
  );
}
