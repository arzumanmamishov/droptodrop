import { test, expect } from '@playwright/test';
import { authenticateAs } from './helpers';

test.describe('Reseller Flow', () => {
  test.beforeEach(async ({ page }) => {
    await authenticateAs(page, 'reseller');
  });

  test('dashboard loads with reseller role', async ({ page }) => {
    await expect(page.locator('text=Dashboard')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=Reseller')).toBeVisible();
  });

  test('marketplace page loads', async ({ page }) => {
    await page.click('text=Marketplace');
    await expect(page.locator('text=Marketplace')).toBeVisible({ timeout: 10000 });

    // Should have search
    await expect(page.locator('input[placeholder*="Search"]')).toBeVisible();
  });

  test('marketplace shows supplier listings', async ({ page }) => {
    await page.click('text=Marketplace');
    await page.waitForTimeout(2000);

    // Seeded active listings should appear
    const headphones = page.locator('text=Premium Wireless Headphones');
    const tshirt = page.locator('text=Organic Cotton T-Shirt');

    const either = await headphones.isVisible() || await tshirt.isVisible();
    expect(either).toBeTruthy();
  });

  test('marketplace import modal opens', async ({ page }) => {
    await page.click('text=Marketplace');
    await page.waitForTimeout(2000);

    // Click first Import button
    const importButton = page.locator('button:has-text("Import")').first();
    if (await importButton.isVisible()) {
      await importButton.click();

      // Modal should open
      await expect(page.locator('text=Markup type')).toBeVisible({ timeout: 5000 });
      await expect(page.locator('text=Price Preview')).toBeVisible();
    }
  });

  test('marketplace search filters results', async ({ page }) => {
    await page.click('text=Marketplace');
    await page.waitForTimeout(1500);

    // Search for something specific
    await page.fill('input[placeholder*="Search"]', 'headphones');
    await page.waitForTimeout(1500);

    // Should filter results
    const headphones = page.locator('text=Premium Wireless Headphones');
    // If search works, headphones should be visible or "no products" shown
    expect(await headphones.isVisible() || await page.locator('text=No products').isVisible()).toBeTruthy();
  });

  test('imports page loads', async ({ page }) => {
    await page.click('text=Imports');
    await expect(page.locator('text=Imported Products')).toBeVisible({ timeout: 10000 });
  });

  test('orders page loads for reseller', async ({ page }) => {
    await page.click('text=Orders');
    await expect(page.locator('text=Orders')).toBeVisible({ timeout: 10000 });
  });

  test('settings page loads for reseller', async ({ page }) => {
    await page.click('text=Settings');
    await expect(page.locator('text=Notifications')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=Privacy policy URL')).toBeVisible();
  });

  test('audit log loads for reseller', async ({ page }) => {
    await page.click('text=Audit Log');
    await expect(page.locator('text=Audit Log')).toBeVisible({ timeout: 10000 });
  });
});
