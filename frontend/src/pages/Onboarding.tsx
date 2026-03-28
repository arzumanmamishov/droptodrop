import { useState } from 'react';
import {
  Page,
  Layout,
  Card,
  Button,
  BlockStack,
  Text,
  InlineStack,
  Banner,
  Icon,
  Divider,
} from '@shopify/polaris';
import { PackageIcon, StoreIcon } from '@shopify/polaris-icons';
import { api } from '../utils/api';

interface OnboardingProps {
  onComplete: (role: string) => void;
}

export default function Onboarding({ onComplete }: OnboardingProps) {
  const [loading, setLoading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSelect = async (role: string) => {
    setLoading(role);
    setError(null);
    try {
      await api.post('/shop/role', { role });
      onComplete(role);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to set role');
    } finally {
      setLoading(null);
    }
  };

  return (
    <Page>
      <Layout>
        <Layout.Section>
          <div className="hero-gradient">
            <BlockStack gap="300" align="center">
              <div style={{ fontSize: '32px', fontWeight: 700, color: 'white' }}>
                Welcome to DropToDrop
              </div>
              <div style={{ fontSize: '16px', color: 'rgba(255,255,255,0.85)', maxWidth: '500px', margin: '0 auto' }}>
                The dropshipping network that connects suppliers with resellers.
                Choose your role to get started.
              </div>
            </BlockStack>
          </div>
        </Layout.Section>

        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <InlineStack gap="300" align="start" blockAlign="center">
                <div style={{ background: '#e3f1df', borderRadius: '12px', padding: '12px', display: 'flex' }}>
                  <Icon source={PackageIcon} tone="success" />
                </div>
                <BlockStack gap="100">
                  <Text as="h2" variant="headingLg">Supplier</Text>
                  <Text as="p" variant="bodySm" tone="subdued">List your products for resellers</Text>
                </BlockStack>
              </InlineStack>
              <Divider />
              <BlockStack gap="200">
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Publish products to the marketplace</Text>
                </InlineStack>
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Set wholesale pricing per variant</Text>
                </InlineStack>
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Receive and fulfill routed orders</Text>
                </InlineStack>
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Auto-sync inventory and tracking</Text>
                </InlineStack>
              </BlockStack>
              <Button
                variant="primary"
                size="large"
                fullWidth
                loading={loading === 'supplier'}
                onClick={() => handleSelect('supplier')}
              >
                Get Started as Supplier
              </Button>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <InlineStack gap="300" align="start" blockAlign="center">
                <div style={{ background: '#e0f0ff', borderRadius: '12px', padding: '12px', display: 'flex' }}>
                  <Icon source={StoreIcon} tone="info" />
                </div>
                <BlockStack gap="100">
                  <Text as="h2" variant="headingLg">Reseller</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Import and sell supplier products</Text>
                </BlockStack>
              </InlineStack>
              <Divider />
              <BlockStack gap="200">
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Browse the supplier marketplace</Text>
                </InlineStack>
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Import products with custom markup</Text>
                </InlineStack>
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Auto-route orders to suppliers</Text>
                </InlineStack>
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" tone="success">&#10003;</Text>
                  <Text as="span" variant="bodySm">Tracking synced to your customers</Text>
                </InlineStack>
              </BlockStack>
              <Button
                variant="primary"
                tone="success"
                size="large"
                fullWidth
                loading={loading === 'reseller'}
                onClick={() => handleSelect('reseller')}
              >
                Get Started as Reseller
              </Button>
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
