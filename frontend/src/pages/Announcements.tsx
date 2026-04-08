import { useState, useCallback } from 'react';
import ConfirmDialog from '../components/ConfirmDialog';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Button, Spinner,
  Banner, InlineStack, Divider, Modal, FormLayout, TextField, Checkbox,
  EmptyState, Icon,
} from '@shopify/polaris';
import { MegaphoneIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { useToast } from '../hooks/useToast';
import { api } from '../utils/api';

interface Announcement {
  id: string;
  supplier_shop_id: string;
  title: string;
  content: string;
  is_pinned: boolean;
  created_at: string;
  is_read: boolean;
  supplier_name: string;
}

interface Props {
  role: string;
}

export default function Announcements({ role }: Props) {
  const toast = useToast();
  const { data, loading, refetch } = useApi<{ announcements: Announcement[] }>('/announcements');
  const [createModal, setCreateModal] = useState(false);
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [isPinned, setIsPinned] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const isSupplier = role === 'supplier';

  const handleCreate = useCallback(async () => {
    setCreating(true);
    setError(null);
    try {
      await api.post('/announcements', { title, content, is_pinned: isPinned });
      setSuccess(true);
      setCreateModal(false);
      setTitle('');
      setContent('');
      setIsPinned(false);
      refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed');
    } finally {
      setCreating(false);
    }
  }, [title, content, isPinned, refetch]);

  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const handleDelete = useCallback(async (id: string) => {
    try {
      await api.delete(`/announcements/${id}`);
      refetch();
    } catch { toast.error('Failed to delete announcement'); }
  }, [refetch, toast]);

  const handleMarkRead = useCallback(async (id: string) => {
    try {
      await api.post(`/announcements/${id}/read`);
      refetch();
    } catch { toast.error('Failed to mark announcement as read'); }
  }, [refetch, toast]);

  if (loading) {
    return <Page title="Announcements"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const announcements = data?.announcements || [];

  return (
    <Page
      title="Announcements"
      subtitle={isSupplier ? 'Broadcast to your resellers' : 'Updates from your suppliers'}
      primaryAction={isSupplier ? { content: 'New Announcement', onAction: () => setCreateModal(true) } : undefined}
    >
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(false)}>Announcement published!</Banner></Layout.Section>}

        <Layout.Section>
          {announcements.length > 0 ? (
            <BlockStack gap="400">
              {announcements.map((ann) => (
                <Card key={ann.id}>
                  <BlockStack gap="300">
                    <InlineStack align="space-between" blockAlign="center">
                      <InlineStack gap="300" blockAlign="center">
                        <div style={{ background: ann.is_pinned ? '#fef3cd' : '#e0f0ff', borderRadius: '8px', padding: '8px', display: 'flex' }}>
                          <Icon source={MegaphoneIcon} />
                        </div>
                        <BlockStack gap="050">
                          <InlineStack gap="200" blockAlign="center">
                            <Text as="h3" variant="headingMd">{ann.title}</Text>
                            {ann.is_pinned && <Badge tone="attention">Pinned</Badge>}
                            {!ann.is_read && !isSupplier && <Badge tone="info">New</Badge>}
                          </InlineStack>
                          {ann.supplier_name && <Text as="p" variant="bodySm" tone="subdued">From: {ann.supplier_name}</Text>}
                        </BlockStack>
                      </InlineStack>
                      <Text as="span" variant="bodySm" tone="subdued">{new Date(ann.created_at).toLocaleDateString()}</Text>
                    </InlineStack>
                    <Divider />
                    <Text as="p" variant="bodyMd">{ann.content}</Text>
                    <InlineStack gap="200">
                      {!isSupplier && !ann.is_read && (
                        <Button size="slim" onClick={() => handleMarkRead(ann.id)}>Mark as Read</Button>
                      )}
                      {isSupplier && (
                        <Button size="slim" tone="critical" onClick={() => setConfirmDelete(ann.id)}>Delete</Button>
                      )}
                    </InlineStack>
                  </BlockStack>
                </Card>
              ))}
            </BlockStack>
          ) : (
            <Card>
              <EmptyState heading="No announcements" image="">
                <p>{isSupplier ? 'Create an announcement to notify all resellers who sell your products.' : 'No announcements from your suppliers yet.'}</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>

      {createModal && (
        <Modal open onClose={() => setCreateModal(false)} title="New Announcement"
          primaryAction={{ content: 'Publish', onAction: handleCreate, loading: creating, disabled: !title || !content }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setCreateModal(false) }]}>
          <Modal.Section>
            <BlockStack gap="400">
              {error && <Banner tone="critical">{error}</Banner>}
              <FormLayout>
                <TextField label="Title" value={title} onChange={setTitle} autoComplete="off" />
                <TextField label="Content" value={content} onChange={setContent} multiline={4} autoComplete="off" />
                <Checkbox label="Pin this announcement (appears at top)" checked={isPinned} onChange={setIsPinned} />
              </FormLayout>
            </BlockStack>
          </Modal.Section>
        </Modal>
      )}

      <ConfirmDialog
        open={confirmDelete !== null}
        title="Delete Announcement"
        message="Are you sure you want to delete this announcement? All resellers will no longer see it."
        onConfirm={() => { if (confirmDelete) { handleDelete(confirmDelete); setConfirmDelete(null); } }}
        onCancel={() => setConfirmDelete(null)}
      />
    </Page>
  );
}
