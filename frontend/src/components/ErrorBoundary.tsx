import { Component, ReactNode } from 'react';
import { Banner, Page, BlockStack, Text, Button } from '@shopify/polaris';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  render() {
    if (this.state.hasError) {
      return (
        <Page title="Something went wrong">
          <BlockStack gap="400">
            <Banner tone="critical">
              <p>An unexpected error occurred. Please try refreshing the page.</p>
            </Banner>
            <Text as="p" variant="bodySm" tone="subdued">
              {this.state.error?.message}
            </Text>
            <Button onClick={() => window.location.reload()}>Refresh Page</Button>
          </BlockStack>
        </Page>
      );
    }

    return this.props.children;
  }
}
