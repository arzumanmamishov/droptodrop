import { Page, Layout, Card, BlockStack, Text, Divider, InlineStack } from '@shopify/polaris';

export default function Support() {
  return (
    <Page title="Help & Support">
      <Layout>
        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Frequently Asked Questions</Text>
              <Divider />

              <BlockStack gap="300">
                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">How do I import products?</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Go to Marketplace, find a product, click "Import". Set your markup (minimum 30%) and select which variants you want.</Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">How does inventory work?</Text>
                  <Text as="p" variant="bodySm" tone="subdued">All resellers share the same inventory pool. When one reseller sells a product, stock updates for everyone automatically.</Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">How do payments work?</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Resellers pay suppliers directly via PayPal. Go to Payouts page to see what you owe or are owed. Add your PayPal email in Settings.</Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">How do returns work?</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Resellers can request a return on fulfilled orders. The supplier uploads a return shipping label, and the customer ships back to the supplier directly.</Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">What is the minimum markup?</Text>
                  <Text as="p" variant="bodySm" tone="subdued">We enforce a minimum 30% markup to protect reseller margins. We recommend 40%+ to cover shipping costs.</Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">Can I choose which product variants to import?</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Yes! When importing, you can select specific variants (colors, sizes) using checkboxes. You don't have to import all variants.</Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">What happens if a supplier removes a product?</Text>
                  <Text as="p" variant="bodySm" tone="subdued">You'll get a notification warning you in advance. Suppliers cannot remove products while orders are in progress.</Text>
                </BlockStack>
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Contact Support</Text>
              <Divider />
              <Text as="p" variant="bodyMd">Need help? Reach out to us:</Text>
              <BlockStack gap="200">
                <InlineStack gap="200" blockAlign="center">
                  <span style={{ fontSize: '16px' }}>📧</span>
                  <Text as="p" variant="bodyMd">support@droptodrop.com</Text>
                </InlineStack>
                <InlineStack gap="200" blockAlign="center">
                  <span style={{ fontSize: '16px' }}>💬</span>
                  <Text as="p" variant="bodyMd">Use the Messages feature to contact suppliers directly</Text>
                </InlineStack>
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Text as="h2" variant="headingMd">Policies</Text>
              <Divider />

              <BlockStack gap="300">
                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">Chargeback Policy</Text>
                  <Text as="p" variant="bodySm" tone="subdued">
                    If a customer disputes a charge, the reseller handles the chargeback with their payment processor.
                    If the chargeback is due to supplier error (wrong item, not shipped), the supplier is responsible —
                    use the Disputes feature to resolve. Supplier reliability scores are affected by chargebacks.
                  </Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">Refund Policy</Text>
                  <Text as="p" variant="bodySm" tone="subdued">
                    Returns go through the supplier: reseller requests return → supplier uploads return label →
                    customer ships to supplier → supplier confirms receipt → reseller processes refund.
                    Since payments are via PayPal, there are no double refund fees like other platforms.
                  </Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">Margin Protection</Text>
                  <Text as="p" variant="bodySm" tone="subdued">
                    We enforce a minimum 30% markup on all imports to protect resellers from unsustainable margins.
                    Always factor in shipping costs — we recommend 40%+ markup. Use the profit calculator during import.
                  </Text>
                </BlockStack>

                <BlockStack gap="100">
                  <Text as="h3" variant="headingSm">Supplier Disconnection</Text>
                  <Text as="p" variant="bodySm" tone="subdued">
                    Suppliers cannot remove listings while orders are in progress. When a supplier pauses a product,
                    all resellers receive a warning notification with time to prepare.
                  </Text>
                </BlockStack>
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section variant="oneHalf">
          <Card>
            <BlockStack gap="300">
              <Text as="h2" variant="headingMd">Quick Tips</Text>
              <Divider />
              <BlockStack gap="200">
                <Text as="p" variant="bodySm">• Set markup to at least 40% to cover shipping</Text>
                <Text as="p" variant="bodySm">• Add your PayPal email in Settings for payments</Text>
                <Text as="p" variant="bodySm">• Use Re-sync to update products after supplier changes</Text>
                <Text as="p" variant="bodySm">• Check Notifications for order updates</Text>
                <Text as="p" variant="bodySm">• Use Disputes for quality issues or problems</Text>
              </BlockStack>
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
