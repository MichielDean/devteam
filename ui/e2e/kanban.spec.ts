import { test, expect, type Page } from '@playwright/test';

// Kanban view E2E — covers AC-001..AC-019 from specs/kanban-view/acceptance.md.
// Every test stubs /api/features so assertions are deterministic regardless of
// the workspace's real feature set.

const PHASES = ['inception', 'planning', 'construction', 'review', 'testing', 'delivery'] as const;
const PHASE_LABELS: Record<string, string> = {
  inception: 'Inception',
  planning: 'Planning',
  construction: 'Construction',
  review: 'Review',
  testing: 'Testing',
  delivery: 'Delivery',
};

type FeatureFixture = {
  id: string;
  title: string;
  status: string;
  priority: number;
  current_phase: string;
  updated_at: string;
  gate_result: { passed: boolean; checks: unknown[] } | null;
  pending_questions_count: number;
};

function feature(partial: Partial<FeatureFixture> & { id: string }): FeatureFixture {
  return {
    title: `Feature ${partial.id}`,
    status: 'in_progress',
    priority: 2,
    current_phase: 'planning',
    updated_at: '2026-06-22T00:00:00Z',
    gate_result: null,
    pending_questions_count: 0,
    ...partial,
  };
}

async function stubFeatures(page: Page, features: FeatureFixture[], totalCount?: number) {
  await page.route('**/api/features', route =>
    route.fulfill({
      status: 200,
      json: { features, total_count: totalCount ?? features.length },
    })
  );
}

// Card root is an <a> with data-testid="feature-card-<id>"; sub-elements
// (feature-card-title, -status, etc.) share the prefix, so scope to <a>.
function cardLocator(page: Page) {
  return page.locator('a[data-testid^="feature-card-"]');
}

// Count only the six phase column sections, excluding header/empty testids.
function columnLocator(page: Page) {
  return page.locator(
    PHASES.map(p => `[data-testid="kanban-column-${p}"]`).join(',')
  );
}

async function expectNoConsoleErrors(page: Page, fn: () => Promise<void>) {
  const errors: string[] = [];
  page.on('pageerror', err => errors.push(`pageerror: ${err.message}`));
  page.on('console', msg => {
    if (msg.type() === 'error') errors.push(`console: ${msg.text()}`);
  });
  await fn();
  expect(errors, `console errors: ${errors.join('\n')}`).toEqual([]);
}

// ---------------------------------------------------------------------------
// US-001 — Kanban board view
// ---------------------------------------------------------------------------

