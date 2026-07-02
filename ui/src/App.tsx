import { Routes, Route, Link } from 'react-router';
import { lazy, Suspense } from 'react';
import Dashboard from './pages/Dashboard';
import FeatureDetail from './pages/FeatureDetail';
import ConnectionStatus from './components/ConnectionStatus';
import { ThemeToggle } from './components/ThemeToggle';

const TmuxPaneViewer = lazy(() => import('./components/TmuxPaneViewer'));
const KnowledgePage = lazy(() => import('./pages/KnowledgePage'));

export default function App() {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <header className="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700 sticky top-0 z-30">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-3 flex items-center justify-between">
          <Link to="/" className="text-xl font-bold text-gray-900 dark:text-white hover:text-blue-600 dark:hover:text-blue-400">
            Dev Team
          </Link>
          <nav className="flex items-center gap-4">
            <Link to="/knowledge" className="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white">Knowledge</Link>
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
            <Suspense fallback={<div className="text-center py-12 text-gray-500">Loading terminal...</div>}>
              <TmuxPaneViewer />
            </Suspense>
          } />
          <Route path="/knowledge" element={
            <Suspense fallback={<div className="text-center py-12 text-gray-500">Loading...</div>}>
              <KnowledgePage />
            </Suspense>
          } />
        </Routes>
      </main>
    </div>
  );
}