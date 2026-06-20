import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  expect: { timeout: 10000 },
  fullyParallel: false,
  retries: 1,
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8765',
    trace: 'on-first-retry',
  },
  webServer: {
    command: process.env.SERVER_BINARY || '~/go/bin/devteam -http :8766',
    port: parseInt(process.env.SERVER_PORT || '8766'),
    reuseExistingServer: !process.env.START_SERVER,
    timeout: 10000,
  },
});