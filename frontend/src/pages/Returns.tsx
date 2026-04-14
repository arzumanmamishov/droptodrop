import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Spinner,
  InlineStack, EmptyState, Button,
  Modal, TextField, FormLayout,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';

interface ReturnRequest {
  id: string;
  order_id: string;
  order_number: string;
  status: string;
  reason: string;
  customer_name: string;
  return_label_url: string;
  supplier_notes: string;
  reseller: string;
  supplier: string;
  created_at: string;
}

interface Props { role: string; }

const statusConfig: Record<string, { color: string; bg: string; label: string }> = {
  requested:      { color: '#92400e', bg: '#fef3c7', label: 'Awaiting Label' },
  label_uploaded: { color: '#1e40af', bg: '#dbeafe', label: 'Label Ready' },
  shipped_back:   { color: '#6d28d9', bg: '#ede9fe', label: 'Shipped Back' },
  received:       { color: '#166534', bg: '#dcfce7', label: 'Received' },
  refunded:       { color: '#166534', bg: '#dcfce7', label: 'Refunded' },
  rejected:       { color: '#991b1b', bg: '#fee2e2', label: 'Rejected' },
};

export default function Returns({ role }: Props) {
  const toast = useToast();
  const { data, loading, refetch } = useApi<{ returns: ReturnRequest[] }>('/returns');
  const isSupplier = role === 'supplier';

  // Label upload modal
  const [labelModal, setLabelModal] = useState<string | null>(null);
  const [labelUrl, setLabelUrl] = useState('');
  const [labelNotes, setLabelNotes] = useState('');
  const [uploading, setUploading] = useState(false);

  const handleUploadLabel = useCallback(async () => {
    if (!labelModal || !labelUrl.trim()) return;
    setUploading(true);
    try {
      await api.put(`/returns/${labelModal}/label`, { label_url: labelUrl, notes: labelNotes });
      toast.success('Return label uploaded');
      setLabelModal(null); setLabelUrl(''); setLabelNotes('');
      refetch();
    } catch { toast.error('Failed to upload label'); }
    finally { setUploading(false); }
  }, [labelModal, labelUrl, labelNotes, refetch, toast]);

  const handleUpdateStatus = useCallback(async (id: string, status: string) => {
    try {
      await api.put(`/returns/${id}/status`, { status });
      toast.success(`Return marked as ${status}`);
      refetch();
    } catch { toast.error('Failed to update return'); }
  }, [refetch, toast]);

  if (loading) return <Page title="Returns"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const returns = data?.returns || [];

  return (
    <Page title="Returns" subtitle={isSupplier ? 'Customer return requests' : 'Your return requests'}>
      <Layout>
        <Layout.Section>
          {returns.length > 0 ? (
            <BlockStack gap="300">
              {returns.map((r) => {
                const cfg = statusConfig[r.status] || statusConfig['requested'];
                return (
                  <Card key={r.id}>
                    <div style={{ padding: '4px 0' }}>
                      <InlineStack align="space-between" blockAlign="start" wrap={false}>
                        <BlockStack gap="100">
                          <InlineStack gap="200" blockAlign="center">
                            <Text as="span" variant="bodyMd" fontWeight="semibold">
                              Order #{r.order_number || r.order_id.slice(0, 8)}
                            </Text>
                            <span style={{
                              padding: '3px 12px', borderRadius: '20px', fontSize: '12px', fontWeight: 700,
                              color: cfg.color, background: cfg.bg,
                            }}>
                              {cfg.label}
                            </span>
                          </InlineStack>
                          <Text as="p" variant="bodySm" tone="subdued">
                            Customer: {r.customer_name || 'N/A'} · {isSupplier ? `from ${r.reseller}` : `via ${r.supplier}`}
                          </Text>
                          <Text as="p" variant="bodySm" tone="subdued">
                            Reason: {r.reason}
                          </Text>
                          {r.supplier_notes && (
                            <Text as="p" variant="bodySm">
                              Supplier notes: {r.supplier_notes}
                            </Text>
                          )}
                          {r.return_label_url && (
                            <a href={r.return_label_url} target="_blank" rel="noopener noreferrer"
                              style={{ fontSize: '13px', color: '#1e40af', fontWeight: 600 }}>
                              Download Return Label
                            </a>
                          )}
                          <Text as="span" variant="bodySm" tone="subdued">
                            {new Date(r.created_at).toLocaleDateString()} {new Date(r.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                          </Text>
                        </BlockStack>

                        <BlockStack gap="200" align="end">
                          {/* Supplier actions */}
                          {isSupplier && r.status === 'requested' && (
                            <>
                              <Button size="slim" variant="primary" onClick={() => setLabelModal(r.id)}>
                                Upload Label
                              </Button>
                              <Button size="slim" tone="critical" onClick={() => handleUpdateStatus(r.id, 'rejected')}>
                                Reject
                              </Button>
                            </>
                          )}
                          {isSupplier && r.status === 'shipped_back' && (
                            <Button size="slim" variant="primary" onClick={() => handleUpdateStatus(r.id, 'received')}>
                              Mark Received
                            </Button>
                          )}
                          {isSupplier && r.status === 'received' && (
                            <Button size="slim" variant="primary" onClick={() => handleUpdateStatus(r.id, 'refunded')}>
                              Mark Refunded
                            </Button>
                          )}

                          {/* Reseller actions */}
                          {!isSupplier && r.status === 'label_uploaded' && (
                            <Button size="slim" onClick={() => handleUpdateStatus(r.id, 'shipped_back')}>
                              Customer Shipped
                            </Button>
                          )}
                        </BlockStack>
                      </InlineStack>
                    </div>
                  </Card>
                );
              })}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading="No returns" image="">
                <p>{isSupplier ? 'Return requests from resellers will appear here.' : 'Your return requests will appear here.'}</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>

      {labelModal && (
        <Modal open onClose={() => setLabelModal(null)} title="Upload Return Label"
          primaryAction={{ content: 'Upload', onAction: handleUploadLabel, loading: uploading, disabled: !labelUrl.trim() }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setLabelModal(null) }]}
        >
          <Modal.Section>
            <FormLayout>
              <TextField label="Return label URL" value={labelUrl} onChange={setLabelUrl} autoComplete="url"
                placeholder="https://..." helpText="Paste the URL to the return shipping label (PDF or image)" />
              <TextField label="Notes for reseller (optional)" value={labelNotes} onChange={setLabelNotes}
                autoComplete="off" multiline={2} placeholder="Any instructions for the return..." />
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
