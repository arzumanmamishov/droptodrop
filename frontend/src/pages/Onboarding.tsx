import { useState } from 'react';
import {
  Page,
  Layout,
  Card,
  Text,
  Button,
  BlockStack,
  InlineStack,
  Banner,
  Box,
} from '@shopify/polaris';
import { api } from '../utils/api';

interface OnboardingProps {
  onComplete: (role: 'supplier' | 'reseller') => void;
}

export default function Onboarding({ onComplete }: OnboardingProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selectRole = async (role: 'supplier' | 'reseller') => {
    setLoading(true);
    setError(null);
    try {
      await api.post('/shop/role', { role });
      onComplete(role);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to set role');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Page title="Welcome to DropToDrop">
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}
        <Layout.Section>
          <BlockStack gap="400">
            <Text as="p" variant="bodyLg">
              Choose your role to get started. This determines how you use the app.
            </Text>
          </BlockStack>
        </Layout.Section>
        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingLg">Supplier</Text>
              <Text as="p" variant="bodyMd">
                You manufacture or source products and want to distribute them to resellers.
                Resellers will import your products and sell them to end customers.
              </Text>
              <Box>
                <BlockStack gap="200">
                  <Text as="p" variant="bodyMd">You will be able to:</Text>
                  <Text as="p" variant="bodySm">- List products for reseller distribution</Text>
                  <Text as="p" variant="bodySm">- Set wholesale pricing per variant</Text>
                  <Text as="p" variant="bodySm">- Receive and fulfill routed orders</Text>
                  <Text as="p" variant="bodySm">- Manage shipping and branding preferences</Text>
                </BlockStack>
              </Box>
              <InlineStack align="end">
                <Button variant="primary" loading={loading} onClick={() => selectRole('supplier')}>
                  I'm a Supplier
                </Button>
              </InlineStack>
            </BlockStack>
          </Card>
        </Layout.Section>
        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingLg">Reseller</Text>
              <Text as="p" variant="bodyMd">
                You run a store and want to sell products from suppliers without holding inventory.
                Orders are automatically routed to suppliers for fulfillment.
              </Text>
              <Box>
                <BlockStack gap="200">
                  <Text as="p" variant="bodyMd">You will be able to:</Text>
                  <Text as="p" variant="bodySm">- Browse supplier product marketplace</Text>
                  <Text as="p" variant="bodySm">- Import products with custom markup</Text>
                  <Text as="p" variant="bodySm">- Auto-route orders to suppliers</Text>
                  <Text as="p" variant="bodySm">- Track fulfillment and sync tracking</Text>
                </BlockStack>
              </Box>
              <InlineStack align="end">
                <Button variant="primary" loading={loading} onClick={() => selectRole('reseller')}>
                  I'm a Reseller
                </Button>
              </InlineStack>
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
