import { useEffect, ReactNode } from 'react';
import { registerAppBridgeTokenProvider } from '../utils/api';

/**
 * AppBridgeProvider initializes the Shopify App Bridge session token flow.
 *
 * When running inside Shopify Admin, App Bridge provides `shopify.idToken()`
 * which returns a signed JWT. This provider registers that function so the
 * API client can retrieve fresh tokens for every request.
 *
 * When running outside Shopify (local dev), this is a no-op and the API client
 * falls back to the session token from OAuth callback / localStorage.
 *
 * Shopify App Bridge v4 automatically injects `window.shopify` when the app
 * is loaded inside the Shopify Admin iframe.
 */

// Type definition for the global shopify object injected by App Bridge
declare global {
  interface Window {
    shopify?: {
      idToken: () => Promise<string>;
      config: {
        apiKey: string;
        shop: string;
      };
    };
  }
}

interface AppBridgeProviderProps {
  children: ReactNode;
}

export default function AppBridgeProvider({ children }: AppBridgeProviderProps) {
  useEffect(() => {
    // Check if we're running inside Shopify Admin (App Bridge v4+ injects window.shopify)
    if (window.shopify?.idToken) {
      registerAppBridgeTokenProvider(() => window.shopify!.idToken());
    }
  }, []);

  return <>{children}</>;
}