test.describe('Kanban view — US-001', () => {
  test('AC-001 [e2e] toggling to Kanban renders six labelled columns with cards in the right column', async ({ page }) => {
    await stubFeatures(page, [
      feature({ id: 'plan-1', current_phase: 'planning' }),
      feature({ id: 'cept-1', current_phase: 'inception' }),
    ]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();

    for (const phase of PHASES) {
      const col = page.getByTestId(`kanban-column-${phase}`);
      await expect(col).toBeVisible();
      await expect(col.getByTestId(`kanban-column-header-${phase}`)).toContainText(PHASE_LABELS[phase]);
    }
    // planning feature inside planning column
    const planningCol = page.getByTestId('kanban-column-planning');
    await expect(planningCol.locator('a[data-testid^="feature-card-"]')).toHaveCount(1);
    await expect(planningCol.getByTestId('feature-card-plan-1')).toBeVisible();
    await expect(page.getByTestId('kanban-column-inception').getByTestId('feature-card-cept-1')).toBeVisible();
  });

  test('AC-002 [e2e] clicking a kanban card navigates to /features/:id via client-side routing', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'abc123', current_phase: 'planning' })]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();

    const fullNavigations: string[] = [];
    page.on('request', req => {
      if (req.resourceType() === 'document') fullNavigations.push(req.url());
    });

    await page.getByTestId('feature-card-abc123').click();
    await expect(page).toHaveURL(/\/features\/abc123$/);
    expect(fullNavigations.filter(u => /\/features\/abc123$/.test(u))).toEqual([]);
  });

  test('AC-003 [e2e] clicking List toggle removes board and restores FeatureList', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'x1', current_phase: 'delivery' })]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    await expect(page.getByTestId('kanban-board')).toBeVisible();

    await page.getByTestId('view-toggle-list').click();
    await expect(page.getByTestId('feature-list')).toBeVisible();
    await expect(page.getByTestId('kanban-board')).toHaveCount(0);
    await expect(page.locator('[data-testid^="kanban-column-"]')).toHaveCount(0);
  });

  test('AC-004 [integration] toggling views does not issue a second /api/features request', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'q1', current_phase: 'planning' })]);

    const requests: string[] = [];
    page.on('request', req => {
      if (req.url().includes('/api/features')) requests.push(req.url());
    });

    await page.goto('/');
    await expect(page.getByTestId('feature-list')).toBeVisible();
    await page.getByTestId('view-toggle-kanban').click();
    await expect(page.getByTestId('kanban-board')).toBeVisible();
    await page.getByTestId('view-toggle-list').click();
    await page.getByTestId('view-toggle-kanban').click();

    // Wait for any pending network to settle.
    await page.waitForLoadState('networkidle');
    expect(requests.filter(u => /\/api\/features(\?|$)/.test(u)).length).toBe(1);
  });

  test('AC-005 [smoke] loading state shows features-loading and no columns', async ({ page }) => {
    // Route that never resolves.
    await page.route('**/api/features', route => {
      // Intentionally do not fulfill.
    });
    await page.addInitScript(() => {
      localStorage.setItem('devteam.dashboard.view', 'kanban');
    });
    await page.goto('/');
    await expect(page.getByTestId('features-loading')).toBeVisible();
    await expect(page.locator('[data-testid^="kanban-column-"]')).toHaveCount(0);
  });

  test('AC-006 [smoke] API error shows features-error and no columns', async ({ page }) => {
    await page.route('**/api/features', route =>
      route.fulfill({ status: 500, json: { error: 'internal_error', details: 'boom' } })
    );
    await page.addInitScript(() => {
      localStorage.setItem('devteam.dashboard.view', 'kanban');
    });
    await page.goto('/');
    await expect(page.getByTestId('features-error')).toBeVisible();
    await expect(page.locator('[data-testid^="kanban-column-"]')).toHaveCount(0);
  });

  test('AC-007 [e2e] zero features: six empty columns + EmptyState CTA', async ({ page }) => {
    await stubFeatures(page, [], 0);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    await expect(columnLocator(page)).toHaveCount(6);
    await expect(page.getByTestId('empty-state-create-button')).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// US-002 — View persistence
// ---------------------------------------------------------------------------

test.describe('Kanban view — US-002 persistence', () => {
  test('AC-008 [e2e] reload restores Kanban view from localStorage', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'p1', current_phase: 'inception' })]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    await expect(page.getByTestId('kanban-board')).toBeVisible();

    await page.reload();
    await expect(page.getByTestId('kanban-column-inception')).toBeVisible();
    const stored = await page.evaluate(() => localStorage.getItem('devteam.dashboard.view'));
    expect(stored).toBe('kanban');
  });

  test('AC-009 [e2e] cleared localStorage defaults to List view', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'd1', current_phase: 'planning' })]);
    await page.addInitScript(() => localStorage.clear());
    await page.goto('/');
    await expect(page.getByTestId('feature-list')).toBeVisible();
    await expect(page.locator('[data-testid^="kanban-column-"]')).toHaveCount(0);
  });

  test('AC-010 [unit] localStorage.setItem throwing does not crash — board still renders', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 't1', current_phase: 'planning' })]);
    await page.addInitScript(() => {
      const key = 'devteam.dashboard.view';
      const origSet = Storage.prototype.setItem;
      Storage.prototype.setItem = function (k: string) {
        if (k === key) throw new DOMException('quota', 'QuotaExceededError');
        return origSet.apply(this, arguments as unknown as [string, string]);
      };
    });
    const pageErrors: string[] = [];
    page.on('pageerror', err => pageErrors.push(err.message));
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    await expect(page.getByTestId('kanban-column-planning')).toBeVisible();
    expect(pageErrors).toEqual([]);
  });

  test('AC-011 [unit] localStorage.getItem throwing falls back to list, no crash', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'g1', current_phase: 'planning' })]);
    await page.addInitScript(() => {
      const key = 'devteam.dashboard.view';
      const origGet = Storage.prototype.getItem;
      Storage.prototype.getItem = function (k: string) {
        if (k === key) throw new DOMException('disabled', 'SecurityError');
        return origGet.apply(this, arguments as unknown as [string]);
      };
    });
    const pageErrors: string[] = [];
    page.on('pageerror', err => pageErrors.push(err.message));
    await page.goto('/');
    await expect(page.getByTestId('feature-list')).toBeVisible();
    expect(pageErrors).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// US-003 — Card information density
// ---------------------------------------------------------------------------

test.describe('Kanban view — US-003 card density', () => {
  test('AC-012 [integration] pending_questions_count badge shows the count', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'q3', current_phase: 'planning', pending_questions_count: 3 })]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    const card = page.getByTestId('feature-card-q3');
    await expect(card.getByTestId('question-badge')).toHaveText('3');
  });

  test('AC-013 [integration] gate_result.passed=false renders gate-failed indicator', async ({ page }) => {
    await stubFeatures(page, [
      feature({ id: 'gf', current_phase: 'planning', gate_result: { passed: false, checks: [] } }),
    ]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    const card = page.getByTestId('feature-card-gf');
    await expect(card.getByTestId('feature-card-gate')).toBeVisible();
    await expect(card.getByTestId('feature-card-gate')).toContainText(/failed/i);
  });

  test('AC-014 [integration] gate_result.passed=true renders gate-passed indicator', async ({ page }) => {
    await stubFeatures(page, [
      feature({ id: 'gp', current_phase: 'planning', gate_result: { passed: true, checks: [] } }),
    ]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    const card = page.getByTestId('feature-card-gp');
    await expect(card.getByTestId('feature-card-gate')).toBeVisible();
    await expect(card.getByTestId('feature-card-gate')).toContainText(/passed/i);
  });

  test('AC-015 [integration] gate_result=null renders no gate indicator', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'gn', current_phase: 'planning', gate_result: null })]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    await expect(page.getByTestId('feature-card-gn').getByTestId('feature-card-gate')).toHaveCount(0);
  });
});

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

