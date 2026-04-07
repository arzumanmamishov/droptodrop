import { useParams, useNavigate } from 'react-router-dom';
import { useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Spinner,
  Banner, InlineStack, Divider, Icon, Button,
} from '@shopify/polaris';
import { EmailIcon, PackageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';

interface SupplierInfoData {
  company_name: string;
  support_email: string;
  return_policy_url: string;
  default_processing_days: number;
  blind_fulfillment: boolean;
  listing_count: number;
}

export default function SupplierInfo() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();
  const { data, loading, error } = useApi<SupplierInfoData>(`/reseller/suppliers/${id}`);

  const handleMessage = useCallback(async () => {
    try {
      const conv = await api.post<{ id: string }>('/conversations', { other_shop_id: id, subject: 'Inquiry' });
      navigate(`/messages?conv=${conv.id}`);
    } catch (err) {
      toast.error('Failed to start conversation');
      navigate('/messages');
    }
  }, [id, navigate, toast]);

  if (loading) {
    return (
      <Page title="Supplier Profile">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  if (error || !data) {
    return (
      <Page title="Supplier Profile" backAction={{ content: 'Back', onAction: () => navigate(-1 as unknown as string) }}>
        <Banner tone="critical">{error || 'Supplier not found'}</Banner>
      </Page>
    );
  }

  const name = data.company_name || 'Supplier';

  return (
    <Page
      title={name}
      backAction={{ content: 'Back', onAction: () => navigate(-1 as unknown as string) }}
      primaryAction={{ content: 'Message Supplier', onAction: handleMessage }}
    >
      <Layout>
        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <InlineStack gap="300" blockAlign="center">
                <div style={{
                  width: '56px', height: '56px', borderRadius: '14px',
                  background: 'linear-gradient(135deg, #1e40af, #3b82f6)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  color: 'white', fontSize: '22px', fontWeight: 700,
                }}>
                  {name.charAt(0).toUpperCase()}
                </div>
                <BlockStack gap="100">
                  <Text as="h2" variant="headingLg">{name}</Text>
                  <InlineStack gap="200">
                    <Badge tone="info">{`${data.listing_count} product${data.listing_count !== 1 ? 's' : ''}`}</Badge>
                    {data.blind_fulfillment && <Badge tone="success">White-label</Badge>}
                  </InlineStack>
                </BlockStack>
              </InlineStack>

              <Divider />

              <BlockStack gap="300">
                <InlineStack gap="200" blockAlign="center">
                  <Icon source={PackageIcon} tone="subdued" />
                  <Text as="p" variant="bodyMd">
                    <Text as="span" fontWeight="semibold">{data.default_processing_days} day{data.default_processing_days !== 1 ? 's' : ''}</Text> processing time
                  </Text>
                </InlineStack>

                {data.support_email && (
                  <InlineStack gap="200" blockAlign="center">
                    <Icon source={EmailIcon} tone="subdued" />
                    <Text as="p" variant="bodyMd">{data.support_email}</Text>
                  </InlineStack>
                )}

                {!data.support_email && (
                  <InlineStack gap="200" blockAlign="center">
                    <Icon source={EmailIcon} tone="subdued" />
                    <Text as="p" variant="bodySm" tone="subdued">No support email provided</Text>
                  </InlineStack>
                )}
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Policies</Text>
              <Divider />
              {data.return_policy_url ? (
                <Button variant="plain" url={data.return_policy_url} external>
                  View Return Policy
                </Button>
              ) : (
                <Text as="p" tone="subdued">No return policy URL provided</Text>
              )}
            </BlockStack>
          </Card>

          <div style={{ marginTop: '16px' }}>
            <Card>
              <BlockStack gap="300">
                <Text as="h2" variant="headingMd">Contact</Text>
                <Divider />
                <Button variant="primary" onClick={handleMessage}>
                  Send Message
                </Button>
              </BlockStack>
            </Card>
          </div>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
