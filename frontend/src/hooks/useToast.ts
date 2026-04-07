import { useCallback } from 'react';

// Uses Shopify App Bridge toast if available, falls back to a simple approach
export function useToast() {
  const show = useCallback((message: string, isError = false) => {
    // Try Shopify App Bridge toast
    if (window.shopify?.toast) {
      window.shopify.toast.show(message, { isError });
      return;
    }

    // Fallback: create a DOM toast
    const existing = document.getElementById('dt-toast');
    if (existing) existing.remove();

    const toast = document.createElement('div');
    toast.id = 'dt-toast';
    toast.textContent = message;
    Object.assign(toast.style, {
      position: 'fixed',
      bottom: '24px',
      left: '50%',
      transform: 'translateX(-50%)',
      padding: '12px 24px',
      borderRadius: '10px',
      fontSize: '14px',
      fontWeight: '600',
      color: '#fff',
      background: isError ? '#dc2626' : '#1e293b',
      boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
      zIndex: '99999',
      transition: 'opacity 0.3s',
      opacity: '0',
    });

    document.body.appendChild(toast);
    requestAnimationFrame(() => { toast.style.opacity = '1'; });
    setTimeout(() => {
      toast.style.opacity = '0';
      setTimeout(() => toast.remove(), 300);
    }, 3000);
  }, []);

  const success = useCallback((msg: string) => show(msg, false), [show]);
  const error = useCallback((msg: string) => show(msg, true), [show]);

  return { show, success, error };
}
