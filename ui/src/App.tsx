import { Routes, Route, Link } from 'react-router';
import { lazy, Suspense } from 'react';
import Dashboard from './pages/Dashboard';
import FeatureDetail from './pages/FeatureDetail';
import ConnectionStatus from './components/ConnectionStatus';
import { ThemeToggle } from './components/ThemeToggle';

const TmuxPaneViewer = lazy(() => import('./components/TmuxPaneViewer'));
const KnowledgePage = lazy(() => import('./pages/KnowledgePage'));
const AdminPage = lazy(() => import('./pages/AdminPage'));

const loadingStyle: React.CSSProperties = { color: 'var(--color-text-tertiary)' };

export default function App() {
  return (
    <div className="min-h-screen">
      <header
        className="sticky top-0 z-30 border-b border-[var(--color-border-subtle)]"
        style={{ backgroundColor: 'var(--color-surface-raised)', boxShadow: 'var(--shadow-sm)' }}
      >
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3 flex items-center justify-between">
          <Link to="/" className="text-lg font-semibold text-[var(--color-text-primary)] hover:text-[var(--color-accent)] transition-colors">
            Dev Team
          </Link>
          <nav className="flex items-center gap-1">
            <Link
              to="/knowledge"
              className="px-3 py-1.5 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-hover)] rounded-[var(--radius-md)] transition-colors"
            >
              Knowledge
            </Link>
            <Link
              to="/admin"
              className="px-3 py-1.5 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-hover)] rounded-[var(--radius-md)] transition-colors"
            >
              Admin
            </Link>
            <ThemeToggle />
          </nav>
        </div>
      </header>
      <ConnectionStatus />
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/features/:id" element={<FeatureDetail />} />
          <Route path="/features/:id/stages/:stageId" element={<FeatureDetail />} />
          <Route path="/features/:id/bolts" element={<FeatureDetail />} />
          <Route path="/features/:id/audit" element={<FeatureDetail />} />
          <Route path="/features/:id/sessions" element={<FeatureDetail />} />
          <Route path="/features/:id/sessions/:phase/pane" element={
            <Suspense fallback={<div className="text-center py-12" style={loadingStyle}>Loading terminal...</div>}>
              <TmuxPaneViewer />
            </Suspense>
          } />
          <Route path="/knowledge" element={
            <Suspense fallback={<div className="text-center py-12" style={loadingStyle}>Loading...</div>}>
              <KnowledgePage />
            </Suspense>
          } />
          <Route path="/admin" element={
            <Suspense fallback={<div className="text-center py-12" style={loadingStyle}>Loading...</div>}>
              <AdminPage />
            </Suspense>
          } />
        </Routes>
      </main>
    </div>
  );
}