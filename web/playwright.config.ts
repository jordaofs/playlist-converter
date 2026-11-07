import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  timeout: 45000,
  expect: { timeout: 15000 },
  reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:5173', trace: 'retain-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: 'go run ../cmd/testserver',
      url: 'http://127.0.0.1:8081/healthz',
      timeout: 120000,
      reuseExistingServer: false,
    },
    {
      command: 'pnpm dev --port 5173 --strictPort',
      url: 'http://127.0.0.1:5173',
      env: { API_PROXY_TARGET: 'http://127.0.0.1:8081' },
      timeout: 60000,
      reuseExistingServer: false,
    },
  ],
});
