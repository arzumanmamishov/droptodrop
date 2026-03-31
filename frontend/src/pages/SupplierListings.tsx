import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
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
  InlineGrid,
  Icon,
  EmptyState,
  TextField,
} from '@shopify/polaris';
import { ImageIcon, CheckIcon, ClockIcon, PauseCircleIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';
import { SupplierListing } from '../types';
import { getCategoryLabel } from '../constants/categories';
import ProductPicker from '../components/ProductPicker';
import ConfirmDialog from '../components/ConfirmDialog';

interface ListingsResponse {
  listings: SupplierListing[];
  total: number;
}

export default function SupplierListings() {
  const navigate = useNavigate();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [search, setSearch] = useState('');
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkAction, setBulkAction] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [confirmBulkDelete, setConfirmBulkDelete] = useState(false);
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

  const handleDelete = useCallback(
    async (listingId: string) => {
      try {
        await api.delete(`/supplier/listings/${listingId}`);
        refetch();
      } catch {
        // Error handling
      }
    },
    [refetch],
  );

  const handleBulkDelete = useCallback(async () => {
    setBulkAction(true);
    for (const id of selectedIds) {
      try { await api.delete(`/supplier/listings/${id}`); } catch { /* skip */ }
    }
    setSelectedIds(new Set());
    setBulkAction(false);
    refetch();
  }, [selectedIds, refetch]);

  const handleBulkPublish = useCallback(async () => {
    setBulkAction(true);
    for (const id of selectedIds) {
      try { await api.put(`/supplier/listings/${id}/status`, { status: 'active' }); } catch { /* skip */ }
    }
    setSelectedIds(new Set());
    setBulkAction(false);
    refetch();
  }, [selectedIds, refetch]);

  const handleBulkPause = useCallback(async () => {
    setBulkAction(true);
    for (const id of selectedIds) {
      try { await api.put(`/supplier/listings/${id}/status`, { status: 'paused' }); } catch { /* skip */ }
    }
    setSelectedIds(new Set());
    setBulkAction(false);
    refetch();
  }, [selectedIds, refetch]);

  if (loading) {
    return (
      <Page title="Supplier Listings">
        <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}>
          <Spinner size="large" />
        </div>
      </Page>
    );
  }

  const listings = data?.listings || [];
  const filteredListings = search
    ? listings.filter(l => l.title.toLowerCase().includes(search.toLowerCase()))
    : listings;

  const activeCount = listings.filter(l => l.status === 'active').length;
  const draftCount = listings.filter(l => l.status === 'draft').length;
  const pausedCount = listings.filter(l => l.status === 'paused').length;

  const statusBadge = (status: string) => {
    const toneMap: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      active: 'success', draft: 'info', paused: 'attention', archived: 'critical',
    };
    return <Badge tone={toneMap[status]}>{status}</Badge>;
  };

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === filteredListings.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(filteredListings.map(l => l.id)));
    }
  };

  const rows = filteredListings.map((listing) => {
    const images = typeof listing.images === 'string' ? JSON.parse(listing.images || '[]') : (listing.images || []);
    const imgUrl = images[0]?.url || images[0]?.URL;
    return [
      <input key={`cb-${listing.id}`} type="checkbox" checked={selectedIds.has(listing.id)} onChange={() => toggleSelect(listing.id)} />,
      <Thumbnail key={`img-${listing.id}`} source={imgUrl || ImageIcon} alt={listing.title} size="small" />,
      listing.title,
      <Badge key={`cat-${listing.id}`} tone="info">{getCategoryLabel(listing.category)}</Badge>,
      String(listing.variants?.length || 0),
      statusBadge(listing.status),
      `${listing.processing_days}d`,
      new Date(listing.updated_at).toLocaleDateString(),
      <InlineStack gap="200" key={listing.id}>
        <Button size="slim" onClick={() => navigate(`/supplier/listings/${listing.id}`)}>Edit</Button>
        {listing.status === 'draft' && (
          <Button size="slim" onClick={() => handleStatusChange(listing.id, 'active')}>Publish</Button>
        )}
        {listing.status === 'active' && (
          <Button size="slim" onClick={() => handleStatusChange(listing.id, 'paused')}>Pause</Button>
        )}
        {listing.status === 'paused' && (
          <Button size="slim" onClick={() => handleStatusChange(listing.id, 'active')}>Resume</Button>
        )}
        <Button size="slim" tone="critical" onClick={() => setConfirmDelete(listing.id)}>Delete</Button>
      </InlineStack>,
    ];
  });

  const totalPages = Math.ceil((data?.total || 0) / limit);

  return (
    <Page
      title="Supplier Listings"
      subtitle={`${data?.total || 0} products`}
      primaryAction={{ content: 'Add Products', onAction: () => setPickerOpen(true) }}
    >
      <Layout>
        {error && (
          <Layout.Section>
            <Banner tone="critical">{error}</Banner>
          </Layout.Section>
        )}

        <Layout.Section>
          <InlineGrid columns={{ xs: 1, md: 3 }} gap="400">
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#e3f1df', borderRadius: '8px', padding: '8px', display: 'flex' }}>
                  <Icon source={CheckIcon} />
                </div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{activeCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Active</Text>
                </BlockStack>
              </InlineStack>
            </Card>
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#e0f0ff', borderRadius: '8px', padding: '8px', display: 'flex' }}>
                  <Icon source={ClockIcon} />
                </div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{draftCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Draft</Text>
                </BlockStack>
              </InlineStack>
            </Card>
            <Card>
              <InlineStack gap="300" blockAlign="center">
                <div style={{ background: '#fef3cd', borderRadius: '8px', padding: '8px', display: 'flex' }}>
                  <Icon source={PauseCircleIcon} />
                </div>
                <BlockStack gap="050">
                  <Text as="p" variant="headingLg">{pausedCount}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">Paused</Text>
                </BlockStack>
              </InlineStack>
            </Card>
          </InlineGrid>
        </Layout.Section>

        <Layout.Section>
          <Card>
            <BlockStack gap="400">
              <InlineStack gap="400" align="space-between" blockAlign="end">
                <div style={{ flex: 1 }}>
                  <TextField
                    label=""
                    labelHidden
                    value={search}
                    onChange={setSearch}
                    placeholder="Search listings..."
                    autoComplete="off"
                    clearButton
                    onClearButtonClick={() => setSearch('')}
                  />
                </div>
                <Filters
                  queryValue=""
                  filters={[{
                    key: 'status',
                    label: 'Status',
                    filter: (
                      <ChoiceList
                        title="Status" titleHidden
                        choices={[
                          { label: 'Active', value: 'active' },
                          { label: 'Draft', value: 'draft' },
                          { label: 'Paused', value: 'paused' },
                        ]}
                        selected={statusFilter}
                        onChange={(v) => { setStatusFilter(v); setPage(0); }}
                      />
                    ),
                    shortcut: true,
                  }]}
                  onQueryChange={() => {}}
                  onQueryClear={() => {}}
                  onClearAll={() => setStatusFilter([])}
                />
              </InlineStack>

              {selectedIds.size > 0 && (
                <InlineStack gap="200">
                  <Text as="span" variant="bodySm">{selectedIds.size} selected</Text>
                  <Button size="slim" loading={bulkAction} onClick={handleBulkPublish}>Publish</Button>
                  <Button size="slim" loading={bulkAction} onClick={handleBulkPause}>Pause</Button>
                  <Button size="slim" tone="critical" loading={bulkAction} onClick={() => setConfirmBulkDelete(true)}>Delete</Button>
                </InlineStack>
              )}

              {rows.length > 0 ? (
                <>
                  <InlineStack gap="200">
                    <input type="checkbox" checked={selectedIds.size === filteredListings.length && filteredListings.length > 0} onChange={toggleSelectAll} />
                    <Text as="span" variant="bodySm" tone="subdued">Select all</Text>
                  </InlineStack>
                  <DataTable
                    columnContentTypes={['text', 'text', 'text', 'text', 'numeric', 'text', 'text', 'text', 'text']}
                    headings={['', '', 'Product', 'Category', 'Variants', 'Status', 'Processing', 'Updated', 'Actions']}
                    rows={rows}
                  />
                </>
              ) : (
                <EmptyState heading="No listings yet" image="">
                  <p>Add products from your Shopify store to list them for resellers.</p>
                </EmptyState>
              )}

              {totalPages > 1 && (
                <InlineStack align="center" gap="200">
                  <Button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>Previous</Button>
                  <Text as="span" variant="bodySm">Page {page + 1} of {totalPages}</Text>
                  <Button disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>Next</Button>
                </InlineStack>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>
      </Layout>

      <ProductPicker
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        onImport={() => { setPickerOpen(false); refetch(); }}
      />

      <ConfirmDialog
        open={confirmDelete !== null}
        title="Delete Listing"
        message="Are you sure you want to delete this listing? This action cannot be undone. Any resellers who imported this product will be notified."
        onConfirm={() => { if (confirmDelete) { handleDelete(confirmDelete); setConfirmDelete(null); } }}
        onCancel={() => setConfirmDelete(null)}
      />

      <ConfirmDialog
        open={confirmBulkDelete}
        title="Delete Selected Listings"
        message={`Are you sure you want to delete ${selectedIds.size} listing(s)? This action cannot be undone.`}
        onConfirm={() => { handleBulkDelete(); setConfirmBulkDelete(false); }}
        onCancel={() => setConfirmBulkDelete(false)}
      />
    </Page>
  );
}
