import { Modal, BlockStack, Text } from '@shopify/polaris';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  destructive?: boolean;
  confirmLabel?: string;
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export default function ConfirmDialog({
  open, title, message, destructive = true,
  confirmLabel = 'Delete', loading = false, onConfirm, onCancel,
}: ConfirmDialogProps) {
  return (
    <Modal
      open={open}
      onClose={onCancel}
      title={title}
      primaryAction={{
        content: confirmLabel,
        onAction: onConfirm,
        destructive,
        loading,
      }}
      secondaryActions={[{ content: 'Cancel', onAction: onCancel }]}
    >
      <Modal.Section>
        <BlockStack gap="200">
          <Text as="p" variant="bodyMd">{message}</Text>
        </BlockStack>
      </Modal.Section>
    </Modal>
  );
}
