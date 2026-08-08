import { test, expect } from '@playwright/test';

test.describe('Target Management', () => {
  let mockTargets: any[];

  test.beforeEach(async ({ page }) => {
    mockTargets = [
      {
        id: 'target-1',
        name: 'Test Target 1',
        baseUrl: 'http://test-1.example.com',
        environment: 'development',
        description: 'First test target',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
    ];

    // Mock the API state check (runs on page load for run-console)
    await page.route('**/healthz', async route => {
      await route.fulfill({ status: 200, body: 'ok' });
    });

    // Mock initial targets fetch
    await page.route('**/v1/targets', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ targets: mockTargets }),
        });
      }
    });

    // Mock initial runs fetch
    await page.route('**/v1/runs', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ runs: [] }),
        });
      }
    });

    await page.goto('/');
  });

  test('should display list of targets', async ({ page }) => {
    // Wait for the targets section to appear
    const targetSection = page.locator('section[aria-label="Target list"]');
    await expect(targetSection).toBeVisible();

    // Verify the mock target is displayed
    await expect(targetSection.locator('text=Test Target 1')).toBeVisible();
    await expect(targetSection.locator('text=http://test-1.example.com')).toBeVisible();
    await expect(targetSection.locator('text=development')).toBeVisible();
  });

  test('should create a new target', async ({ page }) => {
    // Mock POST request to create target
    await page.route('**/v1/targets', async route => {
      if (route.request().method() === 'POST') {
        const body = JSON.parse(route.request().postData() || '{}');
        const newTarget = {
          id: 'target-2',
          ...body,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };

        // Update the mock state so the next GET returns both targets
        mockTargets.push(newTarget);

        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify(newTarget),
        });
      } else if (route.request().method() === 'GET') {
         await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ targets: mockTargets }),
        });
      } else {
        await route.continue();
      }
    });

    // Wait for the targets section to appear
    const targetManagerSection = page.locator('section[aria-label="Target Manager"]');

    // Click "New Target" button
    await targetManagerSection.locator('button:has-text("New Target")').click();

    // Fill the form
    await targetManagerSection.locator('input[placeholder="E.g. Production Search API"]').fill('New E2E Target');
    await targetManagerSection.locator('input[placeholder="https://api.example.com"]').fill('https://e2e.example.com');
    await targetManagerSection.locator('select').selectOption('staging');
    await targetManagerSection.locator('textarea[placeholder="Optional details about this target"]').fill('Created by E2E test');

    // Click Save
    await targetManagerSection.locator('button:has-text("Save Target")').click();

    // Verify the new target is visible in the list
    await expect(page.locator('text=New E2E Target')).toBeVisible();
    await expect(page.locator('text=https://e2e.example.com')).toBeVisible();
    await expect(page.locator('text=staging')).toBeVisible();
  });

  test('should delete a target', async ({ page }) => {
     // Wait for the targets section to appear
    const targetSection = page.locator('section[aria-label="Target list"]');
    await expect(targetSection).toBeVisible();

    // Ensure the target is visible
    await expect(targetSection.locator('text=Test Target 1')).toBeVisible();

    // Mock DELETE request
    await page.route('**/v1/targets/target-1', async route => {
      if (route.request().method() === 'DELETE') {
        // Remove from mock array
        mockTargets.splice(0, mockTargets.length);

        await route.fulfill({
          status: 204,
        });
      } else {
        await route.continue();
      }
    });

    // Mock GET request to return empty array after deletion
    await page.route('**/v1/targets', async route => {
      if (route.request().method() === 'GET') {
         await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ targets: mockTargets }),
        });
      } else {
        await route.continue();
      }
    });

    // Accept any JS dialogs (confirmations)
    page.on('dialog', dialog => dialog.accept());

    // Click Delete button for the first target
    await targetSection.locator('button[title="Delete"]').click();

    // Verify the target is no longer in the list
    await expect(targetSection.locator('text=Test Target 1')).not.toBeVisible();
  });
});
