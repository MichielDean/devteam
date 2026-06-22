# Internal Component Contract: ViewToggle

This feature has **no HTTP API contracts** — it adds no endpoints, no request/response schemas, no backend changes. The contracts below are internal React component prop contracts that the Developer must implement to. They exist so the Reviewer and Tester can verify component boundaries without guessing.

---

## Component: `ViewToggle`

**File**: `ui/src/components/ViewToggle.tsx` (new)

**Purpose**: Segmented control with two options ("List", "Kanban") that switches the Dashboard's view. Persists selection to `localStorage`.

### Props

```typescript
interface ViewToggleProps {
  view: ViewMode;              // 'list' | 'kanban' — current active view
  onChange: (v: ViewMode) => void;  // called when user clicks an option
}
```

- `view` is controlled by the parent (Dashboard). The toggle does NOT own state — it only renders and emits `onChange`. Persistence is the parent's responsibility (single source of truth, one localStorage read/write site).

### Rendered DOM

```
<div data-testid="view-toggle" role="group" aria-label="Dashboard view">
  <button
    data-testid="view-toggle-list"
    aria-pressed={view === 'list'}
    className={view === 'list' ? <active classes> : <inactive classes>}
  >List</button>
  <button
    data-testid="view-toggle-kanban"
    aria-pressed={view === 'kanban'}
    className={view === 'kanban' ? <active classes> : <inactive classes>}
  >Kanban</button>
</div>
```

### Testid contract (CON-006, AC-005)

| testid | element |
|---|---|
| `view-toggle` | container `div` |
| `view-toggle-list` | List button |
| `view-toggle-kanban` | Kanban button |

### Accessibility

- `role="group"` on container with `aria-label="Dashboard view"`.
- `aria-pressed` on each button reflects active state (AC-005 verification target).
- Buttons are real `<button>` elements (keyboard-focusable, Enter/Space activate).

### Visibility

- `ViewToggle` is rendered by Dashboard **only when** `features.length > 0` and not loading and not error (FR-016, AC-020). The component itself does not hide itself — the parent controls mounting.

### Error / edge behavior

- No error paths. Clicking always calls `onChange` with the opposite or same value. Clicking the already-active option is a no-op (parent may ignore or re-set; idempotent).

---

## Component: `KanbanBoard`

**File**: `ui/src/components/KanbanBoard.tsx` (new)

**Purpose**: Renders six phase columns (+ optional "Other"), each containing ordered `FeatureCard`s. Pure presentational — no data fetching.

### Props

```typescript
interface KanbanBoardProps {
  features: FeatureSummary[];   // already-loaded, non-empty
}
```

### Exports

- `default` — the component.
- `orderCards(features: FeatureSummary[]): FeatureSummary[]` — pure sort helper (AC-018 unit test target). Exported from the same file.

### Rendered DOM

```
<div data-testid="kanban-board" className="flex gap-4 overflow-x-auto ...">
  {columns.map(col => (
    <KanbanColumn key={col.phase} column={col} />
  ))}
</div>
```

- `columns` is derived: for each `phase` in `PHASES`, build a `{ phase, label: PHASE_LABELS[phase], features: orderCards(features.filter(f => f.current_phase === phase)), count, testid: kanban-column-${phase} }`. Then, if any features have `current_phase` not in `PHASES`, append an `other` column (FR-017).
- `overflow-x-auto` on the board container provides horizontal scroll (FR-011, AC-014).
- Column width fixed (e.g. `min-w-[18rem]` or similar) so six columns exceed narrow viewports → scroll appears.

### Testid contract

| testid | element |
|---|---|
| `kanban-board` | board container |
| `kanban-column-{phase}` | each of the 6 phase column roots (inception, planning, construction, review, testing, delivery) |
| `kanban-column-other` | the fallback column, only when present |
| `kanban-column-empty-state` | the in-column empty message (one per empty column) |

### Invariants (verifiable)

1. Every `FeatureSummary` in `props.features` appears in exactly one column (FR-007, SC-002). `Σ column.count === features.length`.
2. Column DOM order for known phases === `PHASES` array order (AC-CON-004).
3. Cards inside each column are `FeatureCard` instances (CON-003) — no inline card markup.
4. Empty known columns still render with header, count `(0)`, and `kanban-column-empty-state` (FR-010, AC-011).
5. `other` column renders only when ≥1 feature has unrecognized `current_phase` (FR-017).

### Error / edge behavior

- No error paths. Input is always a non-empty `FeatureSummary[]` (Dashboard gates rendering). Empty input is undefined behavior — Dashboard never passes `[]` (it renders `EmptyState` instead).

---

## Component: `KanbanColumn`

**File**: `ui/src/components/KanbanColumn.tsx` (new)

**Purpose**: A single phase column — header (label + count), body (ordered cards or empty-state).

### Props

```typescript
interface KanbanColumnProps {
  phase: PhaseName | 'other';
  label: string;                // from PHASE_LABELS, or "Other"
  features: FeatureSummary[];   // already filtered + ordered for this column
}
```

### Rendered DOM

```
<div data-testid={`kanban-column-${phase}`} className="flex flex-col min-w-[...] ...">
  <div className="sticky top-0 ..." data-testid={`kanban-column-header-${phase}`}>
    <span>{label}</span>
    <span data-testid={`kanban-column-count-${phase}`}>({features.length})</span>
  </div>
  <div className="flex flex-col gap-2 ...">
    {features.length === 0 ? (
      <div data-testid="kanban-column-empty-state" className="...">No features in {label}</div>
    ) : (
      features.map(f => <FeatureCard key={f.id} feature={f} />)
    )}
  </div>
</div>
```

- `sticky top-0` on header keeps it visible during vertical scroll within the board area (AC-015 verifies header-card alignment during horizontal scroll; sticky also helps vertical).
- `data-testid="kanban-column-empty-state"` is shared across empty columns — tests target it *within* a specific column locator (AC-011).

### Testid contract

| testid | element |
|---|---|
| `kanban-column-${phase}` | column root |
| `kanban-column-header-${phase}` | header row (label + count) |
| `kanban-column-count-${phase}` | the count span |
| `kanban-column-empty-state` | empty-state message (scoped within column in tests) |

### Invariants

- `features.length` displayed in header === actual card count in body (SC-003).
- Cards are `<FeatureCard>` instances (CON-003).
- Empty state is a real element with non-empty text, not a hidden div (AC-011).

### Error / edge behavior

- No error paths. Receives already-ordered features. Renders them.

---

## Contract: `localStorage` persistence (Dashboard-owned)

**Key**: `devteam-dashboard-view`
**Values**: `"list"` | `"kanban"`
**Default (missing/invalid)**: `"list"` (FR-005, AC-004)

### Operations (all in Dashboard.tsx)

1. **Read on mount**: `localStorage.getItem('devteam-dashboard-view')`. If value is `"kanban"`, initial state = `"kanban"`; else `"list"`. Invalid values (e.g. `"foo"`) → `"list"` (defensive).
2. **Write on change**: whenever `view` state changes via the toggle, `localStorage.setItem('devteam-dashboard-view', view)`. Use a `useEffect` watching `view`.

### Test contract (AC-003)

- After clicking Kanban, `localStorage.getItem("devteam-dashboard-view")` === `"kanban"`.
- After `page.reload()`, board is visible (read path works).

### Edge behavior

- `localStorage` access in SSR / disabled storage: wrap read in `try/catch`, default to `"list"`. Not a concern for this app (Vite SPA, no SSR) but defensive code is cheap and prevents a crash if storage is full or blocked. Single try/catch, no abstraction.