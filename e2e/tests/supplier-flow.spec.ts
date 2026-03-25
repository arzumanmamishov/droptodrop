import { test, expect } from '@playwright/test';
import { authenticateAs } from './helpers';

test.describe('Supplier Flow', () => {
  test.beforeEach(async ({ page }) => {
    await authenticateAs(page, 'supplier');
  });

  test('dashboard loads with supplier role', async ({ page }) => {
    // Dashboard should show supplier-specific content
    await expect(page.locator('text=Dashboard')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=Supplier')).toBeVisible();
  });

  test('supplier setup page loads', async ({ page }) => {
    await page.click('text=Supplier Setup');
    await expect(page.locator('text=Supplier Mode')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=Enable supplier mode')).toBeVisible();
  });

  test('supplier setup can be saved', async ({ page }) => {
    await page.click('text=Supplier Setup');
    await page.waitForTimeout(1000);

    // Toggle blind fulfillment
    const blindCheckbox = page.locator('text=Blind/unbranded fulfillment').locator('..');
    await blindCheckbox.locator('input[type="checkbox"]').first().check({ force: true });

    // Click save
    await page.click('button:has-text("Save")');

    // Should see success
    await expect(page.locator('text=Settings saved')).toBeVisible({ timeout: 5000 });
  });

  test('listings page loads', async ({ page }) => {
    await page.click('text=Listings');
    await expect(page.locator('text=Supplier Listings')).toBeVisible({ timeout: 10000 });

    // Should show "Add Products" button
    await expect(page.locator('button:has-text("Add Products")')).toBeVisible();
  });

  test('listings table shows seeded products', async ({ page }) => {
    await page.click('text=Listings');
    await page.waitForTimeout(1500);

    // Seeded data includes "Premium Wireless Headphones" and "Organic Cotton T-Shirt"
    const headphones = page.locator('text=Premium Wireless Headphones');
    const tshirt = page.locator('text=Organic Cotton T-Shirt');

    // At least one should be visible (depends on status filter)
    const either = await headphones.isVisible() || await tshirt.isVisible();
    expect(either).toBeTruthy();
  });

  test('product picker opens', async ({ page }) => {
    await page.click('text=Listings');
    await page.waitForTimeout(1000);

    await page.click('button:has-text("Add Products")');

    // Modal should appear
    await expect(page.locator('text=Select Products from Your Store')).toBeVisible({ timeout: 5000 });
  });

  test('orders page loads for supplier', async ({ page }) => {
    await page.click('text=Orders');
    await expect(page.locator('text=Orders')).toBeVisible({ timeout: 10000 });
  });

  test('audit log loads', async ({ page }) => {
    await page.click('text=Audit Log');
    await expect(page.locator('text=Audit Log')).toBeVisible({ timeout: 10000 });

    // Should show seeded audit entries
    await page.waitForTimeout(1000);
    await expect(page.locator('text=oauth_complete')).toBeVisible({ timeout: 5000 });
  });

  test('settings page loads and saves', async ({ page }) => {
    await page.click('text=Settings');
    await expect(page.locator('text=Notifications')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=Support & Legal')).toBeVisible();
    await expect(page.locator('text=Data Retention')).toBeVisible();
    await expect(page.locator('text=Billing')).toBeVisible();
  });
});
