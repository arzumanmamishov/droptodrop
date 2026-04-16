import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Spinner,
  EmptyState, Divider,
  Modal, TextField, FormLayout,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';
import ConfirmDialog from '../components/ConfirmDialog';

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

const statusConfig: Record<string, { color: string; bg: string; label: string; icon: string }> = {
  requested:      { color: '#92400e', bg: '#fef3c7', label: 'Awaiting Label', icon: '📋' },
  label_uploaded: { color: '#1e40af', bg: '#dbeafe', label: 'Label Ready', icon: '🏷️' },
  shipped_back:   { color: '#6d28d9', bg: '#ede9fe', label: 'Shipped Back', icon: '📦' },
  received:       { color: '#166534', bg: '#dcfce7', label: 'Received', icon: '✅' },
  refunded:       { color: '#166534', bg: '#dcfce7', label: 'Refunded', icon: '💰' },
  rejected:       { color: '#991b1b', bg: '#fee2e2', label: 'Rejected', icon: '❌' },
};

const STEPS = ['requested', 'label_uploaded', 'shipped_back', 'received', 'refunded'];

export default function Returns({ role }: Props) {
  const toast = useToast();
  const { data, loading, refetch } = useApi<{ returns: ReturnRequest[] }>('/returns');
  const isSupplier = role === 'supplier';

  const [labelModal, setLabelModal] = useState<string | null>(null);
  const [labelUrl, setLabelUrl] = useState('');
  const [labelNotes, setLabelNotes] = useState('');
  const [uploading, setUploading] = useState(false);
  const [confirmReject, setConfirmReject] = useState<string | null>(null);

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
  const activeCount = returns.filter(r => !['refunded', 'rejected'].includes(r.status)).length;

  return (
    <Page title="Returns" subtitle={isSupplier ? `${activeCount} active return${activeCount !== 1 ? 's' : ''}` : 'Your return requests'}>
      <Layout>
        <Layout.Section>
          {returns.length > 0 ? (
            <BlockStack gap="400">
              {returns.map((r) => {
                const cfg = statusConfig[r.status] || statusConfig['requested'];
                const stepIndex = STEPS.indexOf(r.status);
                const isRejected = r.status === 'rejected';
                const isDone = r.status === 'refunded' || isRejected;

                return (
                  <Card key={r.id}>
                    {/* Header */}
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '12px' }}>
                      <div>
                        <div style={{ fontSize: '16px', fontWeight: 700, color: '#1e293b' }}>
                          Order #{r.order_number || r.order_id.slice(0, 8)}
                        </div>
                        <div style={{ fontSize: '12px', color: '#94a3b8', marginTop: '2px' }}>
                          {new Date(r.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })} {new Date(r.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                          {' · '}{isSupplier ? `from ${r.reseller}` : `via ${r.supplier}`}
                        </div>
                      </div>
                      <span style={{
                        padding: '4px 14px', borderRadius: '20px', fontSize: '12px', fontWeight: 700,
                        color: cfg.color, background: cfg.bg,
                      }}>
                        {cfg.icon} {cfg.label}
                      </span>
                    </div>

                    {/* Progress bar */}
                    {!isRejected && (
                      <div style={{ marginBottom: '14px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '2px' }}>
                          {STEPS.map((step, i) => (
                            <div key={step} style={{ display: 'flex', alignItems: 'center', flex: 1 }}>
                              <div style={{
                                width: '24px', height: '24px', borderRadius: '50%', flexShrink: 0,
                                background: i <= stepIndex ? '#1e40af' : '#e2e8f0',
                                display: 'flex', alignItems: 'center', justifyContent: 'center',
                                fontSize: '11px', fontWeight: 700, color: i <= stepIndex ? '#fff' : '#94a3b8',
                              }}>
                                {i <= stepIndex ? '✓' : i + 1}
                              </div>
                              {i < STEPS.length - 1 && (
                                <div style={{
                                  flex: 1, height: '3px', margin: '0 2px',
                                  background: i < stepIndex ? '#1e40af' : '#e2e8f0', borderRadius: '2px',
                                }} />
                              )}
                            </div>
                          ))}
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '4px' }}>
                          {STEPS.map((step, i) => (
                            <span key={step} style={{
                              fontSize: '9px', textAlign: 'center', flex: 1,
                              color: i <= stepIndex ? '#1e293b' : '#cbd5e1',
                              fontWeight: i === stepIndex ? 700 : 400,
                            }}>
                              {statusConfig[step]?.label || step}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}

                    <Divider />

                    {/* Details */}
                    <div style={{ display: 'flex', gap: '24px', marginTop: '12px', flexWrap: 'wrap' }}>
                      <div>
                        <div style={{ fontSize: '11px', color: '#94a3b8', marginBottom: '2px' }}>Customer</div>
                        <div style={{ fontSize: '13px', fontWeight: 600 }}>{r.customer_name || 'N/A'}</div>
                      </div>
                      <div>
                        <div style={{ fontSize: '11px', color: '#94a3b8', marginBottom: '2px' }}>Reason</div>
                        <div style={{ fontSize: '13px' }}>{r.reason}</div>
                      </div>
                      {r.supplier_notes && (
                        <div>
                          <div style={{ fontSize: '11px', color: '#94a3b8', marginBottom: '2px' }}>Supplier Notes</div>
                          <div style={{ fontSize: '13px' }}>{r.supplier_notes}</div>
                        </div>
                      )}
                    </div>

                    {/* Return label */}
                    {r.return_label_url && (
                      <div style={{ marginTop: '10px' }}>
                        <a href={r.return_label_url} target="_blank" rel="noopener noreferrer"
                          style={{
                            display: 'inline-flex', alignItems: 'center', gap: '6px',
                            padding: '6px 14px', borderRadius: '8px', fontSize: '13px', fontWeight: 600,
                            background: '#dbeafe', color: '#1e40af', textDecoration: 'none',
                          }}>
                          🏷️ Download Return Label
                        </a>
                      </div>
                    )}

                    {/* Actions */}
                    {!isDone && (
                      <div style={{ marginTop: '12px', display: 'flex', gap: '8px' }}>
                        {isSupplier && r.status === 'requested' && (
                          <>
                            <button onClick={() => setLabelModal(r.id)} style={{
                              padding: '8px 20px', fontSize: '13px', fontWeight: 600,
                              background: '#1e40af', color: '#fff', border: 'none', borderRadius: '8px', cursor: 'pointer',
                            }}>Upload Return Label</button>
                            <button onClick={() => setConfirmReject(r.id)} style={{
                              padding: '8px 20px', fontSize: '13px', fontWeight: 600,
                              background: '#fff', color: '#dc2626', border: '1px solid #fca5a5', borderRadius: '8px', cursor: 'pointer',
                            }}>Reject</button>
                          </>
                        )}
                        {isSupplier && r.status === 'shipped_back' && (
                          <button onClick={() => handleUpdateStatus(r.id, 'received')} style={{
                            padding: '8px 20px', fontSize: '13px', fontWeight: 600,
                            background: '#111', color: '#fff', border: 'none', borderRadius: '8px', cursor: 'pointer',
                          }}>Mark Received</button>
                        )}
                        {isSupplier && r.status === 'received' && (
                          <button onClick={() => handleUpdateStatus(r.id, 'refunded')} style={{
                            padding: '8px 20px', fontSize: '13px', fontWeight: 600,
                            background: '#166534', color: '#fff', border: 'none', borderRadius: '8px', cursor: 'pointer',
                          }}>Mark Refunded</button>
                        )}
                        {!isSupplier && r.status === 'label_uploaded' && (
                          <button onClick={() => handleUpdateStatus(r.id, 'shipped_back')} style={{
                            padding: '8px 20px', fontSize: '13px', fontWeight: 600,
                            background: '#111', color: '#fff', border: 'none', borderRadius: '8px', cursor: 'pointer',
                          }}>Customer Has Shipped</button>
                        )}
                      </div>
                    )}
                  </Card>
                );
              })}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading="No returns" image="">
                <p>{isSupplier ? 'Return requests from resellers will appear here.' : 'Request a return from a fulfilled order.'}</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>

      {labelModal && (
        <Modal open onClose={() => setLabelModal(null)} title="Upload Return Label"
          primaryAction={{ content: 'Upload Label', onAction: handleUploadLabel, loading: uploading, disabled: !labelUrl.trim() }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setLabelModal(null) }]}
        >
          <Modal.Section>
            <FormLayout>
              <TextField label="Return label URL" value={labelUrl} onChange={setLabelUrl} autoComplete="url"
                placeholder="https://..." helpText="Paste URL to the return shipping label (PDF or image)" />
              <TextField label="Notes for reseller (optional)" value={labelNotes} onChange={setLabelNotes}
                autoComplete="off" multiline={2} placeholder="Return instructions..." />
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}

      <ConfirmDialog
        open={confirmReject !== null}
        title="Reject Return"
        message="Are you sure you want to reject this return request? The reseller will be notified."
        confirmLabel="Yes, Reject"
        destructive
        onConfirm={() => { if (confirmReject) { handleUpdateStatus(confirmReject, 'rejected'); setConfirmReject(null); } }}
        onCancel={() => setConfirmReject(null)}
      />
    </Page>
  );
}
