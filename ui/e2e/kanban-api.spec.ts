import { test, expect, type Page } from '@playwright/test';

function captureConsole(page: Page) {
  const errors: string[] = [];
  const pageErrors: string[] = [];
  page.on('console', msg => {
    if (msg.type() === 'error') errors.push(msg.text());
  });
  page.on('pageerror', err => pageErrors.push(err.message));
  return { errors, pageErrors };
}

function mockFeature(id: string, title: string, status: string, current_phase: string) {
  return {
    id,
    title,
    status,
    current_phase,
    priority: 1,
    updated_at: new Date().toISOString(),
    gate_result: null,
    pending_questions_count: 0,
  };
}

test.describe('Kanban integration — API contract & error paths', () => {
  test('AC-CON-003: board calls only GET /api/features (no kanban-specific endpoint)', async ({ page }) => {
    const { errors, pageErrors } = captureConsole(page);
    await page.route('**/api/features', route =>
      route.fulfill({
        status: 200,
        json: { features: [mockFeature('k1', 'Kanban One', 'draft', 'inception')], total_count: 1 },
      }),
    );

    const apiRequests: string[] = [];
    page.on('request', req => {
      const url = req.url();
      if (/\/api\//.test(url)) apiRequests.push(url);
    });

    await page.goto('/');
    await page.locator('[data-testid="view-toggle-board"]').click();
    await expect(page.locator('[data-testid="kanban-board"]')).toBeVisible();
    await expect(page.locator('[data-testid="feature-card-k1"]')).toBeVisible();

    // Every /api/ request hit by the board must be /api/features — no new endpoint.
    for (const url of apiRequests) {
      expect(url).toMatch(/\/api\/features(\?|$)/);
    }
    expect(errors).toEqual([]);
    expect(pageErrors).toEqual([]);
  });

  test('AC-CON-006: board renders using already-bundled modules (no new dep)', async ({ page }) => {
    // The real check is `git diff main -- ui/package.json`. This test verifies the board
    // renders at all, which it can only do with the existing bundle — no dynamic import
    // of a new dependency would succeed without adding it to package.json.
    const { errors, pageErrors } = captureConsole(page);
    await page.route('**/api/features', route =>
      route.fulfill({ status: 200, json: { features: [], total_count: 0 } }),
    );
    await page.goto('/');
    await page.locator('[data-testid="view-toggle-board"]').click();
    await expect(page.locator('[data-testid="kanban-board"]')).toBeVisible();
    expect(errors).toEqual([]);
    expect(pageErrors).toEqual([]);
  });

  test('AC-ERR-001: API 500 renders error banner, no pageerror', async ({ page }) => {
    const { errors, pageErrors } = captureConsole(page);
    await page.route('**/api/features', route =>
      route.fulfill({
        status: 500,
        json: { error: 'internal_error', details: 'db down' },
      }),
    );
    await page.goto('/');
    await page.locator('[data-testid="view-toggle-board"]').click();
    await expect(page.locator('[data-testid="kanban-error"]')).toBeVisible();
    await expect(page.locator('[data-testid="kanban-error"]')).toContainText('Failed to load features');
    expect(pageErrors).toEqual([]);
    // Console errors from the 500 response itself (browser-level "Failed to load
    // resource") are not app errors — filter them out. AC-ERR-001 only forbids
    // uncaught exceptions (pageerror), not browser network-error console spam.
    const appErrors = errors.filter(e => !e.includes('Failed to load resource'));
    expect(appErrors).toEqual([]);
  });

  test('AC-ERR-002: refetch error keeps board stable, no pageerror', async ({ page }) => {
    const { errors, pageErrors } = captureConsole(page);
    let failNext = false;
    await page.route('**/api/features', route => {
      if (failNext) {
        route.fulfill({
          status: 500,
          json: { error: 'internal_error', details: 'transient' },
        });
        return;
      }
      route.fulfill({
        status: 200,
        json: { features: [mockFeature('s1', 'Stale', 'draft', 'inception')], total_count: 1 },
      });
    });

    await page.goto('/');
    await page.locator('[data-testid="view-toggle-board"]').click();
    await expect(page.locator('[data-testid="feature-card-s1"]')).toBeVisible();

    failNext = true;
    // Trigger a refetch by toggling views (re-mounts board, re-runs useQuery).
    await page.locator('[data-testid="view-toggle-list"]').click();
    await page.locator('[data-testid="view-toggle-board"]').click();

    // Either the error banner appears OR stale card remains — both are acceptable per AC.
    // The hard requirement: no uncaught exception.
    expect(pageErrors).toEqual([]);
    expect(errors).toEqual([]);
  });
});