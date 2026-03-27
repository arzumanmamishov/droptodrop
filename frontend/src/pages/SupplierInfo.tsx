import { useParams, useNavigate } from 'react-router-dom';
import {
  Page,
  Layout,
  Card,
  BlockStack,
  Text,
  Badge,
  Spinner,
  Banner,
  InlineStack,
  Divider,
  Icon,
  Button,
} from '@shopify/polaris';
import { StoreIcon, EmailIcon, PackageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';

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
  const { data, loading, error } = useApi<SupplierInfoData>(`/reseller/suppliers/${id}`);

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
      <Page title="Supplier Profile" backAction={{ content: 'Marketplace', onAction: () => navigate('/marketplace') }}>
        <Banner tone="critical">{error || 'Supplier not found'}</Banner>
      </Page>
    );
  }

  return (
    <Page
      title={data.company_name || 'Supplier Profile'}
      backAction={{ content: 'Marketplace', onAction: () => navigate('/marketplace') }}
    >
      <Layout>
        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#e3f1df', borderRadius: '12px', padding: '12px', display: 'flex' }}>
                  <Icon source={StoreIcon} tone="success" />
                </div>
                <BlockStack gap="100">
                  <Text as="h2" variant="headingLg">{data.company_name || 'Unnamed Supplier'}</Text>
                  <Badge tone="success">{data.listing_count} products listed</Badge>
                </BlockStack>
              </InlineStack>
              <Divider />
              <BlockStack gap="200">
                {data.support_email && (
                  <InlineStack gap="200" blockAlign="center">
                    <Icon source={EmailIcon} tone="subdued" />
                    <Text as="p" variant="bodyMd">{data.support_email}</Text>
                  </InlineStack>
                )}
                <InlineStack gap="200" blockAlign="center">
                  <Icon source={PackageIcon} tone="subdued" />
                  <Text as="p" variant="bodyMd">{data.default_processing_days} days processing time</Text>
                </InlineStack>
                {data.blind_fulfillment && (
                  <Badge tone="info">Blind/white-label fulfillment available</Badge>
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
        </Layout.Section>
      </Layout>
    </Page>
  );
}
