import { useState, useCallback, useEffect } from 'react';
import {
  Page,
  Layout,
  Card,
  FormLayout,
  TextField,
  Checkbox,
  Select,
  Banner,
  BlockStack,
  Spinner,
} from '@shopify/polaris';
import { api } from '../utils/api';
import { useToast } from '../hooks/useToast';
import { SupplierProfile } from '../types';

export default function SupplierSetup() {
  const toast = useToast();
  const [, setProfile] = useState<SupplierProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const [isEnabled, setIsEnabled] = useState(false);
  const [processingDays, setProcessingDays] = useState('3');
  const [blindFulfillment, setBlindFulfillment] = useState(false);
  const [approvalMode, setApprovalMode] = useState('auto');
  const [companyName, setCompanyName] = useState('');
  const [supportEmail, setSupportEmail] = useState('');
  const [returnPolicyUrl, setReturnPolicyUrl] = useState('');
  const [paypalEmail, setPaypalEmail] = useState('');
  const [shippingCountries, setShippingCountries] = useState('');

  useEffect(() => {
    api
      .get<SupplierProfile>('/supplier/profile')
      .then((data) => {
        setProfile(data);
        setIsEnabled(data.is_enabled);
        setProcessingDays(String(data.default_processing_days));
        setBlindFulfillment(data.blind_fulfillment);
        setApprovalMode(data.reseller_approval_mode);
        setCompanyName(data.company_name);
        setSupportEmail(data.support_email);
        setReturnPolicyUrl(data.return_policy_url);
        setPaypalEmail(data.paypal_email || '');
        setShippingCountries((data.shipping_countries || []).join(', '));
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await api.put('/supplier/profile', {
        is_enabled: isEnabled,
        default_processing_days: parseInt(processingDays, 10),
        blind_fulfillment: blindFulfillment,
        reseller_approval_mode: approvalMode,
        company_name: companyName,
        support_email: supportEmail,
        return_policy_url: returnPolicyUrl,
        paypal_email: paypalEmail,
        shipping_countries: shippingCountries.split(',').map((s: string) => s.trim()).filter(Boolean),
      });
      toast.success('Profile saved');
      setSuccess(true);
    } catch (err) {
      toast.error('Failed to save profile');
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  }, [isEnabled, processingDays, blindFulfillment, approvalMode, companyName, supportEmail, returnPolicyUrl, paypalEmail, shippingCountries, toast]);

  if (loading) {
    return (
      <Page title="Supplier Setup">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  return (
    <Page
      title="Supplier Setup"
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
          title="Supplier Mode"
          description="Enable supplier mode to make your products available for resellers."
        >
          <Card>
            <BlockStack gap="400">
              <Checkbox
                label="Enable supplier mode"
                checked={isEnabled}
                onChange={setIsEnabled}
              />
            </BlockStack>
          </Card>
        </Layout.AnnotatedSection>

        <Layout.AnnotatedSection
          title="Processing & Shipping"
          description="Set default processing time and shipping preferences."
        >
          <Card>
            <FormLayout>
              <TextField
                label="Default processing time (days)"
                type="number"
                value={processingDays}
                onChange={setProcessingDays}
                min={0}
                autoComplete="off"
              />
              <Checkbox
                label="Blind/unbranded fulfillment"
                helpText="Ship without your branding so resellers can white-label."
                checked={blindFulfillment}
                onChange={setBlindFulfillment}
              />
            </FormLayout>
          </Card>
        </Layout.AnnotatedSection>

        <Layout.AnnotatedSection
          title="Reseller Approval"
          description="Control how resellers can access your products."
        >
          <Card>
            <FormLayout>
              <Select
                label="Approval mode"
                options={[
                  { label: 'Auto-approve all resellers', value: 'auto' },
                  { label: 'Manual approval required', value: 'manual' },
                ]}
                value={approvalMode}
                onChange={setApprovalMode}
              />
            </FormLayout>
          </Card>
        </Layout.AnnotatedSection>

        <Layout.AnnotatedSection
          title="Business Information"
          description="Contact details visible to resellers."
        >
          <Card>
            <FormLayout>
              <TextField label="Company name" value={companyName} onChange={setCompanyName} autoComplete="organization" />
              <TextField label="Support email" type="email" value={supportEmail} onChange={setSupportEmail} autoComplete="email" />
              <TextField label="Return policy URL" value={returnPolicyUrl} onChange={setReturnPolicyUrl} autoComplete="url" />
              <TextField label="PayPal email" type="email" value={paypalEmail} onChange={setPaypalEmail} autoComplete="email" helpText="Resellers will use this to send you payments via PayPal." />
              <TextField label="Shipping countries" value={shippingCountries} onChange={setShippingCountries} autoComplete="off" helpText="Comma-separated country codes (e.g. IT, ES, DE, FR). Leave empty to ship worldwide." placeholder="IT, ES, DE, FR" />
            </FormLayout>
          </Card>
        </Layout.AnnotatedSection>
      </Layout>
    </Page>
  );
}
