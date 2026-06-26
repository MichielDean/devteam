import { test, expect } from '@playwright/test';

test('GET /api/health returns 200 with status ok and version 1.0', async ({ request }) => {
  const response = await request.get('/api/health');
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body.status).toBe('ok');
  expect(body.version).toBe('1.0');
});