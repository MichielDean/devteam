// AuditTab — read-only filtered + paginated audit event list (Bolt 4).
// Backed by GET /api/audit (FR-AUDIT-01..03). Filters by event type and
// actor; pagination via page/page_size. No write affordance (FR-AUDIT-03).

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { listAuditEvents } from '../../api/client';
import { Card } from '../../ui/primitives';

const selectClass =
  'px-3 py-1.5 rounded-[var(--radius-md)] bg-[var(--color-surface)] text-[var(--color-text-primary)] border border-[var(--color-border-subtle)] focus:border-[var(--color-accent)] focus:outline-none text-sm';

const EVENT_TYPE_OPTIONS = [
  '',
  'CONFIG_UPDATED',
  'CONFIG_VALIDATION_FAILED',
  'REPOS_REGISTRY_MUTATED',
  'FEATURE_DEFAULTS_MUTATED',
  'WORKFLOW_START',
  'STAGE_START',
  'STAGE_COMPLETED',
  'GATE_APPROVED',
  'BOLT_STARTED',
  'BOLT_COMPLETED',
];

export default function AuditTab() {
  const [typeFilter, setTypeFilter] = useState('');
  const [actorFilter, setActorFilter] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 25;

  const { data, isLoading } = useQuery({
    queryKey: ['audit', typeFilter, actorFilter, page],
    queryFn: () => listAuditEvents({ type: typeFilter || undefined, actor: actorFilter || undefined, page, page_size: pageSize }),
  });

  const events = data?.events ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div data-testid="audit-tab">
      <div className="flex flex-wrap gap-2 mb-4" data-testid="audit-filters">
        <select value={typeFilter} onChange={(e) => { setTypeFilter(e.target.value); setPage(1); }} className={selectClass} data-testid="audit-filter-type">
          {EVENT_TYPE_OPTIONS.map((o) => <option key={o} value={o}>{o || 'All types'}</option>)}
        </select>
        <input
          type="text"
          placeholder="Actor filter"
          value={actorFilter}
          onChange={(e) => { setActorFilter(e.target.value); setPage(1); }}
          className={selectClass}
          data-testid="audit-filter-actor"
        />
      </div>

      {isLoading ? (
        <p className="text-sm text-[var(--color-text-tertiary)]">Loading...</p>
      ) : events.length === 0 ? (
        <Card className="p-6 text-center">
          <p className="text-sm text-[var(--color-text-tertiary)]" data-testid="audit-empty">No audit events match the current filters.</p>
        </Card>
      ) : (
        <div data-testid="audit-list">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--color-border-subtle)]">
                <th className="text-left py-2 text-xs font-medium text-[var(--color-text-secondary)]">Time</th>
                <th className="text-left py-2 text-xs font-medium text-[var(--color-text-secondary)]">Type</th>
                <th className="text-left py-2 text-xs font-medium text-[var(--color-text-secondary)]">Feature</th>
                <th className="text-left py-2 text-xs font-medium text-[var(--color-text-secondary)]">Actor</th>
                <th className="text-left py-2 text-xs font-medium text-[var(--color-text-secondary)]">Details</th>
              </tr>
            </thead>
            <tbody>
              {events.map((e) => (
                <tr key={e.id} className="border-b border-[var(--color-border-subtle)]" data-testid={`audit-row-${e.id}`}>
                  <td className="py-2 text-xs text-[var(--color-text-tertiary)]">{new Date(e.created_at).toLocaleString()}</td>
                  <td className="py-2 text-xs font-mono text-[var(--color-text-primary)]">{e.event_type}</td>
                  <td className="py-2 text-xs text-[var(--color-text-secondary)]">{e.feature_id}</td>
                  <td className="py-2 text-xs text-[var(--color-text-secondary)]">{e.actor || '—'}</td>
                  <td className="py-2 text-xs text-[var(--color-text-secondary)]">{e.details || ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="flex items-center justify-between mt-4" data-testid="audit-pagination">
            <span className="text-xs text-[var(--color-text-tertiary)]">{total} total</span>
            <div className="flex gap-1">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
                className="px-3 py-1 text-sm rounded-[var(--radius-md)] border border-[var(--color-border-subtle)] text-[var(--color-text-secondary)] disabled:opacity-50"
                data-testid="audit-prev-page"
              >Prev</button>
              <span className="px-3 py-1 text-sm text-[var(--color-text-secondary)]">{page} / {totalPages}</span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
                className="px-3 py-1 text-sm rounded-[var(--radius-md)] border border-[var(--color-border-subtle)] text-[var(--color-text-secondary)] disabled:opacity-50"
                data-testid="audit-next-page"
              >Next</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}