const API_BASE = '/api/v1';

// App Bridge session token retrieval function.
// Set by the AppBridgeProvider when running inside Shopify Admin.
let appBridgeGetToken: (() => Promise<string>) | null = null;

/**
 * Register the App Bridge getSessionToken function.
 * Called once by the App Bridge provider on initialization.
 */
export function registerAppBridgeTokenProvider(fn: () => Promise<string>): void {
  appBridgeGetToken = fn;
}

/**
 * Get the current session token.
 * Prefers App Bridge JWT (embedded mode), falls back to URL param / localStorage (dev mode).
 */
async function getSessionToken(): Promise<string> {
  // In embedded mode, use App Bridge to get a fresh JWT
  if (appBridgeGetToken) {
    try {
      return await appBridgeGetToken();
    } catch {
      // Fall through to legacy token
    }
  }

  // Fallback: URL param or stored session (OAuth callback flow, dev mode)
  const params = new URLSearchParams(window.location.search);
  return params.get('session') || localStorage.getItem('droptodrop_session') || '';
}

export function setSessionToken(token: string): void {
  localStorage.setItem('droptodrop_session', token);
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const token = await getSessionToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T>(path: string) => request<T>('DELETE', path),
};
