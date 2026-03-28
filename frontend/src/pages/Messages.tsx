import { useState, useCallback, useEffect, useRef } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Badge, Button, Spinner,
  InlineStack, Divider, TextField, EmptyState,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';

interface Conversation {
  id: string;
  supplier_shop_id: string;
  reseller_shop_id: string;
  subject: string;
  last_message_at: string;
  other_shop_name: string;
  unread_count: number;
  last_message: string;
}

interface Message {
  id: string;
  conversation_id: string;
  sender_shop_id: string;
  content: string;
  is_read: boolean;
  created_at: string;
  is_mine: boolean;
}

export default function Messages() {
  const { data: convData, loading: convsLoading, refetch: refetchConvs } = useApi<{ conversations: Conversation[] }>('/conversations');
  const [selectedConv, setSelectedConv] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [loadingMsgs, setLoadingMsgs] = useState(false);
  const [newMessage, setNewMessage] = useState('');
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const loadMessages = useCallback(async (convId: string) => {
    setLoadingMsgs(true);
    try {
      const data = await api.get<{ messages: Message[] }>(`/conversations/${convId}/messages`);
      setMessages(data.messages || []);
    } catch { /* */ }
    finally { setLoadingMsgs(false); }
  }, []);

  useEffect(() => {
    if (selectedConv) loadMessages(selectedConv);
  }, [selectedConv, loadMessages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = useCallback(async () => {
    if (!selectedConv || !newMessage.trim()) return;
    setSending(true);
    try {
      await api.post(`/conversations/${selectedConv}/messages`, { content: newMessage });
      setNewMessage('');
      loadMessages(selectedConv);
      refetchConvs();
    } catch { /* */ }
    finally { setSending(false); }
  }, [selectedConv, newMessage, loadMessages, refetchConvs]);

  if (convsLoading) {
    return <Page title="Messages"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const conversations = convData?.conversations || [];

  return (
    <Page title="Messages">
      <Layout>
        <Layout.Section variant="oneThird">
          <Card>
            <BlockStack gap="0">
              <div style={{ padding: '12px 16px' }}>
                <Text as="h2" variant="headingMd">Conversations</Text>
              </div>
              <Divider />
              {conversations.length > 0 ? conversations.map((conv, i) => (
                <div key={conv.id}>
                  <div
                    style={{
                      padding: '12px 16px', cursor: 'pointer',
                      background: selectedConv === conv.id ? '#f0f7ff' : 'transparent',
                    }}
                    onClick={() => setSelectedConv(conv.id)}
                  >
                    <BlockStack gap="100">
                      <InlineStack align="space-between" blockAlign="center">
                        <Text as="span" variant="bodyMd" fontWeight="semibold">{conv.other_shop_name || 'Shop'}</Text>
                        {conv.unread_count > 0 && <Badge tone="attention">{String(conv.unread_count)}</Badge>}
                      </InlineStack>
                      <Text as="p" variant="bodySm" tone="subdued" truncate>{conv.last_message || conv.subject || 'No messages yet'}</Text>
                      <Text as="p" variant="bodySm" tone="subdued">{new Date(conv.last_message_at).toLocaleDateString()}</Text>
                    </BlockStack>
                  </div>
                  {i < conversations.length - 1 && <Divider />}
                </div>
              )) : (
                <div style={{ padding: '24px 16px', textAlign: 'center' }}>
                  <Text as="p" tone="subdued">No conversations yet</Text>
                </div>
              )}
            </BlockStack>
          </Card>
        </Layout.Section>

        <Layout.Section>
          <Card>
            {selectedConv ? (
              <BlockStack gap="0">
                <div style={{ minHeight: '400px', maxHeight: '400px', overflowY: 'auto', padding: '16px' }}>
                  {loadingMsgs ? (
                    <div style={{ display: 'flex', justifyContent: 'center', padding: '2rem' }}><Spinner /></div>
                  ) : messages.length > 0 ? (
                    <BlockStack gap="300">
                      {messages.map((msg) => (
                        <div key={msg.id} style={{ display: 'flex', justifyContent: msg.is_mine ? 'flex-end' : 'flex-start' }}>
                          <div style={{
                            background: msg.is_mine ? '#008060' : '#f6f6f7',
                            color: msg.is_mine ? 'white' : 'inherit',
                            borderRadius: '12px', padding: '8px 14px', maxWidth: '70%',
                          }}>
                            <Text as="p" variant="bodyMd">{msg.content}</Text>
                            <Text as="p" variant="bodySm" tone={msg.is_mine ? undefined : 'subdued'}>
                              {new Date(msg.created_at).toLocaleTimeString()}
                            </Text>
                          </div>
                        </div>
                      ))}
                      <div ref={messagesEndRef} />
                    </BlockStack>
                  ) : (
                    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
                      <Text as="p" tone="subdued">No messages yet. Start the conversation!</Text>
                    </div>
                  )}
                </div>
                <Divider />
                <div style={{ padding: '12px' }}>
                  <InlineStack gap="200" blockAlign="end">
                    <div onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); } }} style={{ flex: 1 }}>
                      <TextField label="" labelHidden value={newMessage} onChange={setNewMessage} placeholder="Type a message..." autoComplete="off" />
                    </div>
                    <Button variant="primary" onClick={handleSend} loading={sending} disabled={!newMessage.trim()}>Send</Button>
                  </InlineStack>
                </div>
              </BlockStack>
            ) : (
              <EmptyState heading="Select a conversation" image="">
                <p>Choose a conversation from the list or start a new one from a supplier's profile.</p>
              </EmptyState>
            )}
          </Card>
        </Layout.Section>
      </Layout>
    </Page>
  );
}
