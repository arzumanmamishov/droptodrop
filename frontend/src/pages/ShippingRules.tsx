import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, Spinner,
  BlockStack, Modal, FormLayout, TextField,
  EmptyState, InlineStack, Text, Button,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';
import { COUNTRY_NAMES } from '../constants/countries';
import ConfirmDialog from '../components/ConfirmDialog';

interface ShippingRule {
  id: string; country_code: string; shipping_rate: number;
  free_shipping_threshold: number | null; estimated_days_min: number;
  estimated_days_max: number; is_active: boolean;
}

export default function ShippingRules() {
  const toast = useToast();
  const { data, loading, refetch } = useApi<{ rules: ShippingRule[] }>('/shipping-rules');
  const [modal, setModal] = useState<'add' | 'edit' | null>(null);
  const [editId, setEditId] = useState('');
  const [country, setCountry] = useState('');
  const [rate, setRate] = useState('5.00');
  const [freeThreshold, setFreeThreshold] = useState('');
  const [daysMin, setDaysMin] = useState('3');
  const [daysMax, setDaysMax] = useState('7');
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const openAdd = () => {
    setModal('add'); setEditId(''); setCountry(''); setRate('5.00'); setFreeThreshold(''); setDaysMin('3'); setDaysMax('7');
  };

  const openEdit = (r: ShippingRule) => {
    setModal('edit'); setEditId(r.id); setCountry(r.country_code);
    setRate(r.shipping_rate.toFixed(2));
    setFreeThreshold(r.free_shipping_threshold ? r.free_shipping_threshold.toFixed(2) : '');
    setDaysMin(String(r.estimated_days_min)); setDaysMax(String(r.estimated_days_max));
  };

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      const body = {
        country_code: country.toUpperCase(),
        shipping_rate: parseFloat(rate) || 0,
        free_shipping_threshold: freeThreshold ? parseFloat(freeThreshold) : null,
        estimated_days_min: parseInt(daysMin) || 3,
        estimated_days_max: parseInt(daysMax) || 7,
      };
      if (modal === 'edit') {
        await api.put(`/shipping-rules/${editId}`, body);
        toast.success('Rule updated');
      } else {
        await api.post('/shipping-rules', body);
        toast.success('Rule added');
      }
      setModal(null); refetch();
    } catch { toast.error('Failed to save rule'); }
    finally { setSaving(false); }
  }, [country, rate, freeThreshold, daysMin, daysMax, modal, editId, refetch, toast]);

  const handleDelete = useCallback(async (id: string) => {
    try {
      await api.delete(`/shipping-rules/${id}`);
      toast.success('Rule deleted');
      refetch();
    } catch { toast.error('Failed to delete rule'); }
  }, [refetch, toast]);

  if (loading) return <Page title="Shipping Rules"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const rules = data?.rules || [];

  return (
    <Page title="Shipping Rules" subtitle={`${rules.length} rule${rules.length !== 1 ? 's' : ''}`} primaryAction={{ content: '+ Add Rule', onAction: openAdd }}>
      <Layout>
        <Layout.Section>
          {rules.length > 0 ? (
            <BlockStack gap="300">
              {rules.map(r => (
                <Card key={r.id}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <InlineStack gap="400" blockAlign="center" wrap>
                      <div style={{
                        width: '44px', height: '44px', borderRadius: '12px', background: '#dbeafe',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontSize: '16px', fontWeight: 700, color: '#1e40af', flexShrink: 0,
                      }}>
                        {r.country_code}
                      </div>
                      <BlockStack gap="050">
                        <Text as="span" variant="bodyMd" fontWeight="semibold">
                          {COUNTRY_NAMES[r.country_code] || r.country_code}
                        </Text>
                        <InlineStack gap="200" wrap>
                          <span style={{ fontSize: '12px', padding: '2px 8px', borderRadius: '6px', fontWeight: 600, background: '#dcfce7', color: '#166534' }}>
                            ${r.shipping_rate.toFixed(2)}
                          </span>
                          {r.free_shipping_threshold && (
                            <span style={{ fontSize: '12px', padding: '2px 8px', borderRadius: '6px', fontWeight: 600, background: '#dbeafe', color: '#1e40af' }}>
                              Free over ${r.free_shipping_threshold.toFixed(2)}
                            </span>
                          )}
                          <span style={{ fontSize: '12px', color: '#94a3b8' }}>
                            {r.estimated_days_min}–{r.estimated_days_max} days
                          </span>
                        </InlineStack>
                      </BlockStack>
                    </InlineStack>
                    <InlineStack gap="200">
                      <Button size="slim" onClick={() => openEdit(r)}>Edit</Button>
                      <button
                        onClick={() => setConfirmDelete(r.id)}
                        style={{
                          padding: '4px 14px', fontSize: '13px', fontWeight: 600,
                          background: '#fee2e2', color: '#dc2626', border: '1px solid #fca5a5',
                          borderRadius: '8px', cursor: 'pointer',
                        }}
                      >Delete</button>
                    </InlineStack>
                  </div>
                </Card>
              ))}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading="No shipping rules" image="">
                <p>Add shipping rules to set rates per country for your resellers.</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>

      {modal && (
        <Modal open onClose={() => setModal(null)} title={modal === 'edit' ? 'Edit Shipping Rule' : 'Add Shipping Rule'}
          primaryAction={{ content: 'Save', onAction: handleSave, loading: saving, disabled: !country }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setModal(null) }]}
        >
          <Modal.Section>
            <FormLayout>
              <TextField label="Country Code" value={country} onChange={setCountry} autoComplete="off" maxLength={10} placeholder="US, DE, GB" />
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

      <ConfirmDialog
        open={confirmDelete !== null}
        title="Delete Shipping Rule"
        message="Are you sure you want to delete this shipping rule?"
        confirmLabel="Delete"
        destructive
        onConfirm={() => { if (confirmDelete) { handleDelete(confirmDelete); setConfirmDelete(null); } }}
        onCancel={() => setConfirmDelete(null)}
      />
    </Page>
  );
}
