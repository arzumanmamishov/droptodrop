import { useState } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Button, InlineStack,
  Icon, Divider, Badge, Banner,
} from '@shopify/polaris';
import { CheckIcon } from '@shopify/polaris-icons';

interface Props {
  role: string;
  onComplete: () => void;
}

const SUPPLIER_STEPS = [
  { id: 'profile', title: 'Set Up Profile', desc: 'Add your company name and contact info', link: '/supplier/setup' },
  { id: 'shipping', title: 'Add Shipping Rules', desc: 'Set shipping rates per country', link: '/shipping-rules' },
  { id: 'products', title: 'Add Products', desc: 'Import products from your Shopify store', link: '/supplier/listings' },
  { id: 'publish', title: 'Publish Listings', desc: 'Make products available to resellers', link: '/supplier/listings' },
];

const RESELLER_STEPS = [
  { id: 'browse', title: 'Browse Marketplace', desc: 'Discover products from suppliers', link: '/marketplace' },
  { id: 'import', title: 'Import Products', desc: 'Add supplier products to your store', link: '/marketplace' },
  { id: 'pricing', title: 'Set Your Prices', desc: 'Configure markup and margins', link: '/imports' },
  { id: 'subscribe', title: 'Choose a Plan', desc: 'Select billing plan for your business', link: '/billing' },
];

export default function SetupWizard({ role, onComplete }: Props) {
  const steps = role === 'supplier' ? SUPPLIER_STEPS : RESELLER_STEPS;
  const [completedSteps, setCompletedSteps] = useState<Set<string>>(new Set());
  const progress = (completedSteps.size / steps.length) * 100;

  const markComplete = (stepId: string) => {
    setCompletedSteps(prev => {
      const next = new Set(prev);
      next.add(stepId);
      return next;
    });
  };

  return (
    <Page title="Quick Setup">
      <Layout>
        <Layout.Section>
          <div style={{
            background: 'linear-gradient(135deg, #2d6a4f 0%, #1b4332 100%)',
            borderRadius: '16px', padding: '32px', color: 'white',
          }}>
            <BlockStack gap="300">
              <Text as="h2" variant="headingLg">
                <span style={{ color: 'white' }}>
                  {progress === 100 ? 'Setup Complete!' : `Let's get you started`}
                </span>
              </Text>
              <Text as="p" variant="bodyMd">
                <span style={{ color: 'rgba(255,255,255,0.8)' }}>
                  {progress === 100
                    ? 'Your store is ready. Start selling!'
                    : `Complete these steps to set up your ${role} account.`}
                </span>
              </Text>
              <div style={{ background: 'rgba(255,255,255,0.3)', borderRadius: '8px', overflow: 'hidden' }}>
                <div style={{
                  width: `${progress}%`, height: '8px',
                  background: 'white', borderRadius: '8px',
                  transition: 'width 0.5s ease',
                }} />
              </div>
              <Text as="p" variant="bodySm">
                <span style={{ color: 'rgba(255,255,255,0.7)' }}>{completedSteps.size} of {steps.length} steps completed</span>
              </Text>
            </BlockStack>
          </div>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="0">
              {steps.map((step, i) => {
                const isComplete = completedSteps.has(step.id);
                return (
                  <div key={step.id}>
                    <div style={{ padding: '16px', background: isComplete ? '#f0fdf4' : 'transparent' }}>
                      <InlineStack align="space-between" blockAlign="center">
                        <InlineStack gap="400" blockAlign="center">
                          <div style={{
                            width: '36px', height: '36px', borderRadius: '50%',
                            background: isComplete ? '#008060' : '#f6f6f7',
                            display: 'flex', alignItems: 'center', justifyContent: 'center',
                            color: isComplete ? 'white' : '#6d7175', fontWeight: 600,
                          }}>
                            {isComplete ? <Icon source={CheckIcon} tone="base" /> : i + 1}
                          </div>
                          <BlockStack gap="050">
                            <Text as="span" variant="bodyMd" fontWeight="semibold">{step.title}</Text>
                            <Text as="span" variant="bodySm" tone="subdued">{step.desc}</Text>
                          </BlockStack>
                        </InlineStack>
                        {isComplete ? (
                          <Badge tone="success">Done</Badge>
                        ) : (
                          <Button size="slim" url={step.link} onClick={() => markComplete(step.id)}>
                            Start
                          </Button>
                        )}
                      </InlineStack>
                    </div>
                    {i < steps.length - 1 && <Divider />}
                  </div>
                );
              })}
            </BlockStack>
          </Card>
        </Layout.Section>

        {progress === 100 && (
          <Layout.Section>
            <Banner tone="success" action={{ content: 'Go to Dashboard', onAction: onComplete }}>
              You're all set! Your store is ready to go.
            </Banner>
          </Layout.Section>
        )}
      </Layout>
    </Page>
  );
}
