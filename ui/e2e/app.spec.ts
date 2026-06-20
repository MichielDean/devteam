import { test, expect } from '@playwright/test';

test.describe('Dev Team Web UI', () => {

  test('feature list loads and shows features', async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto('/');

    await expect(page.locator('h2')).toContainText('Features');
    await expect(page.locator('[data-testid*="feature-card"]')).toHaveCount({ min: 1 });

    expect(consoleErrors).toEqual([]);
  });

  test('feature list handles empty state', async ({ page }) => {
    await page.goto('/');
    const features = page.locator('[data-testid*="feature-card"]');
    const count = await features.count();
    if (count > 0) {
      test.skip();
    }
    await expect(page.locator('text=No features')).toBeVisible();
  });

  test('feature detail page renders correctly', async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto('/');
    const firstFeature = page.locator('[data-testid*="feature-card"]').first();
    if (!(await firstFeature.isVisible())) {
      test.skip();
    }
    await firstFeature.click();

    await expect(page.locator('h1')).toBeVisible();
    await expect(page.locator('text=Pipeline Progress')).toBeVisible();

    expect(consoleErrors).toEqual([]);
  });

  test('new feature button opens form', async ({ page }) => {
    await page.goto('/');
    const newButton = page.locator('button:has-text("New Feature")');
    if (!(await newButton.isVisible())) {
      test.skip();
    }
    await newButton.click();
    await expect(page.locator('form, [role="dialog"], [data-testid="create-form"]')).toBeVisible();
  });

  test('phase progress indicators render', async ({ page }) => {
    await page.goto('/');
    const firstFeature = page.locator('[data-testid*="feature-card"]').first();
    if (!(await firstFeature.isVisible())) {
      test.skip();
    }
    await firstFeature.click();

    const phases = ['Inception', 'Planning', 'Construction', 'Review', 'Testing', 'Delivery'];
    for (const phase of phases) {
      await expect(page.locator(`text=${phase}`)).toBeVisible();
    }
  });

  test('API returns valid JSON with arrays not null', async ({ request }) => {
    const response = await request.get('/api/features');
    expect(response.ok()).toBeTruthy();

    const body = await response.json();

    expect(Array.isArray(body.features)).toBeTruthy();

    if (body.features.length > 0) {
      const featureResponse = await request.get(`/api/features/${body.features[0].id}`);
      expect(featureResponse.ok()).toBeTruthy();
      const feature = await featureResponse.json();

      expect(typeof feature.id).toBe('string');
      expect(typeof feature.title).toBe('string');
      expect(typeof feature.status).toBe('string');
      expect(typeof feature.current_phase).toBe('string');
      expect(typeof feature.phase_states).toBe('object');
      expect(feature.phase_states).not.toBeNull();

      for (const [, state] of Object.entries(feature.phase_states)) {
        const s = state as Record<string, unknown>;
        expect(Array.isArray(s.artifacts)).toBeTruthy();
        if (s.gate_result) {
          const gr = s.gate_result as Record<string, unknown>;
          expect(Array.isArray(gr.checks)).toBeTruthy();
          if (gr.missing_arts !== undefined) {
            expect(Array.isArray(gr.missing_arts)).toBeTruthy();
          }
        }
      }

      expect(Array.isArray(feature.dependencies)).toBeTruthy();
      expect(Array.isArray(feature.repos)).toBeTruthy();
    }
  });

  test('API 404 returns proper error for missing feature', async ({ request }) => {
    const response = await request.get('/api/features/nonexistent-feature-id');
    expect(response.status()).toBe(404);

    const body = await response.json();
    expect(body.error).toBeTruthy();
  });

  test('API 400 returns proper error for invalid create', async ({ request }) => {
    const response = await request.post('/api/features', {
      data: {},
    });
    expect(response.status()).toBe(400);

    const body = await response.json();
    expect(body.error).toBeTruthy();
  });
});