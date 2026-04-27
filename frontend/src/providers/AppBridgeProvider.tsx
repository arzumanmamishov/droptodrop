import { useEffect, useState, ReactNode } from 'react';
import { registerAppBridgeTokenProvider } from '../utils/api';

declare global {
  interface Window {
    shopify?: {
      idToken: () => Promise<string>;
      toast: {
        show: (message: string, options?: { isError?: boolean; duration?: number }) => void;
      };
      config: {
        apiKey: string;
        shop: string;
      };
    };
  }
  // App Bridge web components
  namespace JSX {
    interface IntrinsicElements {
      'ui-nav-menu': React.DetailedHTMLProps<React.HTMLAttributes<HTMLElement>, HTMLElement>;
    }
  }
}

interface AppBridgeProviderProps {
  children: ReactNode;
}

// Register App Bridge immediately (not in useEffect) so it's available
// before any child component's useEffect runs.
function initAppBridge() {
  if (window.shopify?.idToken) {
    registerAppBridgeTokenProvider(() => window.shopify!.idToken());
    return true;
  }
  return false;
}

// Try to register immediately on module load
const initializedEarly = initAppBridge();

export default function AppBridgeProvider({ children }: AppBridgeProviderProps) {
  const [ready, setReady] = useState(initializedEarly);

  useEffect(() => {
    if (ready) return;

    // If not ready yet, poll briefly for window.shopify to become available
    let attempts = 0;
    const interval = setInterval(() => {
      if (initAppBridge() || attempts > 20) {
        clearInterval(interval);
        setReady(true);
      }
      attempts++;
    }, 100);

    return () => clearInterval(interval);
  }, [ready]);

  if (!ready) {
    return null;
  }

  return <>{children}</>;
}
