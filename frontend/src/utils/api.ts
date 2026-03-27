const API_BASE = '/api/v1';

let appBridgeGetToken: (() => Promise<string>) | null = null;

export function registerAppBridgeTokenProvider(fn: () => Promise<string>): void {
  appBridgeGetToken = fn;
}

async function getSessionToken(): Promise<string> {
  if (appBridgeGetToken) {
    try {
      return await appBridgeGetToken();
    } catch {
      // Fall through to legacy token
    }
  }
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
  retries = 2,
): Promise<T> {
  const token = await getSessionToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const response = await fetch(`${API_BASE}${path}`, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({ error: 'Request failed' }));
        const message = error.error || `HTTP ${response.status}`;

        // Don't retry client errors (4xx)
        if (response.status >= 400 && response.status < 500) {
          throw new Error(message);
        }

        // Retry server errors (5xx)
        if (attempt < retries) {
          await new Promise(r => setTimeout(r, 1000 * (attempt + 1)));
          continue;
        }
        throw new Error(message);
      }

      return response.json();
    } catch (err) {
      if (err instanceof TypeError && attempt < retries) {
        // Network error - retry
        await new Promise(r => setTimeout(r, 1000 * (attempt + 1)));
        continue;
      }
      throw err;
    }
  }

  throw new Error('Request failed after retries');
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T>(path: string) => request<T>('DELETE', path),
};
