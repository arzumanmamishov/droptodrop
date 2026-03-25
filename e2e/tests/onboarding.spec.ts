import { test, expect } from '@playwright/test';

test.describe('Onboarding', () => {
  test('unauthenticated user sees install message', async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(2000);

    // Without a valid session, should show install prompt or loading
    const hasInstallMsg = await page.locator('text=install the app').isVisible();
    const hasLoading = await page.locator('text=Loading').isVisible();
    const hasOnboarding = await page.locator('text=Welcome to DropToDrop').isVisible();

    expect(hasInstallMsg || hasLoading || hasOnboarding).toBeTruthy();
  });

  test('health endpoint returns ok', async ({ request }) => {
    const response = await request.get('http://localhost:8080/health');
    expect(response.ok()).toBeTruthy();

    const body = await response.json();
    expect(body.status).toBe('ok');
  });

  test('API requires authentication', async ({ request }) => {
    const response = await request.get('http://localhost:8080/api/v1/shop');
    expect(response.status()).toBe(401);
  });

  test('API accepts valid session token', async ({ request }) => {
    const response = await request.get('http://localhost:8080/api/v1/shop', {
      headers: {
        Authorization: 'Bearer dev_supplier_session_token',
      },
    });
    expect(response.ok()).toBeTruthy();

    const body = await response.json();
    expect(body.role).toBe('supplier');
    expect(body.shopify_domain).toBe('supplier-demo.myshopify.com');
  });

  test('API rejects invalid session token', async ({ request }) => {
    const response = await request.get('http://localhost:8080/api/v1/shop', {
      headers: {
        Authorization: 'Bearer invalid_token_here',
      },
    });
    expect(response.status()).toBe(401);
  });

  test('webhook endpoint rejects without HMAC', async ({ request }) => {
    const response = await request.post('http://localhost:8080/webhooks/app/uninstalled', {
      data: { shop_domain: 'test.myshopify.com' },
      headers: { 'Content-Type': 'application/json' },
    });
    expect(response.status()).toBe(401);
  });

  test('compliance endpoint rejects without HMAC', async ({ request }) => {
    const endpoints = [
      '/webhooks/compliance/customers-data-request',
      '/webhooks/compliance/customers-redact',
      '/webhooks/compliance/shop-redact',
    ];

    for (const endpoint of endpoints) {
      const response = await request.post(`http://localhost:8080${endpoint}`, {
        data: { shop_domain: 'test.myshopify.com' },
        headers: { 'Content-Type': 'application/json' },
      });
      expect(response.status()).toBe(401);
    }
  });
});
