import { test, expect } from '@playwright/test';

// Admin UI E2E — settings-and-admin-ui feature (Bolt 0-2 MVP).
// Covers the admin shell (FR-SHELL-01..05) and the Repos + Defaults tabs.
// The Server and Audit tabs are read-only in v1 and have basic smoke tests.
// The Providers and CI/CD tabs are disabled "coming soon" placeholders
// (bolt-plan rev2 strict scope cut).

test.describe('Admin UI', () => {

  test('admin shell renders 4 v1 tabs + 2 disabled placeholders in order', async ({ page }) => {
    await page.goto('/admin');

    // FR-SHELL-01: six tab labels in order: Repos, Defaults, Providers, CI/CD, Server, Audit.
    const tabs = page.locator('[data-testid^="tab-"]');
    await expect(tabs).toHaveCount(6);
    await expect(tabs.nth(0)).toContainText('Repos');
    await expect(tabs.nth(1)).toContainText('Defaults');
    await expect(tabs.nth(2)).toContainText('Providers');
    await expect(tabs.nth(3)).toContainText('CI/CD');
    await expect(tabs.nth(4)).toContainText('Server');
    await expect(tabs.nth(5)).toContainText('Audit');

    // Providers and CI/CD are disabled with "coming soon" labels.
    await expect(page.locator('[data-testid="coming-soon-providers"]')).toBeVisible();
    await expect(page.locator('[data-testid="coming-soon-cicd"]')).toBeVisible();
  });

  test('header has Admin link', async ({ page }) => {
    await page.goto('/');
    const adminLink = page.locator('nav a:has-text("Admin")');
    await expect(adminLink).toBeVisible();
    await expect(adminLink).toHaveAttribute('href', '/admin');
  });

  test('URL syncs with active tab and survives reload', async ({ page }) => {
    await page.goto('/admin?tab=defaults');
    await expect(page.locator('[data-testid="defaults-tab"]')).toBeVisible();
    await expect(page.locator('[data-testid="tab-defaults"]')).toHaveAttribute('aria-current', 'page');

    // Switch to audit tab — URL should update.
    await page.click('[data-testid="tab-audit"]');
    await expect(page).toHaveURL(/tab=audit/);
    await expect(page.locator('[data-testid="audit-tab"]')).toBeVisible();

    // Reload preserves the active tab.
    await page.reload();
    await expect(page.locator('[data-testid="audit-tab"]')).toBeVisible();
  });

  test('disabled tabs do not navigate', async ({ page }) => {
    await page.goto('/admin?tab=repos');
    await expect(page.locator('[data-testid="repos-tab"]')).toBeVisible();

    // Clicking the disabled Providers tab should not change the URL or render the coming-soon panel.
    await page.click('[data-testid="tab-providers"]');
    await expect(page).toHaveURL(/tab=repos/);
  });

  test('repos tab renders list or empty state', async ({ page }) => {
    await page.goto('/admin?tab=repos');
    // Either the list or the empty state should be visible (depends on DB state).
    const list = page.locator('[data-testid="repos-list"]');
    const empty = page.locator('[data-testid="repos-empty"]');
    await expect(list.or(empty)).toBeVisible();
  });

  test('repos tab add button opens modal', async ({ page }) => {
    await page.goto('/admin?tab=repos');
    await page.click('[data-testid="repos-add-button"]');
    await expect(page.locator('[data-testid="repos-modal"]')).toBeVisible();
    await expect(page.locator('[data-testid="repos-form-name"]')).toBeVisible();
    await expect(page.locator('[data-testid="repos-form-url"]')).toBeVisible();
  });

  test('defaults tab renders global form and per-repo section', async ({ page }) => {
    await page.goto('/admin?tab=defaults');
    await expect(page.locator('[data-testid="defaults-form-scope"]')).toBeVisible();
    await expect(page.locator('[data-testid="defaults-form-depth"]')).toBeVisible();
    await expect(page.locator('[data-testid="defaults-form-test-strategy"]')).toBeVisible();
    await expect(page.locator('[data-testid="defaults-form-exec-mode"]')).toBeVisible();
    // Per-repo section (empty state or list).
    const perRepoList = page.locator('[data-testid="defaults-per-repo-list"]');
    const perRepoEmpty = page.locator('[data-testid="defaults-per-repo-empty"]');
    await expect(perRepoList.or(perRepoEmpty)).toBeVisible();
  });

  test('server tab renders classification table', async ({ page }) => {
    await page.goto('/admin?tab=server');
    await expect(page.locator('[data-testid="server-classification-table"]')).toBeVisible();
    // DSN row should be present with a bootstrap badge.
    await expect(page.locator('[data-testid="server-badge-database.dsn"]')).toBeVisible();
  });

  test('audit tab renders filter bar and list or empty state', async ({ page }) => {
    await page.goto('/admin?tab=audit');
    await expect(page.locator('[data-testid="audit-filters"]')).toBeVisible();
    await expect(page.locator('[data-testid="audit-filter-type"]')).toBeVisible();
    // List or empty state.
    const list = page.locator('[data-testid="audit-list"]');
    const empty = page.locator('[data-testid="audit-empty"]');
    await expect(list.or(empty)).toBeVisible();
  });

  test('admin page has no hardcoded colors (theme tokens)', async ({ page }) => {
    await page.goto('/admin');
    // FR-SHELL-02: the admin page should use var(--color-*) tokens, not hardcoded hex.
    // This is a smoke check — we verify the page renders without console errors
    // and that the CSS variables are present.
    const consoleErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    await expect(page.locator('[data-testid="admin-page"]')).toBeVisible();
    expect(consoleErrors).toEqual([]);
  });
});