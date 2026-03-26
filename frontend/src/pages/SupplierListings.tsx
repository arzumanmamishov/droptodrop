import { useState, useCallback } from 'react';
import {
  Page,
  Layout,
  Card,
  DataTable,
  Badge,
  Button,
  Spinner,
  Banner,
  Filters,
  ChoiceList,
  BlockStack,
  Text,
  InlineStack,
  Thumbnail,
} from '@shopify/polaris';
import { ImageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { SupplierListing } from '../types';
import ProductPicker from '../components/ProductPicker';

interface ListingsResponse {
  listings: SupplierListing[];
  total: number;
}

export default function SupplierListings() {
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [page, setPage] = useState(0);
  const [pickerOpen, setPickerOpen] = useState(false);
  const limit = 20;

  const statusQuery = statusFilter.length === 1 ? `&status=${statusFilter[0]}` : '';
  const { data, loading, error, refetch } = useApi<ListingsResponse>(
    `/supplier/listings?limit=${limit}&offset=${page * limit}${statusQuery}`,
  );

  const handleStatusChange = useCallback(
    async (listingId: string, newStatus: string) => {
      try {
        await api.put(`/supplier/listings/${listingId}/status`, { status: newStatus });
        refetch();
      } catch {
        // Error handling
      }
    },
    [refetch],
  );

  if (loading) {
    return (
      <Page title="Supplier Listings">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      active: 'success',
      draft: 'info',
      paused: 'attention',
      archived: 'critical',
    };
    return <Badge tone={toneMap[status]}>{status}</Badge>;
  };

  const rows = (data?.listings || []).map((listing) => {
    const images = typeof listing.images === 'string' ? JSON.parse(listing.images || '[]') : (listing.images || []);
    const imgUrl = images[0]?.url || images[0]?.URL;
    return [
    <Thumbnail key={`img-${listing.id}`} source={imgUrl || ImageIcon} alt={listing.title} size="small" />,
    listing.title,
    String(listing.variants?.length || 0),
    statusBadge(listing.status),
    `${listing.processing_days}d`,
    new Date(listing.updated_at).toLocaleDateString(),
    <InlineStack gap="200" key={listing.id}>
      {listing.status === 'draft' && (
        <Button size="slim" onClick={() => handleStatusChange(listing.id, 'active')}>
          Publish
        </Button>
      )}
      {listing.status === 'active' && (
        <Button size="slim" onClick={() => handleStatusChange(listing.id, 'paused')}>
          Pause
        </Button>
      )}
      {listing.status === 'paused' && (
        <Button size="slim" onClick={() => handleStatusChange(listing.id, 'active')}>
          Resume
        </Button>
      )}
    </InlineStack>,
  ];});

  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page
      title="Supplier Listings"
      primaryAction={{ content: 'Add Products', onAction: () => setPickerOpen(true) }}
    >
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}
        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <Filters
                queryValue=""
                filters={[
                  {
                    key: 'status',
                    label: 'Status',
                    filter: (
                      <ChoiceList
                        title="Status"
                        titleHidden
                        choices={[
                          { label: 'Active', value: 'active' },
                          { label: 'Draft', value: 'draft' },
                          { label: 'Paused', value: 'paused' },
                          { label: 'Archived', value: 'archived' },
                        ]}
                        selected={statusFilter}
                        onChange={setStatusFilter}
                      />
                    ),
                    shortcut: true,
                  },
                ]}
                onQueryChange={() => {}}
                onQueryClear={() => {}}
                onClearAll={() => setStatusFilter([])}
              />

              {rows.length > 0 ? (
                <DataTable
                  columnContentTypes={['text', 'text', 'numeric', 'text', 'text', 'text', 'text']}
                  headings={['', 'Product', 'Variants', 'Status', 'Processing', 'Updated', 'Actions']}
                  rows={rows}
                />
              ) : (
                <Text as="p" tone="subdued">
                  No listings found. Create listings by selecting products from your Shopify store.
                </Text>
              )}

              {totalPages > 1 && (
                <InlineStack align="center" gap="200">
                  <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
                    Previous
                  </Button>
                  <Text as="span" variant="bodySm">
                    Page {page + 1} of {totalPages}
                  </Text>
                  <Button disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>
                    Next
                  </Button>
                </InlineStack>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>

      <ProductPicker
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        onImport={() => refetch()}
      />
    </Page>
  );
}
