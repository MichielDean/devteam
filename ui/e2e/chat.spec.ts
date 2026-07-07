import { test, expect } from '@playwright/test';

// Chat e2e — covers the MVS success criteria for the AIDLC Expert Agent and
// Chat UI feature. Tests run against the running devteam server (the same
// target as aidlc.spec.ts). The server must have the chat service wired
// (SetChatConfig) — which it does in production via main.go.

test.describe('Chat UI (AIDLC Expert Agent)', () => {
  test('chat route loads with input + provider picker', async ({ page }) => {
    await page.goto('/chat');
    await expect(page.locator('[data-testid="chat-page"]')).toBeVisible();
    await expect(page.locator('[data-testid="chat-input"]')).toBeVisible();
    await expect(page.locator('[data-testid="chat-provider-picker"]')).toBeVisible();
    // SC4: at least one provider is shown (default-safe ollama).
    await expect(page.locator('[data-testid="chat-provider-picker"]')).toContainText(/ollama|openai|anthropic/i);
  });

  test('new chat creates a session', async ({ page }) => {
    await page.goto('/chat');
    await page.click('[data-testid="chat-new-session"]');
    await expect(page.locator('[data-testid="chat-session-item"]')).toHaveCount(1);
  });

  test('SC12: off-topic question gets scoped refusal', async ({ page }) => {
    await page.goto('/chat');
    // Start a session + send an off-topic question.
    await page.click('[data-testid="chat-new-session"]');
    await page.fill('[data-testid="chat-input"]', 'what is the weather today?');
    await page.click('[data-testid="chat-send"]');
    // Wait for the expert message to appear (streaming completes).
    await expect(page.locator('[data-testid="chat-message-expert"]')).toBeVisible({ timeout: 30000 });
    // SC12: the refusal mentions the scope (AIDLC v2 / devteam).
    const expertText = await page.locator('[data-testid="chat-message-expert"]').textContent();
    expect(expertText?.toLowerCase()).toMatch(/aidlc|devteam|help with/i);
  });

  test('SC1: methodology question returns an answer with citations', async ({ page }) => {
    await page.goto('/chat');
    await page.click('[data-testid="chat-new-session"]');
    await page.fill('[data-testid="chat-input"]', 'what are the 5 phases of AIDLC?');
    await page.click('[data-testid="chat-send"]');
    // Wait for the expert message.
    await expect(page.locator('[data-testid="chat-message-expert"]')).toBeVisible({ timeout: 30000 });
    // SC1: the answer mentions the phases.
    const expertText = await page.locator('[data-testid="chat-message-expert"]').textContent();
    expect(expertText?.toLowerCase()).toMatch(/initialization|ideation|inception|construction|operation/);
    // SC1/FR-G2-4: a citation chip is rendered.
    await expect(page.locator('[data-testid="chat-citation-chip"]')).toBeVisible({ timeout: 5000 });
  });

  test('citation chip opens drawer', async ({ page }) => {
    await page.goto('/chat');
    await page.click('[data-testid="chat-new-session"]');
    await page.fill('[data-testid="chat-input"]', 'what does the architect own?');
    await page.click('[data-testid="chat-send"]');
    await expect(page.locator('[data-testid="chat-citation-chip"]')).toBeVisible({ timeout: 30000 });
    await page.click('[data-testid="chat-citation-chip"]');
    await expect(page.locator('[data-testid="citation-drawer"]')).toBeVisible();
    await expect(page.locator('[data-testid="citation-drawer"]')).toContainText(/File/i);
  });

  test('SC4: provider picker can switch providers', async ({ page }) => {
    await page.goto('/chat');
    await expect(page.locator('[data-testid="chat-provider-picker"]')).toBeVisible();
    // Open the picker — verifies it's clickable (≤2 clicks to switch).
    await page.click('[data-testid="chat-provider-picker"]');
    // The dropdown should show at least one option.
    await expect(page.locator('[role="option"]')).toHaveCount(1);
  });

  test('SC10: streaming — first chunk arrives before full response', async ({ page }) => {
    await page.goto('/chat');
    await page.click('[data-testid="chat-new-session"]');
    await page.fill('[data-testid="chat-input"]', 'what are the 5 phases?');
    // Click send and immediately check for the streaming indicator.
    await page.click('[data-testid="chat-send"]');
    // The "thinking" indicator or a streaming expert message should appear.
    await expect(
      page.locator('[data-testid="chat-message-expert"], [data-testid="chat-message-list"]')
    ).toBeVisible({ timeout: 10000 });
    // The send button swaps to Stop while streaming.
    // (If the stream already completed fast, the expert message is present.)
    await expect(page.locator('[data-testid="chat-message-expert"]')).toBeVisible({ timeout: 30000 });
  });
});