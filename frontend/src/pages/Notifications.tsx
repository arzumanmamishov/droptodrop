import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Button, Spinner,
  InlineStack, EmptyState, Divider, Box, Icon,
} from '@shopify/polaris';
import { NotificationIcon, CheckIcon } from '@shopify/polaris-icons';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';

interface Notification {
  id: string;
  title: string;
  message: string;
  type: 'info' | 'success' | 'warning' | 'error';
  is_read: boolean;
  link: string;
  created_at: string;
}

interface NotifResponse {
  notifications: Notification[];
  total: number;
}

export default function Notifications() {
  const [page, setPage] = useState(0);
  const limit = 30;
  const { data, loading, refetch } = useApi<NotifResponse>(
    `/notifications?limit=${limit}&offset=${page * limit}`,
  );
  const [markingAll, setMarkingAll] = useState(false);

  const handleMarkRead = useCallback(async (id: string) => {
    try {
      await api.post(`/notifications/read/${id}`);
      refetch();
    } catch { /* */ }
  }, [refetch]);

  const handleMarkAllRead = useCallback(async () => {
    setMarkingAll(true);
    try {
      await api.post('/notifications/read-all');
      refetch();
    } catch { /* */ }
    finally { setMarkingAll(false); }
  }, [refetch]);

  if (loading) {
    return <Page title="Notifications"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const notifications = data?.notifications || [];
  const unreadCount = notifications.filter(n => !n.is_read).length;

  const typeTone = (type: string): 'success' | 'attention' | 'critical' | 'info' => {
    const map: Record<string, 'success' | 'attention' | 'critical' | 'info'> = {
      success: 'success', warning: 'attention', error: 'critical', info: 'info',
    };
    return map[type] || 'info';
  };

  return (
    <Page
      title="Notifications"
      subtitle={unreadCount > 0 ? `${unreadCount} unread` : 'All caught up'}
      primaryAction={unreadCount > 0 ? { content: 'Mark All Read', onAction: handleMarkAllRead, loading: markingAll } : undefined}
    >
      <Layout>
        <Layout.Section>
          {notifications.length > 0 ? (
            <Card>
              <BlockStack gap="0">
                {notifications.map((n, i) => (
                  <div key={n.id}>
                    <Box padding="400" background={n.is_read ? undefined : 'bg-surface-secondary'}>
                      <InlineStack align="space-between" blockAlign="start">
                        <InlineStack gap="300" blockAlign="start">
                          <div style={{
                            background: n.is_read ? '#f6f6f7' : '#e0f0ff',
                            borderRadius: '8px', padding: '8px', display: 'flex',
                          }}>
                            <Icon source={n.is_read ? CheckIcon : NotificationIcon} />
                          </div>
                          <BlockStack gap="100">
                            <InlineStack gap="200" blockAlign="center">
                              <Text as="span" variant="headingSm" fontWeight={n.is_read ? 'regular' : 'semibold'}>
                                {n.title}
                              </Text>
                              <Badge tone={typeTone(n.type)}>{n.type}</Badge>
                            </InlineStack>
                            <Text as="p" variant="bodySm" tone="subdued">{n.message}</Text>
                            <Text as="p" variant="bodySm" tone="subdued">{new Date(n.created_at).toLocaleString()}</Text>
                          </BlockStack>
                        </InlineStack>
                        {!n.is_read && (
                          <Button size="slim" onClick={() => handleMarkRead(n.id)}>Mark Read</Button>
                        )}
                      </InlineStack>
                    </Box>
                    {i < notifications.length - 1 && <Divider />}
                  </div>
                ))}
              </BlockStack>
            </Card>
          ) : (
            <Card>
              <EmptyState heading="No notifications" image="">
                <p>You're all caught up. Notifications about orders, imports, and issues will appear here.</p>
              </EmptyState>
            </Card>
          )}
        </Layout.Section>
      </Layout>
    </Page>
  );
}
