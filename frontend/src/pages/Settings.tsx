import { useState, useEffect, useCallback } from 'react';
import {
  Page,
  Layout,
  Card,
  FormLayout,
  TextField,
  Checkbox,
  Banner,
  BlockStack,
  Spinner,
  Text,
} from '@shopify/polaris';
import { api } from '../utils/api';
import { AppSettings } from '../types';

export default function Settings() {
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const [notificationsEnabled, setNotificationsEnabled] = useState(true);
  const [notificationEmail, setNotificationEmail] = useState('');
  const [supportEmail, setSupportEmail] = useState('');
  const [privacyPolicyUrl, setPrivacyPolicyUrl] = useState('');
  const [termsUrl, setTermsUrl] = useState('');
  const [dataRetentionDays, setDataRetentionDays] = useState('365');

  useEffect(() => {
    api
      .get<AppSettings>('/settings')
      .then((data) => {
        setSettings(data);
        setNotificationsEnabled(data.notifications_enabled);
        setNotificationEmail(data.notification_email);
        setSupportEmail(data.support_email);
        setPrivacyPolicyUrl(data.privacy_policy_url);
        setTermsUrl(data.terms_url);
        setDataRetentionDays(String(data.data_retention_days));
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      await api.put('/settings', {
        notifications_enabled: notificationsEnabled,
        notification_email: notificationEmail,
        support_email: supportEmail,
        privacy_policy_url: privacyPolicyUrl,
        terms_url: termsUrl,
        data_retention_days: parseInt(dataRetentionDays, 10),
      });
      setSuccess(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  }, [notificationsEnabled, notificationEmail, supportEmail, privacyPolicyUrl, termsUrl, dataRetentionDays]);

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
            <Banner tone="success" onDismiss={() => setSuccess(false)}>Settings saved.</Banner>
          </Layout.Section>
        )}

        <Layout.AnnotatedSection
          title="Notifications"
          description="Configure how you receive notifications."
        >
          <Card>
            <FormLayout>
              <Checkbox
                label="Enable notifications"
                checked={notificationsEnabled}
                onChange={setNotificationsEnabled}
              />
              <TextField
                label="Notification email"
                type="email"
                value={notificationEmail}
                onChange={setNotificationEmail}
                autoComplete="email"
              />
            </FormLayout>
          </Card>
        </Layout.AnnotatedSection>

        <Layout.AnnotatedSection
          title="Support & Legal"
          description="Required for Shopify App Store listing."
        >
          <Card>
            <FormLayout>
              <TextField
                label="Support email"
                type="email"
                value={supportEmail}
                onChange={setSupportEmail}
                autoComplete="email"
                helpText="Displayed to merchants and required for app review."
              />
              <TextField
                label="Privacy policy URL"
                value={privacyPolicyUrl}
                onChange={setPrivacyPolicyUrl}
                autoComplete="url"
                helpText="Required for Shopify App Store submission."
              />
              <TextField
                label="Terms of service URL"
                value={termsUrl}
                onChange={setTermsUrl}
                autoComplete="url"
              />
            </FormLayout>
          </Card>
        </Layout.AnnotatedSection>

        <Layout.AnnotatedSection
          title="Data Retention"
          description="How long to keep historical data."
        >
          <Card>
            <FormLayout>
              <TextField
                label="Data retention (days)"
                type="number"
                value={dataRetentionDays}
                onChange={setDataRetentionDays}
                min={30}
                autoComplete="off"
              />
            </FormLayout>
          </Card>
        </Layout.AnnotatedSection>

        <Layout.AnnotatedSection
          title="Billing"
          description="Current plan and billing status."
        >
          <Card>
            <BlockStack gap="200">
              <Text as="p" variant="bodyMd">
                Current plan: <strong>{settings?.billing_plan || 'Free'}</strong>
              </Text>
              <Text as="p" variant="bodySm" tone="subdued">
                Billing integration is ready for Shopify Managed Pricing or Billing API activation.
              </Text>
            </BlockStack>
          </Card>
        </Layout.AnnotatedSection>
      </Layout>
    </Page>
  );
}
