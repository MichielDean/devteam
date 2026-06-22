import { defineConfig } from '@playwright/test';
import path from 'path';

const repoRoot = path.resolve(__dirname, '..');

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  expect: { timeout: 10000 },
  fullyParallel: false,
  retries: 1,
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:18765',
    trace: 'on-first-retry',
  },
  webServer: {
    // Use port 18765 for tests to avoid conflicts with the production
    // devteam-web service running on :8765.
    // SERVER_BINARY should be a full command including cd if needed, e.g.:
    //   SERVER_BINARY="cd /path/to/repo && /path/to/binary -http :18765"
    command: process.env.SERVER_BINARY || `cd ${repoRoot} && ~/go/bin/devteam -http :18765`,
    cwd: repoRoot,
    port: parseInt(process.env.SERVER_PORT || '18765'),
    // Reuse an existing server on the port if one is running.
    // Set START_SERVER=1 to force Playwright to start its own server.
    reuseExistingServer: !process.env.START_SERVER,
    timeout: 15000,
  },
});