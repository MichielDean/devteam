import { lazy, Suspense } from 'react';
import { useSearchParams } from 'react-router';

const ReposTab = lazy(() => import('./admin/ReposTab'));
const DefaultsTab = lazy(() => import('./admin/DefaultsTab'));
const ServerTab = lazy(() => import('./admin/ServerTab'));
const AuditTab = lazy(() => import('./admin/AuditTab'));

// Bolt-plan rev2 FR-SHELL-01 re-scope: 4 v1 tabs + 2 reserved fast-follow
// slots (Providers, CI/CD shown as disabled "coming soon" placeholders).
// The tab order is fixed (FR-SHELL-01): Repos, Defaults, Providers, CI/CD,
// Server, Audit.
type TabDef = {
  id: string;
  label: string;
  disabled?: boolean;
  comingSoon?: boolean;
};

const V1_TABS: TabDef[] = [
  { id: 'repos', label: 'Repos' },
  { id: 'defaults', label: 'Defaults' },
  { id: 'providers', label: 'Providers', disabled: true, comingSoon: true },
  { id: 'cicd', label: 'CI/CD', disabled: true, comingSoon: true },
  { id: 'server', label: 'Server' },
  { id: 'audit', label: 'Audit' },
];

const loadingStyle: React.CSSProperties = { color: 'var(--color-text-tertiary)' };

export default function AdminPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get('tab') ?? 'repos';

  const handleTabChange = (id: string) => {
    setSearchParams({ tab: id });
  };

  return (
    <div data-testid="admin-page">
      <h2 className="text-xl font-medium text-[var(--color-text-primary)] mb-4">Admin</h2>
      <div className="border-b border-[var(--color-border-subtle)] mb-4">
        <div className="flex gap-1">
          {V1_TABS.map((tab) => (
            <button
              key={tab.id}
              onClick={() => !tab.disabled && handleTabChange(tab.id)}
              disabled={tab.disabled}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.id
                  ? 'border-[var(--color-accent)] text-[var(--color-text-primary)]'
                  : 'border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
              } ${tab.disabled ? 'opacity-50 cursor-not-allowed hover:text-[var(--color-text-secondary)]' : ''}`}
              data-testid={`tab-${tab.id}`}
              aria-current={activeTab === tab.id ? 'page' : undefined}
            >
              {tab.label}
              {tab.comingSoon && (
                <span className="ml-1.5 text-xs text-[var(--color-text-tertiary)]" data-testid={`coming-soon-${tab.id}`}>
                  (soon)
                </span>
              )}
            </button>
          ))}
        </div>
      </div>
      <div className="mt-4">
        <Suspense fallback={<div className="text-center py-12" style={loadingStyle}>Loading...</div>}>
          {activeTab === 'repos' && <ReposTab />}
          {activeTab === 'defaults' && <DefaultsTab />}
          {activeTab === 'server' && <ServerTab />}
          {activeTab === 'audit' && <AuditTab />}
          {(activeTab === 'providers' || activeTab === 'cicd') && (
            <div className="text-center py-12 text-[var(--color-text-tertiary)]" data-testid="coming-soon-panel">
              This tab is a fast-follow integration and will be enabled once its sibling feature ships.
            </div>
          )}
        </Suspense>
      </div>
    </div>
  );
}