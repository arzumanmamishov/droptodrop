import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Spinner,
  Banner, InlineStack, Divider, Modal, FormLayout, TextField, Select,
  EmptyState, Icon, InlineGrid,
} from '@shopify/polaris';
import { DiscountIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';

interface Deal {
  id: string; title: string; discount_type: string; discount_value: number;
  starts_at: string; ends_at: string; max_uses: number; current_uses: number;
  is_active: boolean; created_at: string;
}
interface Props { role: string; }

export default function Deals({ role }: Props) {
  const isSupplier = role === 'supplier';
  const { data, loading, refetch } = useApi<{ deals: Deal[] }>('/deals');
  const [createModal, setCreateModal] = useState(false);
  const [title, setTitle] = useState('');
  const [discountType, setDiscountType] = useState('percentage');
  const [discountValue, setDiscountValue] = useState('10');
  const [daysValid, setDaysValid] = useState('7');
  const [maxUses, setMaxUses] = useState('0');
  const [creating, setCreating] = useState(false);
  const [success, setSuccess] = useState(false);

  const handleCreate = useCallback(async () => {
    setCreating(true);
    try {
      const now = new Date();
      const ends = new Date(now.getTime() + parseInt(daysValid) * 24 * 60 * 60 * 1000);
      await api.post('/deals', {
        title, discount_type: discountType,
        discount_value: parseFloat(discountValue),
        starts_at: now.toISOString(), ends_at: ends.toISOString(),
        max_uses: parseInt(maxUses) || 0,
      });
      setSuccess(true);
      setCreateModal(false);
      setTitle(''); setDiscountValue('10');
      refetch();
    } catch { /* */ }
    finally { setCreating(false); }
  }, [title, discountType, discountValue, daysValid, maxUses, refetch]);

  if (loading) return <Page title="Deals"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const deals = data?.deals || [];
  const activeDeals = deals.filter(d => d.is_active && new Date(d.ends_at) > new Date());
  const expiredDeals = deals.filter(d => !d.is_active || new Date(d.ends_at) <= new Date());

  return (
    <Page title="Deals & Discounts" primaryAction={isSupplier ? { content: 'Create Deal', onAction: () => setCreateModal(true) } : undefined}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(false)}>Deal created!</Banner></Layout.Section>}

        <Layout.Section>
          <Text as="h2" variant="headingMd">{isSupplier ? 'Your Active Deals' : 'Available Deals'}</Text>
        </Layout.Section>

        <Layout.Section>
          {activeDeals.length > 0 ? (
            <InlineGrid columns={{ xs: 1, md: 2 }} gap="400">
              {activeDeals.map((deal) => (
                <Card key={deal.id}>
                  <BlockStack gap="300">
                    <InlineStack gap="300" blockAlign="center">
                      <div style={{ background: '#e3f1df', borderRadius: '10px', padding: '10px', display: 'flex' }}>
                        <Icon source={DiscountIcon} tone="success" />
                      </div>
                      <BlockStack gap="050">
                        <Text as="h3" variant="headingMd">{deal.title}</Text>
                        <Badge tone="success">Active</Badge>
                      </BlockStack>
                    </InlineStack>
                    <Divider />
                    <InlineStack align="space-between">
                      <Text as="span" variant="headingLg" tone="success">
                        {deal.discount_type === 'percentage' ? `${deal.discount_value}% OFF` : `$${deal.discount_value} OFF`}
                      </Text>
                      {deal.max_uses > 0 && (
                        <Text as="span" variant="bodySm" tone="subdued">{deal.current_uses}/{deal.max_uses} used</Text>
                      )}
                    </InlineStack>
                    <Text as="p" variant="bodySm" tone="subdued">
                      Ends {new Date(deal.ends_at).toLocaleDateString()}
                    </Text>
                  </BlockStack>
                </Card>
              ))}
            </InlineGrid>
          ) : (
            <Card><EmptyState heading="No active deals" image=""><p>{isSupplier ? 'Create a deal to offer discounts to resellers.' : 'No deals available from your suppliers right now.'}</p></EmptyState></Card>
          )}
        </Layout.Section>

        {expiredDeals.length > 0 && (
          <>
            <Layout.Section><Text as="h2" variant="headingMd" tone="subdued">Expired/Inactive</Text></Layout.Section>
            <Layout.Section>
              <Card>
                <BlockStack gap="200">
                  {expiredDeals.map((deal) => (
                    <InlineStack key={deal.id} align="space-between" blockAlign="center">
                      <Text as="span" variant="bodySm">{deal.title}</Text>
                      <InlineStack gap="200">
                        <Text as="span" variant="bodySm" tone="subdued">{deal.discount_type === 'percentage' ? `${deal.discount_value}%` : `$${deal.discount_value}`}</Text>
                        <Badge>Expired</Badge>
                      </InlineStack>
                    </InlineStack>
                  ))}
                </BlockStack>
              </Card>
            </Layout.Section>
          </>
        )}
      </Layout>

      {createModal && (
        <Modal open onClose={() => setCreateModal(false)} title="Create a Deal"
          primaryAction={{ content: 'Create', onAction: handleCreate, loading: creating, disabled: !title }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setCreateModal(false) }]}>
          <Modal.Section>
            <FormLayout>
              <TextField label="Deal Title" value={title} onChange={setTitle} autoComplete="off" placeholder="e.g. Spring Sale 20% Off" />
              <Select label="Discount Type" options={[{ label: 'Percentage', value: 'percentage' }, { label: 'Fixed Amount', value: 'fixed' }]} value={discountType} onChange={setDiscountType} />
              <TextField label="Discount Value" type="number" value={discountValue} onChange={setDiscountValue} suffix={discountType === 'percentage' ? '%' : '$'} autoComplete="off" />
              <TextField label="Valid for (days)" type="number" value={daysValid} onChange={setDaysValid} autoComplete="off" />
              <TextField label="Max uses (0 = unlimited)" type="number" value={maxUses} onChange={setMaxUses} autoComplete="off" />
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
