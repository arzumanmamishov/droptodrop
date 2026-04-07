import { useState, useCallback, useEffect } from 'react';
import {
  Page, Layout, Card, FormLayout, TextField, Banner, BlockStack, Spinner,
} from '@shopify/polaris';
import { api } from '../utils/api';
import { useToast } from '../hooks/useToast';

interface ResellerProfile {
  paypal_email: string;
}

export default function ResellerSettings() {
  const toast = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [paypalEmail, setPaypalEmail] = useState('');

  useEffect(() => {
    api.get<ResellerProfile>('/reseller/profile')
      .then((data) => {
        setPaypalEmail(data.paypal_email || '');
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await api.put('/reseller/profile', { paypal_email: paypalEmail });
      toast.success('Settings saved');
      setSuccess(true);
    } catch (err) {
      toast.error('Failed to save settings');
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  }, [paypalEmail, toast]);

  if (loading) {
    return (
      <Page title="Settings">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  return (
    <Page
      title="Settings"
      primaryAction={{ content: 'Save', onAction: handleSave, loading: saving }}
    >
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical" onDismiss={() => setError(null)}>{error}</Banner>
          </Layout.Section>
        )}
        {success && (
          <Layout.Section>
            <Banner tone="success" onDismiss={() => setSuccess(false)}>Settings saved successfully.</Banner>
          </Layout.Section>
        )}

        <Layout.AnnotatedSection
          title="Payment Information"
          description="Add your PayPal email so suppliers can identify your payments."
        >
          <Card>
            <BlockStack gap="400">
              <FormLayout>
                <TextField
                  label="PayPal email"
                  type="email"
                  value={paypalEmail}
                  onChange={setPaypalEmail}
                  autoComplete="email"
                  helpText="This email will be shared with suppliers for payment verification."
                />
              </FormLayout>
            </BlockStack>
          </Card>
        </Layout.AnnotatedSection>
      </Layout>
    </Page>
  );
}