test.describe('Kanban view — edge cases', () => {
  test('AC-016 [unit] unknown current_phase lands in Other column after Delivery', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'unk', current_phase: 'rolling_out' })]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();

    // six standard columns exist
    for (const phase of PHASES) {
      await expect(page.getByTestId(`kanban-column-${phase}`)).toBeVisible();
    }
    const other = page.getByTestId('kanban-column-other');
    await expect(other).toBeVisible();
    await expect(other.getByTestId('feature-card-unk')).toBeVisible();

    // Other is after Delivery: check DOM order.
    const order = await page.locator('[data-testid^="kanban-column-"]').evaluateAll(els =>
      els.map(e => e.getAttribute('data-testid'))
    );
    const deliveryIdx = order.indexOf('kanban-column-delivery');
    const otherIdx = order.indexOf('kanban-column-other');
    expect(otherIdx).toBeGreaterThan(deliveryIdx);
  });

  test('AC-017 [unit] no unknown phases → Other column absent', async ({ page }) => {
    await stubFeatures(page, [feature({ id: 'k1', current_phase: 'inception' })]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    await expect(page.getByTestId('kanban-column-other')).toHaveCount(0);
    await expect(columnLocator(page)).toHaveCount(6);
  });

  test('AC-018 [integration] one feature per phase → each column has exactly one card, total = 6', async ({ page }) => {
    const feats = PHASES.map((phase, i) => feature({ id: `f${i}`, current_phase: phase }));
    await stubFeatures(page, feats);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    for (const phase of PHASES) {
      await expect(page.getByTestId(`kanban-column-${phase}`).locator('a[data-testid^="feature-card-"]')).toHaveCount(1);
    }
    await expect(cardLocator(page)).toHaveCount(6);
  });

  test('AC-019 [integration] 50 features render with no console errors', async ({ page }) => {
    const feats: FeatureFixture[] = Array.from({ length: 50 }, (_, i) =>
      feature({ id: `big-${i}`, current_phase: PHASES[i % PHASES.length] })
    );
    await stubFeatures(page, feats);
    await expectNoConsoleErrors(page, async () => {
      await page.goto('/');
      await page.getByTestId('view-toggle-kanban').click();
      await expect(cardLocator(page)).toHaveCount(50);
    });
  });

  // Adversarial: agent failure mode #2 — null vs empty array. The board's
  // groupFeaturesByPhase guards `features ?? []`, but Dashboard derives
  // `features = data?.features ?? []` before rendering. A malformed 200
  // response with `features: null` must not crash the board or emit a
  // console error; it should render as an empty board (six empty columns).
  test('CON-011/adversarial [smoke] API returns features:null → six empty columns, no console error', async ({ page }) => {
    await page.route('**/api/features', route =>
      route.fulfill({ status: 200, json: { features: null as unknown as FeatureFixture[], total_count: 0 } })
    );
    await expectNoConsoleErrors(page, async () => {
      await page.goto('/');
      // total_count 0 → Dashboard takes the empty branch (EmptyState + six
      // empty columns in kanban view).
      await page.getByTestId('view-toggle-kanban').click();
      await expect(columnLocator(page)).toHaveCount(6);
      await expect(page.getByTestId('empty-state-create-button')).toBeVisible();
    });
  });

  // Adversarial: agent failure mode #1 — phantom method / crash on
  // unexpected payload. A feature missing current_phase (undefined) must
  // land in Other, not throw. groupFeaturesByPhase reads f.current_phase;
  // undefined is not in the known set → Other bucket.
  test('CON-011/adversarial [unit] feature missing current_phase → Other column, no crash', async ({ page }) => {
    await stubFeatures(page, [
      // current_phase deliberately absent (undefined serializes to omitted in JSON).
      { ...feature({ id: 'nop', current_phase: 'planning' }), current_phase: undefined as unknown as string },
    ]);
    await page.goto('/');
    await page.getByTestId('view-toggle-kanban').click();
    await expect(page.getByTestId('kanban-column-other')).toBeVisible();
    await expect(page.getByTestId('kanban-column-other').getByTestId('feature-card-nop')).toBeVisible();
  });
});