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
    // Binary loads devteam.yaml from its working directory; repo root holds the
    // config, so cd there before launching. Port matches baseURL below.
    // SERVER_PORT drives both the -http flag and the port Playwright waits on,
    // so run-tests.sh can isolate the test server from the production :8765 one.
    command:
      process.env.SERVER_BINARY ||
      `cd .. && ~/go/bin/devteam -http :${process.env.SERVER_PORT || '8765'}`,
    port: parseInt(process.env.SERVER_PORT || '8765'),
    // Reuse the systemd-managed devteam-web.service when present (production-ish
    // local install). Set START_SERVER=1 to force a fresh process.
    reuseExistingServer: !process.env.START_SERVER,
    timeout: 15000,
  },
});