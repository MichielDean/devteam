import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getDefaults, putGlobalDefaults, deleteRepoDefaults } from '../../api/client';
import type { DefaultsRow } from '../../types/admin';
import { Button, Badge, Card } from '../../ui/primitives';
import { useToast } from '../../components/Toast';

const selectClass =
  'w-full px-3 py-2 rounded-[var(--radius-md)] bg-[var(--color-surface)] text-[var(--color-text-primary)] border border-[var(--color-border-subtle)] focus:border-[var(--color-accent)] focus:outline-none text-sm';
const labelClass = 'block text-xs font-medium text-[var(--color-text-secondary)] mb-1';

const SCOPE_OPTIONS = ['', 'greenfield', 'feature', 'enterprise', 'mvp', 'infra', 'security-patch'];
const DEPTH_OPTIONS = ['', 'minimal', 'standard', 'comprehensive'];
const TEST_STRATEGY_OPTIONS = ['', 'unit', 'integration', 'e2e', 'none'];
const EXEC_MODE_OPTIONS = ['', 'human', 'autonomous', 'guided'];

export default function DefaultsTab() {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [form, setForm] = useState<DefaultsRow>({ scope: '', depth: '', test_strategy: '', execution_mode: '' });
  const [loaded, setLoaded] = useState(false);

  const { data } = useQuery({
    queryKey: ['defaults'],
    queryFn: getDefaults,
  });

  // Load global values into the form once when data arrives.
  if (data && !loaded) {
    setForm({
      scope: data.global.scope ?? '',
      depth: data.global.depth ?? '',
      test_strategy: data.global.test_strategy ?? '',
      execution_mode: data.global.execution_mode ?? '',
    });
    setLoaded(true);
  }

  const saveGlobalMutation = useMutation({
    mutationFn: putGlobalDefaults,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['defaults'] });
      addToast('success', 'Global defaults saved');
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const deleteRepoMutation = useMutation({
    mutationFn: deleteRepoDefaults,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['defaults'] });
      addToast('success', 'Per-repo override removed');
    },
    onError: (err: Error) => addToast('error', err.message),
  });

  const submitGlobal = () => {
    saveGlobalMutation.mutate(form);
  };

  return (
    <div data-testid="defaults-tab" className="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <Card className="p-4">
        <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-3">Global Defaults</h3>
        <p className="text-xs text-[var(--color-text-tertiary)] mb-3">
          Applied when no explicit value is supplied and no per-repo override exists. Precedence: explicit &gt; per-repo &gt; global &gt; scope-derived.
        </p>
        <div className="space-y-3">
          <div>
            <label className={labelClass}>Scope</label>
            <select value={form.scope} onChange={(e) => setForm({ ...form, scope: e.target.value })} className={selectClass} data-testid="defaults-form-scope">
              {SCOPE_OPTIONS.map((o) => <option key={o} value={o}>{o || '— (scope-derived)'}</option>)}
            </select>
          </div>
          <div>
            <label className={labelClass}>Depth</label>
            <select value={form.depth} onChange={(e) => setForm({ ...form, depth: e.target.value })} className={selectClass} data-testid="defaults-form-depth">
              {DEPTH_OPTIONS.map((o) => <option key={o} value={o}>{o || '— (scope-derived)'}</option>)}
            </select>
          </div>
          <div>
            <label className={labelClass}>Test Strategy</label>
            <select value={form.test_strategy} onChange={(e) => setForm({ ...form, test_strategy: e.target.value })} className={selectClass} data-testid="defaults-form-test-strategy">
              {TEST_STRATEGY_OPTIONS.map((o) => <option key={o} value={o}>{o || '— (scope-derived)'}</option>)}
            </select>
          </div>
          <div>
            <label className={labelClass}>Execution Mode</label>
            <select value={form.execution_mode} onChange={(e) => setForm({ ...form, execution_mode: e.target.value })} className={selectClass} data-testid="defaults-form-exec-mode">
              {EXEC_MODE_OPTIONS.map((o) => <option key={o} value={o}>{o || '— (default: human)'}</option>)}
            </select>
          </div>
          <Button variant="primary" size="sm" onClick={submitGlobal} disabled={saveGlobalMutation.isPending} data-testid="defaults-form-save">Save Global</Button>
        </div>
      </Card>

      <Card className="p-4">
        <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-3">Per-Repo Overrides ({data?.per_repo?.length ?? 0})</h3>
        {(data?.per_repo ?? []).length === 0 ? (
          <p className="text-sm text-[var(--color-text-tertiary)]" data-testid="defaults-per-repo-empty">No per-repo overrides set.</p>
        ) : (
          <div className="space-y-2" data-testid="defaults-per-repo-list">
            {(data?.per_repo ?? []).map((row) => (
              <div key={row.repo} className="p-3 rounded-[var(--radius-md)]" style={{ backgroundColor: 'var(--color-surface-hover)' }}>
                <div className="flex items-center justify-between mb-1">
                  <Badge color="blue">{row.repo}</Badge>
                  <Button variant="danger" size="sm" onClick={() => row.repo && deleteRepoMutation.mutate(row.repo)} data-testid={`defaults-per-repo-delete-${row.repo}`}>Remove</Button>
                </div>
                <div className="text-xs text-[var(--color-text-secondary)]">
                  {row.scope && <span className="mr-2">scope={row.scope}</span>}
                  {row.depth && <span className="mr-2">depth={row.depth}</span>}
                  {row.test_strategy && <span className="mr-2">test={row.test_strategy}</span>}
                  {row.execution_mode && <span>mode={row.execution_mode}</span>}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}