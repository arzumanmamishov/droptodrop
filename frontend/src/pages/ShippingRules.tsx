import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, Spinner,
  Banner, InlineStack, Modal, FormLayout, TextField,
  EmptyState, DataTable,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';

interface ShippingRule {
  id: string; country_code: string; shipping_rate: number;
  free_shipping_threshold: number | null; estimated_days_min: number;
  estimated_days_max: number; is_active: boolean;
}

export default function ShippingRules() {
  const toast = useToast();
  const { data, loading, refetch } = useApi<{ rules: ShippingRule[] }>('/shipping-rules');
  const [addModal, setAddModal] = useState(false);
  const [country, setCountry] = useState('');
  const [rate, setRate] = useState('5.00');
  const [freeThreshold, setFreeThreshold] = useState('');
  const [daysMin, setDaysMin] = useState('3');
  const [daysMax, setDaysMax] = useState('7');
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState(false);

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      await api.post('/shipping-rules', {
        country_code: country.toUpperCase(),
        shipping_rate: parseFloat(rate) || 0,
        free_shipping_threshold: freeThreshold ? parseFloat(freeThreshold) : null,
        estimated_days_min: parseInt(daysMin) || 3,
        estimated_days_max: parseInt(daysMax) || 7,
      });
      setSuccess(true);
      setAddModal(false);
      setCountry(''); setRate('5.00'); setFreeThreshold('');
      refetch();
    } catch { toast.error('Failed to save shipping rule'); }
    finally { setSaving(false); }
  }, [country, rate, freeThreshold, daysMin, daysMax, refetch, toast]);

  if (loading) return <Page title="Shipping Rules"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const rules = data?.rules || [];

  return (
    <Page title="Shipping Rules" subtitle="Set shipping rates per country" primaryAction={{ content: 'Add Rule', onAction: () => setAddModal(true) }}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(false)}>Shipping rule saved!</Banner></Layout.Section>}
        <Layout.Section>
          <Card>
            {rules.length > 0 ? (
              <DataTable
                columnContentTypes={['text', 'numeric', 'numeric', 'text']}
                headings={['Country', 'Rate', 'Free Shipping Above', 'Delivery Estimate']}
                rows={rules.map(r => [
                  r.country_code,
                  `$${r.shipping_rate.toFixed(2)}`,
                  r.free_shipping_threshold ? `$${r.free_shipping_threshold.toFixed(2)}` : '-',
                  `${r.estimated_days_min}-${r.estimated_days_max} days`,
                ])}
              />
            ) : (
              <EmptyState heading="No shipping rules" image="">
                <p>Add shipping rules to set rates per country for your resellers.</p>
              </EmptyState>
            )}
          </Card>
        </Layout.Section>
      </Layout>

      {addModal && (
        <Modal open onClose={() => setAddModal(false)} title="Add Shipping Rule"
          primaryAction={{ content: 'Save', onAction: handleSave, loading: saving, disabled: !country }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setAddModal(false) }]}>
          <Modal.Section>
            <FormLayout>
              <TextField label="Country Code (e.g. US, DE, GB)" value={country} onChange={setCountry} autoComplete="off" maxLength={10} />
              <TextField label="Shipping Rate" type="number" value={rate} onChange={setRate} prefix="$" autoComplete="off" />
              <TextField label="Free Shipping Threshold (optional)" type="number" value={freeThreshold} onChange={setFreeThreshold} prefix="$" autoComplete="off" helpText="Orders above this amount get free shipping" />
              <InlineStack gap="200">
                <TextField label="Min Days" type="number" value={daysMin} onChange={setDaysMin} autoComplete="off" />
                <TextField label="Max Days" type="number" value={daysMax} onChange={setDaysMax} autoComplete="off" />
              </InlineStack>
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
