import { useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Page,
  Layout,
  Card,
  DataTable,
  Badge,
  Spinner,
  Banner,
  BlockStack,
  Text,
  TextField,
  FormLayout,
  Modal,
  InlineStack,
  Icon,
  Divider,
  Box,
} from '@shopify/polaris';
import { CheckIcon, ClockIcon, XIcon, PackageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { RoutedOrder, FulfillmentEvent } from '../types';

interface OrderDetailResponse {
  order: RoutedOrder;
  fulfillments: FulfillmentEvent[];
}

interface OrderDetailProps {
  role: string;
}

const ORDER_STEPS = ['pending', 'accepted', 'processing', 'fulfilled'];

function StatusTimeline({ currentStatus }: { currentStatus: string }) {
  const currentIndex = ORDER_STEPS.indexOf(currentStatus);
  const isRejected = currentStatus === 'rejected' || currentStatus === 'cancelled';

  return (
    <InlineStack gap="100" align="center" blockAlign="center">
      {ORDER_STEPS.map((step, i) => {
        const isCompleted = !isRejected && i <= currentIndex;
        const isCurrent = !isRejected && i === currentIndex;
        const color = isCompleted ? '#008060' : '#8c8c8c';
        return (
          <InlineStack key={step} gap="100" blockAlign="center">
            <div style={{
              width: '32px', height: '32px', borderRadius: '50%',
              background: isCompleted ? '#e3f1df' : (isRejected && i === 0 ? '#fde8e8' : '#f6f6f7'),
              border: isCurrent ? '2px solid #008060' : '2px solid transparent',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              {isRejected && i === 0 ? (
                <Icon source={XIcon} tone="critical" />
              ) : isCompleted ? (
                <Icon source={CheckIcon} tone="success" />
              ) : (
                <Text as="span" variant="bodySm" tone="subdued">{i + 1}</Text>
              )}
            </div>
            <Text as="span" variant="bodySm" tone={isCompleted ? undefined : 'subdued'} fontWeight={isCurrent ? 'semibold' : undefined}>
              {step.charAt(0).toUpperCase() + step.slice(1)}
            </Text>
            {i < ORDER_STEPS.length - 1 && (
              <div style={{ width: '40px', height: '2px', background: isCompleted && i < currentIndex ? '#008060' : '#e1e1e1' }} />
            )}
          </InlineStack>
        );
      })}
      {isRejected && (
        <InlineStack gap="100" blockAlign="center">
          <div style={{
            width: '32px', height: '32px', borderRadius: '50%', background: '#fde8e8',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <Icon source={XIcon} tone="critical" />
          </div>
          <Text as="span" variant="bodySm" tone="critical" fontWeight="semibold">{currentStatus}</Text>
        </InlineStack>
      )}
    </InlineStack>
  );
}

export default function OrderDetail({ role }: OrderDetailProps) {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data, loading, error, refetch } = useApi<OrderDetailResponse>(`/orders/${id}`);

  const [fulfillModal, setFulfillModal] = useState(false);
  const [trackingNumber, setTrackingNumber] = useState('');
  const [trackingUrl, setTrackingUrl] = useState('');
  const [trackingCompany, setTrackingCompany] = useState('');
  const [fulfilling, setFulfilling] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleAccept = useCallback(async () => {
    try {
      await api.post(`/supplier/orders/${id}/accept`);
      refetch();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed');
    }
  }, [id, refetch]);

  const handleReject = useCallback(async () => {
    try {
      await api.post(`/supplier/orders/${id}/reject`, { reason: 'Rejected by supplier' });
      refetch();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Failed');
    }
  }, [id, refetch]);

  const handleFulfill = useCallback(async () => {
    setFulfilling(true);
    setActionError(null);
    try {
      await api.post(`/supplier/orders/${id}/fulfill`, {
        routed_order_id: id,
        tracking_number: trackingNumber,
        tracking_url: trackingUrl,
        tracking_company: trackingCompany,
      });
      setFulfillModal(false);
      refetch();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Fulfillment failed');
    } finally {
      setFulfilling(false);
    }
  }, [id, trackingNumber, trackingUrl, trackingCompany, refetch]);

  if (loading) {
    return (
      <Page title="Order Detail">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  if (error || !data) {
    return (
      <Page title="Order Detail">
        <Banner tone="critical">{error || 'Order not found'}</Banner>
      </Page>
    );
  }

  const { order, fulfillments } = data;
  const isSupplier = role === 'supplier';
  const canAccept = isSupplier && order.status === 'pending';
  const canFulfill = isSupplier && (order.status === 'accepted' || order.status === 'processing');

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      pending: 'attention', accepted: 'info', fulfilled: 'success',
      rejected: 'critical', unfulfilled: 'attention',
    };
    return <Badge tone={toneMap[status]}>{status}</Badge>;
  };

  return (
    <Page
      title={`Order ${order.reseller_order_number || order.id.slice(0, 8)}`}
      backAction={{ content: 'Orders', onAction: () => navigate('/orders') }}
      primaryAction={canFulfill ? { content: 'Add Fulfillment', onAction: () => setFulfillModal(true) } : undefined}
      secondaryActions={canAccept ? [
        { content: 'Accept', onAction: handleAccept },
        { content: 'Reject', onAction: handleReject, destructive: true },
      ] : []}
    >
      <Layout>
        {actionError && (
          <Layout.Section>
            <Banner tone="critical" onDismiss={() => setActionError(null)}>{actionError}</Banner>
          </Layout.Section>
        )}

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Order Progress</Text>
              <Divider />
              <Box padding="400">
                <StatusTimeline currentStatus={order.status} />
              </Box>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="300">
              <InlineStack gap="200" blockAlign="center">
                <Icon source={PackageIcon} tone="subdued" />
                <Text as="h2" variant="headingMd">Order Info</Text>
              </InlineStack>
              <Divider />
              <InlineStack gap="800" wrap>
                <BlockStack gap="100">
                  <Text as="span" variant="bodySm" tone="subdued">Status</Text>
                  {statusBadge(order.status)}
                </BlockStack>
                <BlockStack gap="100">
                  <Text as="span" variant="bodySm" tone="subdued">Total</Text>
                  <Text as="span" variant="headingSm">${order.total_wholesale_amount.toFixed(2)} {order.currency}</Text>
                </BlockStack>
                <BlockStack gap="100">
                  <Text as="span" variant="bodySm" tone="subdued">Date</Text>
                  <Text as="span" variant="bodyMd">{new Date(order.created_at).toLocaleString()}</Text>
                </BlockStack>
              </InlineStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Shipping</Text>
              <Divider />
              <Text as="p" variant="bodyMd" fontWeight="semibold">{order.customer_shipping_name || 'N/A'}</Text>
              {order.customer_shipping_address && (
                <Text as="p" variant="bodySm" tone="subdued">
                  {[
                    order.customer_shipping_address.address1,
                    order.customer_shipping_address.city,
                    order.customer_shipping_address.province,
                    order.customer_shipping_address.zip,
                    order.customer_shipping_address.country,
                  ].filter(Boolean).join(', ')}
                </Text>
              )}
              {order.customer_email && <Text as="p" variant="bodySm">{order.customer_email}</Text>}
              {order.customer_phone && <Text as="p" variant="bodySm">{order.customer_phone}</Text>}
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Line Items</Text>
              <Divider />
              {order.items && order.items.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'numeric', 'numeric', 'text', 'numeric']}
                  headings={['Product', 'SKU', 'Qty', 'Unit Price', 'Status', 'Fulfilled']}
                  rows={order.items.map((item) => [
                    item.title,
                    item.sku || '-',
                    item.quantity,
                    `$${item.wholesale_unit_price.toFixed(2)}`,
                    statusBadge(item.fulfillment_status),
                    item.fulfilled_quantity,
                  ])}
                />
              ) : (
                <Text as="p" tone="subdued">No line items</Text>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>

        {fulfillments && fulfillments.length > 0 && (
          <Layout.Section>
            <Card>
              <BlockStack gap="400">
                <Text as="h2" variant="headingMd">Fulfillment History</Text>
                <Divider />
                <DataTable
                  columnContentTypes={['text', 'text', 'text', 'text', 'text']}
                  headings={['Tracking', 'Carrier', 'Status', 'Synced', 'Date']}
                  rows={fulfillments.map((f) => [
                    f.tracking_url ? (
                      <a href={f.tracking_url} target="_blank" rel="noopener noreferrer" key={f.id}>{f.tracking_number}</a>
                    ) : f.tracking_number,
                    f.tracking_company || '-',
                    statusBadge(f.status),
                    f.synced_to_reseller ? <Badge key={`s-${f.id}`} tone="success">Synced</Badge> : <Badge key={`s-${f.id}`}>Pending</Badge>,
                    new Date(f.created_at).toLocaleString(),
                  ])}
                />
              </BlockStack>
            </Card>
          </Layout.Section>
        )}
      </Layout>

      {fulfillModal && (
        <Modal
          open={true}
          onClose={() => setFulfillModal(false)}
          title="Add Fulfillment"
          primaryAction={{ content: 'Submit Fulfillment', onAction: handleFulfill, loading: fulfilling }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setFulfillModal(false) }]}
        >
          <Modal.Section>
            <FormLayout>
              <TextField label="Tracking number" value={trackingNumber} onChange={setTrackingNumber} autoComplete="off" />
              <TextField label="Tracking URL" value={trackingUrl} onChange={setTrackingUrl} autoComplete="url" />
              <TextField label="Carrier/company" value={trackingCompany} onChange={setTrackingCompany} autoComplete="off" />
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
