import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Spinner,
  Banner, InlineStack, Divider, EmptyState, Button,
  Modal, TextField,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';

interface Reseller {
  reseller_shop_id: string;
  domain: string;
  status: string;
  reason: string;
  import_count: number;
  order_count: number;
}

export default function Resellers() {
  const { data, loading, error, refetch } = useApi<{ resellers: Reseller[] }>('/supplier/resellers');
  const [actionModal, setActionModal] = useState<Reseller | null>(null);
  const [newStatus, setNewStatus] = useState('paused');
  const [reason, setReason] = useState('');
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);

  const handleUpdate = useCallback(async () => {
    if (!actionModal) return;
    setSaving(true);
    try {
      await api.put(`/supplier/resellers/${actionModal.reseller_shop_id}/status`, {
        status: newStatus, reason,
      });
      setSuccess(`Reseller ${newStatus === 'blocked' ? 'blocked' : newStatus === 'paused' ? 'paused' : 're-activated'} successfully.`);
      setActionModal(null);
      setReason('');
      refetch();
    } catch { /* */ }
    finally { setSaving(false); }
  }, [actionModal, newStatus, reason, refetch]);

  if (loading) {
    return <Page title="My Resellers"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const resellers = data?.resellers || [];
  const activeCount = resellers.filter(r => r.status === 'active').length;
  const blockedCount = resellers.filter(r => r.status === 'blocked' || r.status === 'paused').length;

  const statusBadge = (status: string) => {
    const map: Record<string, 'success' | 'attention' | 'critical'> = {
      active: 'success', paused: 'attention', blocked: 'critical',
    };
    return <Badge tone={map[status] || 'info'}>{status}</Badge>;
  };

  return (
    <Page title="My Resellers" subtitle={`${resellers.length} resellers selling your products`}>
      <Layout>
        {error && <Layout.Section><Banner tone="critical">{error}</Banner></Layout.Section>}
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(null)}>{success}</Banner></Layout.Section>}

        <Layout.Section>
          <InlineStack gap="300">
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">{activeCount}</div>
              <div className="stat-card-label">Active Resellers</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">{blockedCount}</div>
              <div className="stat-card-label">Paused / Blocked</div>
            </div>
            <div className="stat-card" style={{ flex: 1 }}>
              <div className="stat-card-value">{resellers.reduce((sum, r) => sum + r.order_count, 0)}</div>
              <div className="stat-card-label">Total Orders</div>
            </div>
          </InlineStack>
        </Layout.Section>

        <Layout.Section>
          {resellers.length > 0 ? (
            <Card>
              <BlockStack gap="0">
                {resellers.map((reseller, i) => (
                  <div key={reseller.reseller_shop_id}>
                    <div style={{ padding: '16px' }}>
                      <InlineStack align="space-between" blockAlign="center" wrap={false}>
                        <BlockStack gap="100">
                          <Text as="span" variant="bodyMd" fontWeight="semibold">{reseller.domain}</Text>
                          <InlineStack gap="200">
                            {statusBadge(reseller.status)}
                            <Text as="span" variant="bodySm" tone="subdued">{reseller.import_count} products imported</Text>
                            <Text as="span" variant="bodySm" tone="subdued">{reseller.order_count} orders</Text>
                          </InlineStack>
                          {reseller.reason && (
                            <Text as="span" variant="bodySm" tone="critical">Reason: {reseller.reason}</Text>
                          )}
                        </BlockStack>
                        <InlineStack gap="200" wrap={false}>
                          {reseller.status === 'active' ? (
                            <>
                              <Button size="slim" onClick={() => { setActionModal(reseller); setNewStatus('paused'); }}>Pause</Button>
                              <Button size="slim" tone="critical" onClick={() => { setActionModal(reseller); setNewStatus('blocked'); }}>Block</Button>
                            </>
                          ) : (
                            <Button size="slim" variant="primary" onClick={() => { setActionModal(reseller); setNewStatus('active'); }}>Reactivate</Button>
                          )}
                        </InlineStack>
                      </InlineStack>
                    </div>
                    {i < resellers.length - 1 && <Divider />}
                  </div>
                ))}
              </BlockStack>
            </Card>
          ) : (
            <Card>
              <EmptyState heading="No resellers yet" image="">
                <p>When resellers import your products, they'll appear here. You can manage their access.</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>

      {actionModal && (
        <Modal open onClose={() => setActionModal(null)}
          title={newStatus === 'active' ? 'Reactivate Reseller' : newStatus === 'blocked' ? 'Block Reseller' : 'Pause Reseller'}
          primaryAction={{
            content: newStatus === 'active' ? 'Reactivate' : newStatus === 'blocked' ? 'Block' : 'Pause',
            onAction: handleUpdate,
            loading: saving,
            destructive: newStatus !== 'active',
          }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setActionModal(null) }]}
        >
          <Modal.Section>
            <BlockStack gap="300">
              <Text as="p" variant="bodyMd">
                {newStatus === 'active'
                  ? `Reactivate ${actionModal.domain}? Their imports will be restored.`
                  : newStatus === 'blocked'
                  ? `Block ${actionModal.domain}? All their imports of your products will be paused and they cannot import new ones.`
                  : `Pause ${actionModal.domain}? Their imports will be temporarily paused.`}
              </Text>
              <TextField label="Reason (optional)" value={reason} onChange={setReason} autoComplete="off" multiline={2}
                placeholder={newStatus === 'blocked' ? 'e.g., Suspicious activity, policy violation...' : 'e.g., Temporary pause for review...'} />
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
