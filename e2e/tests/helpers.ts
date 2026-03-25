import { Page } from '@playwright/test';

/**
 * Authenticate as a specific role by navigating with a session token.
 * Uses the dev session tokens created by seed.sql.
 */
export async function authenticateAs(page: Page, role: 'supplier' | 'reseller') {
  const tokens: Record<string, string> = {
    supplier: 'dev_supplier_session_token',
    reseller: 'dev_reseller_session_token',
  };
  const token = tokens[role];

  // Navigate with session query param (simulates OAuth redirect)
  await page.goto(`/?session=${token}`);

  // Wait for the app to load
  await page.waitForSelector('[class*="Frame"]', { timeout: 10000 }).catch(() => {
    // Polaris Frame might not have a stable class; wait for any content
  });

  // Give React time to hydrate
  await page.waitForTimeout(1000);
}

/**
 * Navigate to a page within the app.
 */
export async function navigateTo(page: Page, path: string) {
  await page.goto(path);
  await page.waitForTimeout(500);
}

/**
 * Wait for an API response to complete.
 */
export async function waitForApi(page: Page, urlPattern: string) {
  await page.waitForResponse(
    (response) => response.url().includes(urlPattern) && response.status() === 200,
    { timeout: 10000 },
  );
}
