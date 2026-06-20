import { Routes, Route } from 'react-router';
import Dashboard from './pages/Dashboard';
import FeatureDetail from './pages/FeatureDetail';
import ConnectionStatus from './components/ConnectionStatus';
import { ThemeToggle } from './components/ThemeToggle';

export default function App() {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <header className="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex items-center justify-between">
          <h1 className="text-xl font-bold text-gray-900 dark:text-white">
            Dev Team
          </h1>
          <ThemeToggle />
        </div>
      </header>
      <ConnectionStatus />
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/features/:id" element={<FeatureDetail />} />
        </Routes>
      </main>
    </div>
  );
}