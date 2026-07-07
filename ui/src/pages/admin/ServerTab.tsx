// ServerTab — v1 minimal (Bolt 3 is post-MVP).
// Shows the restart-classification table as a read-only reference. The
// writable server config store (server_config table, /api/settings/server
// handlers) ships in Bolt 3. This tab validates the shell navigation and
// the classification UX pattern without blocking the MVP.

import { Card } from '../../ui/primitives';

export default function ServerTab() {
  const classifications: { field: string; classification: string }[] = [
    { field: 'database.dsn', classification: 'Bootstrap, write-only' },
    { field: 'pipeline.human_interaction_timeout_minutes', classification: 'Restart required' },
    { field: 'pipeline.execution_mode', classification: 'Restart required (default)' },
    { field: 'spec_repo.path', classification: 'Bootstrap, read-only' },
    { field: 'spec_repo.specs_dir', classification: 'Immediate' },
    { field: 'spec_repo.constitution_dir', classification: 'Immediate' },
    { field: 'intake.loose_idea.*', classification: 'Immediate' },
    { field: 'intake.external_spec.*', classification: 'Immediate' },
    { field: 'extensions.*', classification: 'Restart required' },
    { field: 'plugins.*', classification: 'Restart required' },
    { field: 'roles.* (YAML map)', classification: 'Not exposed (descriptive metadata)' },
  ];

  const badgeColor = (cls: string): string => {
    if (cls.startsWith('Restart')) return 'var(--color-warning, #f59e0b)';
    if (cls.startsWith('Immediate')) return 'var(--color-success)';
    if (cls.startsWith('Bootstrap')) return 'var(--color-text-tertiary)';
    return 'var(--color-text-tertiary)';
  };

  return (
    <div data-testid="server-tab">
      <Card className="p-4">
        <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">Server Config — Restart Classification</h3>
        <p className="text-xs text-[var(--color-text-tertiary)] mb-3">
          Editing server fields via the admin UI ships in a follow-up bolt. This view shows how each field takes effect.
        </p>
        <table className="w-full text-sm" data-testid="server-classification-table">
          <thead>
            <tr className="border-b border-[var(--color-border-subtle)]">
              <th className="text-left py-2 text-xs font-medium text-[var(--color-text-secondary)]">Field</th>
              <th className="text-left py-2 text-xs font-medium text-[var(--color-text-secondary)]">Classification</th>
            </tr>
          </thead>
          <tbody>
            {classifications.map((row) => (
              <tr key={row.field} className="border-b border-[var(--color-border-subtle)]">
                <td className="py-2 text-[var(--color-text-primary)] font-mono text-xs">{row.field}</td>
                <td className="py-2">
                  <span
                    className="px-2 py-0.5 rounded text-xs font-medium"
                    style={{ backgroundColor: badgeColor(row.classification), color: 'white' }}
                    data-testid={`server-badge-${row.field}`}
                  >
                    {row.classification}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}