import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Page, Layout, Card, Badge, Button, Spinner, Banner,
  BlockStack, Text, InlineStack, Thumbnail, TextField,
  EmptyState, Divider,
} from '@shopify/polaris';
import { ImageIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
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
  const toast = useToast();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [confirmBulkDelete, setConfirmBulkDelete] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkLoading, setBulkLoading] = useState(false);

  const { data, loading, error, refetch } = useApi<ListingsResponse>(
    '/supplier/listings?limit=100&offset=0',
  );

  const handleStatusChange = useCallback(async (id: string, status: string) => {
    try {
      await api.put(`/supplier/listings/${id}/status`, { status });
      toast.success(`Listing ${status === 'active' ? 'published' : 'paused'}`);
      refetch();
    } catch { toast.error('Failed to update listing status'); }
  }, [refetch, toast]);

  const handleDelete = useCallback(async (id: string) => {
    try {
      await api.delete(`/supplier/listings/${id}`);
      toast.success('Listing deleted');
      refetch();
    } catch { toast.error('Failed to delete listing'); }
  }, [refetch, toast]);

  const handleBulkAction = useCallback(async (action: 'active' | 'paused' | 'delete') => {
    setBulkLoading(true);
    for (const id of selectedIds) {
      try {
        if (action === 'delete') {
          await api.delete(`/supplier/listings/${id}`);
        } else {
          await api.put(`/supplier/listings/${id}/status`, { status: action });
        }
      } catch { toast.error(`Failed to ${action} listing`); }
    }
    const actionLabel = action === 'delete' ? 'deleted' : action === 'active' ? 'published' : 'paused';
    toast.success(`${selectedIds.size} listing(s) ${actionLabel}`);
    setSelectedIds(new Set());
    setBulkLoading(false);
    refetch();
  }, [selectedIds, refetch, toast]);

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

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
  const filtered = search
    ? listings.filter(l => l.title.toLowerCase().includes(search.toLowerCase()))
    : listings;

  const activeCount = listings.filter(l => l.status === 'active').length;
  const draftCount = listings.filter(l => l.status === 'draft').length;
  const pausedCount = listings.filter(l => l.status === 'paused').length;

  const getImage = (listing: SupplierListing): string | null => {
    try {
      const imgs = typeof listing.images === 'string' ? JSON.parse(listing.images || '[]') : (listing.images || []);
      return imgs[0]?.url || imgs[0]?.URL || null;
    } catch { return null; }
  };

  const statusTone = (s: string): 'success' | 'attention' | 'info' | 'critical' => {
    const map: Record<string, 'success' | 'attention' | 'info' | 'critical'> = {
      active: 'success', draft: 'info', paused: 'attention', archived: 'critical',
    };
    return map[s] || 'info';
  };

  return (
    <Page
      title="Listings"
      subtitle={`${listings.length} products`}
      primaryAction={{ content: 'Add Products', onAction: () => setPickerOpen(true) }}
    >
      <Layout>
        {error && <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>}

        {/* Stats */}
        <Layout.Section>
          <InlineStack gap="300">
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">{activeCount}</div>
              <div className="stat-card-label">Active</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">{draftCount}</div>
              <div className="stat-card-label">Draft</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">{pausedCount}</div>
              <div className="stat-card-label">Paused</div>
            </div>
          </InlineStack>
        </Layout.Section>

        {/* Search + Bulk Actions */}
        <Layout.Section>
          <Card>
            <BlockStack gap="300">
              <TextField
                label="" labelHidden value={search} onChange={setSearch}
                placeholder="Search listings..." autoComplete="off"
                clearButton onClearButtonClick={() => setSearch('')}
              />
              {selectedIds.size > 0 && (
                <InlineStack gap="200" blockAlign="center">
                  <Text as="span" variant="bodySm" fontWeight="semibold">{selectedIds.size} selected</Text>
                  <Button size="slim" loading={bulkLoading} onClick={() => handleBulkAction('active')}>Publish</Button>
                  <Button size="slim" loading={bulkLoading} onClick={() => handleBulkAction('paused')}>Pause</Button>
                  <Button size="slim" tone="critical" loading={bulkLoading} onClick={() => setConfirmBulkDelete(true)}>Delete</Button>
                </InlineStack>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>

        {/* Product List */}
        <Layout.Section>
          {filtered.length > 0 ? (
            <Card>
              <BlockStack gap="0">
                {filtered.map((listing, i) => {
                  const imgUrl = getImage(listing);
                  const isSelected = selectedIds.has(listing.id);
                  return (
                    <div key={listing.id}>
                      <div style={{
                        padding: '14px 16px',
                        background: isSelected ? '#f0fdf4' : 'transparent',
                      }}>
                        <InlineStack align="space-between" blockAlign="center" wrap={false}>
                          {/* Left: checkbox + image + info */}
                          <InlineStack gap="400" blockAlign="start" wrap={false}>
                            <input
                              type="checkbox"
                              checked={isSelected}
                              onChange={() => toggleSelect(listing.id)}
                              style={{ width: '16px', height: '16px', cursor: 'pointer', marginTop: '4px' }}
                            />
                            <Thumbnail source={imgUrl || ImageIcon} alt={listing.title} size="medium" />
                            <BlockStack gap="100">
                              <Text as="span" variant="bodyMd" fontWeight="semibold">{listing.title}</Text>
                              <InlineStack gap="200" wrap>
                                <Badge tone={statusTone(listing.status)}>{listing.status}</Badge>
                                <Text as="span" variant="bodySm" tone="subdued">{getCategoryLabel(listing.category)}</Text>
                              </InlineStack>
                              {/* Stock info */}
                              {(() => {
                                const totalStock = listing.variants?.reduce((s, v) => s + v.inventory_quantity, 0) || 0;
                                const allocationPct = listing.marketplace_stock_percent || 100;
                                const marketplaceStock = Math.floor((totalStock * allocationPct) / 100);
                                const price = listing.variants?.[0]?.wholesale_price;
                                return (
                                  <InlineStack gap="300" wrap>
                                    <span style={{
                                      padding: '2px 8px', borderRadius: '6px', fontSize: '11px', fontWeight: 600,
                                      background: '#f1f5f9', color: '#475569',
                                    }}>
                                      Total Stock: {totalStock}
                                    </span>
                                    <span style={{
                                      padding: '2px 8px', borderRadius: '6px', fontSize: '11px', fontWeight: 600,
                                      background: marketplaceStock > 0 ? '#dcfce7' : '#fee2e2',
                                      color: marketplaceStock > 0 ? '#166534' : '#991b1b',
                                    }}>
                                      Marketplace: {marketplaceStock} ({allocationPct}%)
                                    </span>
                                    {price != null && (
                                      <span style={{ fontSize: '12px', color: '#64748b', fontWeight: 500 }}>
                                        ${price.toFixed(2)} wholesale
                                      </span>
                                    )}
                                    <span style={{ fontSize: '11px', color: '#94a3b8' }}>
                                      {listing.processing_days}d processing
                                    </span>
                                  </InlineStack>
                                );
                              })()}
                            </BlockStack>
                          </InlineStack>

                          {/* Right: actions */}
                          <BlockStack gap="200" align="end">
                            <InlineStack gap="200" wrap={false}>
                              <Button size="slim" onClick={() => navigate(`/supplier/listings/${listing.id}`)}>Edit</Button>
                              {listing.status === 'draft' && (
                                <Button size="slim" variant="primary" onClick={() => handleStatusChange(listing.id, 'active')}>Publish</Button>
                              )}
                              {listing.status === 'active' && (
                                <Button size="slim" onClick={() => handleStatusChange(listing.id, 'paused')}>Pause</Button>
                              )}
                              {listing.status === 'paused' && (
                                <Button size="slim" onClick={() => handleStatusChange(listing.id, 'active')}>Resume</Button>
                              )}
                            </InlineStack>
                            <button
                              onClick={() => setConfirmDelete(listing.id)}
                              style={{
                                padding: '4px 14px', fontSize: '13px', fontWeight: 600,
                                background: '#fee2e2', color: '#dc2626', border: '1px solid #fca5a5',
                                borderRadius: '8px', cursor: 'pointer', transition: 'all 0.15s',
                              }}
                              onMouseOver={(e) => { e.currentTarget.style.background = '#dc2626'; e.currentTarget.style.color = '#fff'; }}
                              onMouseOut={(e) => { e.currentTarget.style.background = '#fee2e2'; e.currentTarget.style.color = '#dc2626'; }}
                            >
                              Delete
                            </button>
                          </BlockStack>
                        </InlineStack>
                      </div>
                      {i < filtered.length - 1 && <Divider />}
                    </div>
                  );
                })}
              </BlockStack>
            </Card>
          ) : (
            <Card>
              <EmptyState heading="No listings yet" image="">
                <p>Add products from your Shopify store to list them for resellers.</p>
              </EmptyState>
            </Card>
          )}
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
        message="Are you sure you want to delete this listing? This cannot be undone."
        onConfirm={() => { if (confirmDelete) { handleDelete(confirmDelete); setConfirmDelete(null); } }}
        onCancel={() => setConfirmDelete(null)}
      />

      <ConfirmDialog
        open={confirmBulkDelete}
        title="Delete Selected"
        message={`Delete ${selectedIds.size} listing(s)? This cannot be undone.`}
        onConfirm={() => { handleBulkAction('delete'); setConfirmBulkDelete(false); }}
        onCancel={() => setConfirmBulkDelete(false)}
      />
    </Page>
  );
}
