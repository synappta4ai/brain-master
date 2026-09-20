import { defineConfig } from '@playwright/test';

/**
 * E2E contra el stack real: front Angular (4200) → gateway Go (8080) →
 * worker Python gRPC (50051). Si el dev server no está corriendo,
 * Playwright lo levanta con `npm start`.
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  reporter: [['list']],
  use: {
    baseURL: 'http://localhost:4200',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'npm start',
    url: 'http://localhost:4200',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
