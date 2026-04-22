import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: 'tests',
  timeout: 30_000,
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:5173'
  },
  webServer: {
    command: 'npm run dev -- --port 5173',
    port: 5173,
    reuseExistingServer: !process.env.CI,
    env: { AGENTPOP_API_URL: process.env.AGENTPOP_API_URL ?? 'http://localhost:8091' }
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }]
});
