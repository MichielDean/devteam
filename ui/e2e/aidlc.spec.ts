import { test, expect } from '@playwright/test';

test.describe('AIDLC v2 Web UI', () => {

  test('dashboard loads with features', async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });

    await page.goto('/');
    await expect(page.locator('h2')).toContainText('Features');
    expect(consoleErrors).toEqual([]);
  });

  test('intake form shows scope selector with auto-detect', async ({ page }) => {
    await page.goto('/');
    await page.click('[data-testid="create-feature-button"]');

    await expect(page.locator('[data-testid="intake-form"]')).toBeVisible();
    await expect(page.locator('[data-testid="scope-select"]')).toBeVisible();
    await expect(page.locator('[data-testid="depth-select"]')).toBeVisible();

    // Type a bugfix intent — should auto-detect
    await page.fill('[data-testid="title-input"]', 'Fix crash');
    await page.fill('[data-testid="description-input"]', 'fix bug');
    await expect(page.locator('[data-testid="scope-hint"]')).toContainText('Auto-detected: Bug Fix');
  });

  test('intake form auto-detects POC scope', async ({ page }) => {
    await page.goto('/');
    await page.click('[data-testid="create-feature-button"]');

    await page.fill('[data-testid="title-input"]', 'POC auth');
    await page.fill('[data-testid="description-input"]', 'proof of concept');
    await expect(page.locator('[data-testid="scope-hint"]')).toContainText('Proof of Concept');
  });

  test('intake form auto-detects security-patch scope', async ({ page }) => {
    await page.goto('/');
    await page.click('[data-testid="create-feature-button"]');

    await page.fill('[data-testid="title-input"]', 'CVE-2024 fix');
    await page.fill('[data-testid="description-input"]', 'security patch CVE');
    await expect(page.locator('[data-testid="scope-hint"]')).toContainText('Security Patch');
  });

  test('feature detail shows stage progress', async ({ page }) => {
    await page.goto('/');
    const card = page.locator('[data-testid*="feature-card"]').first();
    if (await card.count() > 0) {
      await card.click();
      await page.waitForLoadState('networkidle');
      // Stage progress should be visible
      await expect(page.locator('[data-testid="stage-progress"]')).toBeVisible();
    }
  });

  test('feature detail shows audit timeline', async ({ page }) => {
    await page.goto('/');
    const card = page.locator('[data-testid*="feature-card"]').first();
    if (await card.count() > 0) {
      await card.click();
      await page.waitForLoadState('networkidle');
      await expect(page.locator('[data-testid="audit-timeline"]')).toBeVisible();
    }
  });

  test('feature detail shows scope and depth', async ({ page }) => {
    await page.goto('/');
    const card = page.locator('[data-testid*="feature-card"]').first();
    if (await card.count() > 0) {
      await card.click();
      await page.waitForLoadState('networkidle');
      await expect(page.locator('[data-testid="feature-scope"]')).toBeVisible();
      await expect(page.locator('[data-testid="feature-depth"]')).toBeVisible();
    }
  });

  test('feature card shows scope badge', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    const card = page.locator('[data-testid*="feature-card"]').first();
    if (await card.count() > 0) {
      await expect(card.locator('[data-testid="feature-card-scope"]')).toBeVisible();
    }
  });

  test('no console errors on dashboard', async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    expect(consoleErrors).toEqual([]);
  });

  test('no console errors on feature detail', async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });

    await page.goto('/');
    const card = page.locator('[data-testid*="feature-card"]').first();
    if (await card.count() > 0) {
      await card.click();
      await page.waitForLoadState('networkidle');
    }
    expect(consoleErrors).toEqual([]);
  });
});