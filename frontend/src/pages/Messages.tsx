import { useState, useCallback, useEffect, useRef } from 'react';
import { Page, Spinner, Text } from '@shopify/polaris';
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
  const [selectedConv, setSelectedConv] = useState<string | null>(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get('conv');
  });
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
    const params = new URLSearchParams(window.location.search);
    const toShopId = params.get('to');
    if (toShopId && !selectedConv) {
      api.post<{ id: string }>('/conversations', { other_shop_id: toShopId, subject: '' })
        .then((conv) => {
          setSelectedConv(conv.id);
          refetchConvs();
        })
        .catch(() => {});
    }
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

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  if (convsLoading) {
    return <Page title="Messages"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;
  }

  const conversations = convData?.conversations || [];
  const activeConv = conversations.find(c => c.id === selectedConv);

  return (
    <Page title="Messages">
      <div style={{
        display: 'flex', height: 'calc(100vh - 140px)', minHeight: '500px',
        background: '#fff', borderRadius: '12px', border: '1px solid #e2e8f0', overflow: 'hidden',
      }}>
        {/* Sidebar: conversation list */}
        <div style={{
          width: '300px', minWidth: '300px', borderRight: '1px solid #e2e8f0',
          display: 'flex', flexDirection: 'column', background: '#f8fafc',
        }}>
          <div style={{ padding: '16px 20px', borderBottom: '1px solid #e2e8f0' }}>
            <Text as="h2" variant="headingMd">Chats</Text>
          </div>
          <div style={{ flex: 1, overflowY: 'auto' }}>
            {conversations.length > 0 ? conversations.map((conv) => (
              <div
                key={conv.id}
                onClick={() => setSelectedConv(conv.id)}
                style={{
                  padding: '14px 20px', cursor: 'pointer',
                  background: selectedConv === conv.id ? '#dbeafe' : 'transparent',
                  borderLeft: selectedConv === conv.id ? '3px solid #1e40af' : '3px solid transparent',
                  borderBottom: '1px solid #f1f5f9',
                  transition: 'all 0.1s',
                }}
                onMouseOver={(e) => { if (selectedConv !== conv.id) e.currentTarget.style.background = '#f1f5f9'; }}
                onMouseOut={(e) => { if (selectedConv !== conv.id) e.currentTarget.style.background = 'transparent'; }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                  <span style={{ fontSize: '14px', fontWeight: 600, color: '#1e293b' }}>
                    {conv.other_shop_name || 'Shop'}
                  </span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    <span style={{ fontSize: '11px', color: '#94a3b8' }}>
                      {new Date(conv.last_message_at).toLocaleDateString([], { month: 'short', day: 'numeric' })}
                    </span>
                    {conv.unread_count > 0 && (
                      <span style={{
                        background: '#1e40af', color: '#fff', fontSize: '11px', fontWeight: 700,
                        borderRadius: '10px', padding: '1px 7px', minWidth: '18px', textAlign: 'center',
                      }}>
                        {conv.unread_count}
                      </span>
                    )}
                  </div>
                </div>
                <p style={{
                  fontSize: '13px', color: '#64748b', margin: 0,
                  overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                }}>
                  {conv.last_message || 'No messages yet'}
                </p>
              </div>
            )) : (
              <div style={{ padding: '40px 20px', textAlign: 'center' }}>
                <p style={{ color: '#94a3b8', fontSize: '14px' }}>No conversations yet</p>
              </div>
            )}
          </div>
        </div>

        {/* Main: chat area */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', background: '#fff' }}>
          {selectedConv ? (
            <>
              {/* Chat header */}
              <div style={{
                padding: '14px 24px', borderBottom: '1px solid #e2e8f0',
                display: 'flex', alignItems: 'center', gap: '12px',
              }}>
                <div style={{
                  width: '38px', height: '38px', borderRadius: '50%',
                  background: 'linear-gradient(135deg, #1e40af, #3b82f6)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  color: '#fff', fontSize: '16px', fontWeight: 700, flexShrink: 0,
                }}>
                  {(activeConv?.other_shop_name || 'S').charAt(0).toUpperCase()}
                </div>
                <div>
                  <div style={{ fontSize: '15px', fontWeight: 600, color: '#1e293b' }}>
                    {activeConv?.other_shop_name || 'Shop'}
                  </div>
                  {activeConv?.subject && (
                    <div style={{ fontSize: '12px', color: '#94a3b8' }}>{activeConv.subject}</div>
                  )}
                </div>
              </div>

              {/* Messages */}
              <div style={{
                flex: 1, overflowY: 'auto', padding: '20px 24px',
                background: '#f8fafc',
              }}>
                {loadingMsgs ? (
                  <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner /></div>
                ) : messages.length > 0 ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                    {messages.map((msg, i) => {
                      const showDate = i === 0 || new Date(msg.created_at).toDateString() !== new Date(messages[i - 1].created_at).toDateString();
                      return (
                        <div key={msg.id}>
                          {showDate && (
                            <div style={{ textAlign: 'center', margin: '16px 0 8px' }}>
                              <span style={{
                                fontSize: '11px', color: '#94a3b8', background: '#e2e8f0',
                                padding: '3px 12px', borderRadius: '10px',
                              }}>
                                {new Date(msg.created_at).toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })}
                              </span>
                            </div>
                          )}
                          <div style={{
                            display: 'flex',
                            justifyContent: msg.is_mine ? 'flex-end' : 'flex-start',
                          }}>
                            <div style={{
                              maxWidth: '70%',
                              padding: '10px 16px',
                              borderRadius: msg.is_mine ? '18px 18px 4px 18px' : '18px 18px 18px 4px',
                              background: msg.is_mine ? '#1e40af' : '#fff',
                              color: msg.is_mine ? '#fff' : '#1e293b',
                              boxShadow: msg.is_mine ? 'none' : '0 1px 2px rgba(0,0,0,0.06)',
                              border: msg.is_mine ? 'none' : '1px solid #e2e8f0',
                            }}>
                              <div style={{ fontSize: '14px', lineHeight: '1.5', wordBreak: 'break-word' }}>
                                {msg.content}
                              </div>
                              <div style={{
                                fontSize: '11px', marginTop: '4px', textAlign: 'right',
                                opacity: msg.is_mine ? 0.7 : 0.5,
                              }}>
                                {new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                              </div>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                    <div ref={messagesEndRef} />
                  </div>
                ) : (
                  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
                    <div style={{ textAlign: 'center' }}>
                      <div style={{ fontSize: '40px', marginBottom: '8px' }}>💬</div>
                      <p style={{ color: '#94a3b8', fontSize: '14px' }}>No messages yet. Say hello!</p>
                    </div>
                  </div>
                )}
              </div>

              {/* Input area */}
              <div style={{
                padding: '12px 20px', borderTop: '1px solid #e2e8f0',
                background: '#fff',
              }}>
                <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-end' }}>
                  <textarea
                    value={newMessage}
                    onChange={(e) => setNewMessage(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder="Type a message..."
                    rows={1}
                    style={{
                      flex: 1, padding: '10px 16px', fontSize: '14px',
                      border: '1px solid #e2e8f0', borderRadius: '24px',
                      outline: 'none', resize: 'none', fontFamily: 'inherit',
                      lineHeight: '1.4', maxHeight: '120px',
                      transition: 'border-color 0.15s',
                    }}
                    onFocus={(e) => (e.target.style.borderColor = '#1e40af')}
                    onBlur={(e) => (e.target.style.borderColor = '#e2e8f0')}
                  />
                  <button
                    onClick={handleSend}
                    disabled={!newMessage.trim() || sending}
                    style={{
                      width: '42px', height: '42px', borderRadius: '50%',
                      background: newMessage.trim() ? '#1e40af' : '#e2e8f0',
                      border: 'none', cursor: newMessage.trim() ? 'pointer' : 'default',
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                      transition: 'background 0.15s', flexShrink: 0,
                    }}
                  >
                    <svg width="20" height="20" viewBox="0 0 24 24" fill={newMessage.trim() ? '#fff' : '#94a3b8'}>
                      <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
                    </svg>
                  </button>
                </div>
              </div>
            </>
          ) : (
            <div style={{
              flex: 1, display: 'flex', justifyContent: 'center', alignItems: 'center',
              background: '#f8fafc',
            }}>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: '56px', marginBottom: '12px' }}>💬</div>
                <h3 style={{ fontSize: '18px', fontWeight: 600, color: '#1e293b', margin: '0 0 4px' }}>
                  Select a conversation
                </h3>
                <p style={{ color: '#94a3b8', fontSize: '14px', margin: 0 }}>
                  Choose a chat from the list or message a supplier from Imports
                </p>
              </div>
            </div>
          )}
        </div>
      </div>
    </Page>
  );
}
