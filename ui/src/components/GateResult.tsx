import type { GateResult as GateResultType } from '../types';

interface GateResultProps {
  gateResult: GateResultType;
}

export default function GateResult({ gateResult }: GateResultProps) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6" data-testid="gate-result">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
          Gate: {gateResult.phase}
        </h3>
        <span
          className={`px-3 py-1 rounded-full text-sm font-medium ${
            gateResult.passed
              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
              : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
          }`}
          data-testid="gate-result-badge"
        >
          {gateResult.passed ? 'Passed' : 'Failed'}
        </span>
      </div>

      {gateResult.checks && gateResult.checks.length > 0 && (
        <div className="space-y-2" data-testid="gate-checks">
          {gateResult.checks.map((check, index) => (
            <div
              key={index}
              className="flex items-start gap-2 p-2 rounded-md bg-gray-50 dark:bg-gray-700"
              data-testid={`gate-check-${index}`}
            >
              <span className={`text-sm font-medium ${check.passed ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                {check.passed ? '✓' : '✗'}
              </span>
              <div className="flex-1">
                <p className="text-sm font-medium text-gray-900 dark:text-white">{check.name}</p>
                {check.message && (
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{check.message}</p>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}