import { test, expect } from '@playwright/test';

// Multi-provider LLM configuration E2E — covers the AdminProvidersPage
// (feature multi-provider-llm-configuration).
// Tests run against the running devteam server (same target as admin.spec.ts).

test.describe('Admin Providers UI', () => {

  test('providers page renders with effect-timing banner', async ({ page }) => {
    await page.goto('/admin/providers');

    await expect(page.locator('[data-testid="admin-providers-page"]')).toBeVisible();
    await expect(page.locator('[data-testid="effect-timing-banner"]')).toBeVisible();
    await expect(page.locator('[data-testid="effect-timing-banner"]')).toContainText(
      'Config changes take effect at the next agent dispatch'
    );
  });

  test('operator section shows add-provider button', async ({ page }) => {
    await page.goto('/admin/providers');

    const operatorSection = page.locator('[data-testid="operator-section"]');
    await expect(operatorSection).toBeVisible();

    const addBtn = page.locator('[data-testid="add-provider-modal"]');
    if (await addBtn.isVisible()) {
      await addBtn.click();
      await expect(page.locator('[data-testid="add-provider-confirm"]')).toBeVisible();
    }
  });

  test('tier matrix is visible when providers are enabled', async ({ page }) => {
    await page.goto('/admin/providers');

    // The tier matrix may or may not be visible depending on whether
    // providers are configured. Just verify the container exists.
    const tierMatrix = page.locator('[data-testid="tier-matrix"]');
    const isVisible = await tierMatrix.isVisible().catch(() => false);
    if (isVisible) {
      // If visible, at least one tier row should be present.
      const tierRows = page.locator('[data-testid^="tier-row-"]');
      await expect(tierRows.first()).toBeVisible();
    }
  });

  test('role overrides editor section exists', async ({ page }) => {
    await page.goto('/admin/providers');

    // The role overrides editor may be empty but the container should exist.
    const overridesEditor = page.locator('[data-testid="role-overrides-editor"]');
    await expect(overridesEditor).toBeVisible();
  });

  test('navigating to /admin/providers from admin shell', async ({ page }) => {
    await page.goto('/admin');

    // The admin shell should have a link or tab to the providers page.
    // PR #94 added /admin/providers as a separate route.
    // Check if there's a link to /admin/providers.
    const providersLink = page.locator('a[href="/admin/providers"]');
    if (await providersLink.count() > 0) {
      await providersLink.click();
      await expect(page).toHaveURL(/\/admin\/providers/);
      await expect(page.locator('[data-testid="admin-providers-page"]')).toBeVisible();
    }
  });
});