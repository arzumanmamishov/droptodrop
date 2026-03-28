import { useState, useCallback } from 'react';
import {
  Page, Layout, Card, BlockStack, Text, Spinner,
  Banner, InlineStack, Modal, FormLayout, TextField, Select,
  EmptyState, ProgressBar,
} from '@shopify/polaris';
import { useApi } from '../hooks/useApi';
import { api } from '../utils/api';

interface Review {
  id: string; rating: number; title: string; comment: string;
  created_at: string; reseller_domain: string;
}
interface ReviewSummary {
  average_rating: number; total_reviews: number;
  five_star: number; four_star: number; three_star: number; two_star: number; one_star: number;
}
interface Props { role: string; shopId: string; }

export default function Reviews({ role, shopId }: Props) {
  const isSupplier = role === 'supplier';
  const endpoint = isSupplier ? `/reviews/${shopId}` : null;
  const { data, loading, refetch } = useApi<{ reviews: Review[]; summary: ReviewSummary }>(endpoint || '/reviews/none');

  const [writeModal, setWriteModal] = useState(false);
  const [supplierID, setSupplierID] = useState('');
  const [rating, setRating] = useState('5');
  const [title, setTitle] = useState('');
  const [comment, setComment] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState(false);

  const handleSubmit = useCallback(async () => {
    setSubmitting(true);
    try {
      await api.post('/reviews', { supplier_shop_id: supplierID, rating: parseInt(rating), title, comment });
      setSuccess(true);
      setWriteModal(false);
      setTitle(''); setComment('');
      refetch();
    } catch { /* */ }
    finally { setSubmitting(false); }
  }, [supplierID, rating, title, comment, refetch]);

  if (loading) return <Page title="Reviews"><div style={{ display: 'flex', justifyContent: 'center', padding: '3rem' }}><Spinner size="large" /></div></Page>;

  const reviews = data?.reviews || [];
  const summary = data?.summary;
  const stars = (n: number) => '★'.repeat(n) + '☆'.repeat(5 - n);

  return (
    <Page title="Reviews" primaryAction={!isSupplier ? { content: 'Write Review', onAction: () => setWriteModal(true) } : undefined}>
      <Layout>
        {success && <Layout.Section><Banner tone="success" onDismiss={() => setSuccess(false)}>Review submitted!</Banner></Layout.Section>}

        {summary && summary.total_reviews > 0 && (
          <Layout.Section>
            <Card>
              <InlineStack gap="800" blockAlign="center">
                <BlockStack gap="100" align="center">
                  <Text as="p" variant="heading2xl">{summary.average_rating.toFixed(1)}</Text>
                  <Text as="p" variant="bodyMd" tone="success">{stars(Math.round(summary.average_rating))}</Text>
                  <Text as="p" variant="bodySm" tone="subdued">{summary.total_reviews} reviews</Text>
                </BlockStack>
                <BlockStack gap="100" align="start">
                  {[['5', summary.five_star], ['4', summary.four_star], ['3', summary.three_star], ['2', summary.two_star], ['1', summary.one_star]].map(([label, count]) => (
                    <InlineStack key={label as string} gap="200" blockAlign="center">
                      <Text as="span" variant="bodySm">{label}★</Text>
                      <div style={{ width: '120px' }}><ProgressBar progress={summary.total_reviews > 0 ? ((count as number) / summary.total_reviews) * 100 : 0} size="small" /></div>
                      <Text as="span" variant="bodySm" tone="subdued">{count as number}</Text>
                    </InlineStack>
                  ))}
                </BlockStack>
              </InlineStack>
            </Card>
          </Layout.Section>
        )}

        <Layout.Section>
          {reviews.length > 0 ? (
            <BlockStack gap="300">
              {reviews.map((r) => (
                <Card key={r.id}>
                  <BlockStack gap="200">
                    <InlineStack align="space-between" blockAlign="center">
                      <InlineStack gap="200" blockAlign="center">
                        <Text as="span" variant="bodyMd" tone="success">{stars(r.rating)}</Text>
                        {r.title && <Text as="span" variant="headingSm">{r.title}</Text>}
                      </InlineStack>
                      <Text as="span" variant="bodySm" tone="subdued">{new Date(r.created_at).toLocaleDateString()}</Text>
                    </InlineStack>
                    {r.comment && <Text as="p" variant="bodyMd">{r.comment}</Text>}
                    <Text as="p" variant="bodySm" tone="subdued">by {r.reseller_domain || 'Anonymous'}</Text>
                  </BlockStack>
                </Card>
              ))}
            </BlockStack>
          ) : (
            <Card><EmptyState heading={isSupplier ? "No reviews yet" : "No reviews to show"} image=""><p>{isSupplier ? 'Reviews from resellers will appear here.' : 'Write a review for a supplier after completing an order.'}</p></EmptyState></Card>
          )}
        </Layout.Section>
      </Layout>

      {writeModal && (
        <Modal open onClose={() => setWriteModal(false)} title="Write a Review"
          primaryAction={{ content: 'Submit', onAction: handleSubmit, loading: submitting, disabled: !supplierID }}
          secondaryActions={[{ content: 'Cancel', onAction: () => setWriteModal(false) }]}>
          <Modal.Section>
            <FormLayout>
              <TextField label="Supplier Shop ID" value={supplierID} onChange={setSupplierID} autoComplete="off" helpText="Paste supplier ID from the supplier profile page" />
              <Select label="Rating" options={[{label:'5 - Excellent',value:'5'},{label:'4 - Good',value:'4'},{label:'3 - Average',value:'3'},{label:'2 - Poor',value:'2'},{label:'1 - Terrible',value:'1'}]} value={rating} onChange={setRating} />
              <TextField label="Title (optional)" value={title} onChange={setTitle} autoComplete="off" />
              <TextField label="Comment" value={comment} onChange={setComment} multiline={3} autoComplete="off" />
            </FormLayout>
          </Modal.Section>
        </Modal>
      )}
    </Page>
  );
}
