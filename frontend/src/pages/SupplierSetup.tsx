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
  Modal,
  Button,
  InlineStack,
} from '@shopify/polaris';
import { api } from '../utils/api';
import { useToast } from '../hooks/useToast';
import { SupplierProfile } from '../types';
import { COUNTRIES, COUNTRY_NAMES } from '../constants/countries';

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
  const [countryModal, setCountryModal] = useState(false);
  const [selectedCountries, setSelectedCountries] = useState<Set<string>>(new Set());
  const [countrySearch, setCountrySearch] = useState('');

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
        setSelectedCountries(new Set(data.shipping_countries || []));
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
              <BlockStack gap="200">
                <InlineStack align="space-between" blockAlign="center">
                  <span style={{ fontSize: '14px', fontWeight: 500 }}>Shipping countries</span>
                  <Button size="slim" onClick={() => setCountryModal(true)}>
                    {selectedCountries.size > 0 ? `Change (${selectedCountries.size})` : 'Select Countries'}
                  </Button>
                </InlineStack>
                {selectedCountries.size > 0 ? (
                  <InlineStack gap="200" wrap>
                    {Array.from(selectedCountries).map(c => (
                      <span key={c} style={{ padding: '2px 8px', borderRadius: '6px', fontSize: '12px', fontWeight: 600, background: '#dbeafe', color: '#1e40af' }}>
                        {COUNTRY_NAMES[c] || c}
                      </span>
                    ))}
                  </InlineStack>
                ) : (
                  <span style={{ fontSize: '13px', color: '#94a3b8' }}>Ships worldwide (no restrictions)</span>
                )}
              </BlockStack>
            </FormLayout>
          </Card>
        </Layout.AnnotatedSection>
      </Layout>

      {countryModal && (
        <Modal open onClose={() => setCountryModal(false)} title="Select Shipping Countries"
          primaryAction={{ content: selectedCountries.size > 0 ? `Save (${selectedCountries.size})` : 'Ship Worldwide', onAction: () => {
            setShippingCountries(Array.from(selectedCountries).join(', '));
            setCountryModal(false);
          }}}
          secondaryActions={[{ content: 'Cancel', onAction: () => setCountryModal(false) }]}
        >
          <Modal.Section>
            <BlockStack gap="300">
              <input type="text" placeholder="Search countries..." value={countrySearch}
                onChange={(e) => setCountrySearch(e.target.value)}
                style={{ width: '100%', padding: '8px 12px', border: '1px solid #e2e8f0', borderRadius: '8px', fontSize: '14px' }}
              />
              <InlineStack gap="200" wrap>
                <Button size="slim" onClick={() => setSelectedCountries(new Set(COUNTRIES))}>All</Button>
                <Button size="slim" onClick={() => setSelectedCountries(new Set())}>Clear</Button>
                <Button size="slim" onClick={() => setSelectedCountries(new Set(['DE','AT','CH','FR','IT','ES','NL','BE','PT','PL','CZ','SE','DK','NO','FI','IE','GB']))}>EU + UK</Button>
                <Button size="slim" onClick={() => setSelectedCountries(new Set(['US','CA','MX']))}>N. America</Button>
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
                      onChange={() => setSelectedCountries(prev => { const next = new Set(prev); if (next.has(code)) next.delete(code); else next.add(code); return next; })}
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
