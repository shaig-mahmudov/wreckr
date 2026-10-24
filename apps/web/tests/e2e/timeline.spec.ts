import { test, expect } from '@playwright/test';

test.describe('Live Timeline Rendering', () => {
  const mockRunId = 'run-123';
  const mockRuns = [
    {
      id: mockRunId,
      status: 'running',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
  ];

  const mockEvents = [
    {
      run_id: mockRunId,
      sequence: 1,
      type: 'run_started',
      level: 'info',
      message: 'Run has started',
      created_at: new Date().toISOString(),
    },
    {
      run_id: mockRunId,
      sequence: 2,
      type: 'request_sent',
      level: 'debug',
      message: 'Sent GET /api/test',
      created_at: new Date().toISOString(),
    }
  ];

  test.beforeEach(async ({ page }) => {
    // Mock the API state check
    await page.route('**/healthz', async route => {
      await route.fulfill({ status: 200, body: 'ok' });
    });

    // Mock initial targets fetch
    await page.route('**/v1/targets', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ targets: [] }),
        });
      }
    });

    // Mock initial runs fetch to return our mock run
    await page.route('**/v1/runs', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ runs: mockRuns }),
        });
      }
    });

    // Mock the initial events fetch
    await page.route(`**/v1/runs/${mockRunId}/events`, async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ events: mockEvents }),
        });
      }
    });

    // We don't necessarily need to mock the SSE stream for a basic rendering test,
    // as the initial fetch should populate the timeline. If the component falls back
    // to streaming, Playwright might just ignore the unhandled route or we can mock it.
    await page.route(`**/v1/runs/${mockRunId}/events/stream`, async route => {
      await route.fulfill({
          status: 200,
          contentType: 'text/event-stream',
          body: '',
      });
    });

    await page.goto('/');
  });

  test('should render run events in the timeline', async ({ page }) => {
    // Select the run from the list to trigger event loading
    // Assuming the first run in the list is our mock run
    await page.click(`text=${mockRunId}`);

    // Wait for the timeline section to appear
    const timelineSection = page.locator('section[aria-label="Run event timeline"]');
    await expect(timelineSection).toBeVisible();

    // Verify the events are rendered
    await expect(page.locator('text=run_started')).toBeVisible();
    await expect(page.locator('text=Run has started')).toBeVisible();

    await expect(page.locator('text=request_sent')).toBeVisible();
    await expect(page.locator('text=Sent GET /api/test')).toBeVisible();
  });
});
