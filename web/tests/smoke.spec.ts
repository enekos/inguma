import { test, expect } from '@playwright/test';

// These tests assume bin/apid is running on :8091 against internal/api/testdata/corpus.
// Start it before `npm run test:e2e`.

test('home renders fixture tools', async ({ page }) => {
	await page.goto('/');
	await expect(page.locator('h1')).toContainText('Find agentic tools');
	await expect(page.getByText('Tool A').first()).toBeVisible();
});

test('tool detail shows install tabs with both adapters', async ({ page }) => {
	await page.goto('/t/tool-a');
	await expect(page.locator('h1')).toContainText('Tool A');
	await expect(page.getByRole('button', { name: 'CLI' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Claude Code' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Cursor' })).toBeVisible();
});

test('category page lists scoped tools', async ({ page }) => {
	await page.goto('/categories/search');
	await expect(page.locator('h1')).toContainText('search');
	await expect(page.getByText('Tool A')).toBeVisible();
});
