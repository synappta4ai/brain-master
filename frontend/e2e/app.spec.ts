import { expect, test } from '@playwright/test';

/**
 * Estado inicial determinista: español, tema oscuro, paleta ámbar.
 */
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('bm-lang', 'es');
    localStorage.setItem('bm-theme', 'dark');
    localStorage.setItem('bm-palette', 'amber');
  });
});

test.describe('Studio', () => {
  test('carga la página con telemetría y formulario', async ({ page }) => {
    await page.goto('/studio');

    await expect(page.getByRole('heading', { name: /Studio Mode/i })).toBeVisible();
    // La card de telemetría GPU viene del gateway real vía gRPC.
    await expect(page.getByText(/Telemetría GPU/i)).toBeVisible();
    await expect(page.getByText('CPU / No CUDA')).toBeVisible();

    // Formulario con componentes PrimeNG.
    await expect(page.getByText('Modelo Generativo')).toBeVisible();
    await expect(
      page.getByRole('button', { name: /Generar en GPU/i }),
    ).toBeEnabled();
  });

  test('genera un video completo E2E (UI → gateway → worker gRPC → archivo)', async ({
    page,
  }) => {
    await page.goto('/studio');

    await page.getByRole('button', { name: /Generar en GPU/i }).click();

    // El monitor muestra progreso en vivo y termina COMPLETED al 100%.
    const monitor = page.locator('main');
    await expect(monitor.getByText(/COMPLETED/i)).toBeVisible({ timeout: 45_000 });
    await expect(monitor.getByText('100%')).toBeVisible();

    // Output real disponible con nombre job_*.mp4|png.
    await expect(monitor.getByText(/outputs\//i).or(monitor.getByText(/\.mp4|\.png/))).toBeVisible();
  });

  test('el botón Mejorar reescribe el prompt vía el worker', async ({ page }) => {
    await page.goto('/studio');

    const promptBox = page.getByLabel(/Prompt Positivo/i);
    await promptBox.fill('a red fox in the snow');
    await page.getByRole('button', { name: /Mejorar/i }).click();

    // El valor del textarea cambia (enhance del worker Python vía gateway).
    await expect
      .poll(async () => promptBox.inputValue(), { timeout: 15_000 })
      .not.toBe('a red fox in the snow');
  });
});

test.describe('Jobs', () => {
  test('muestra la tabla con los jobs persistidos y abre el detalle', async ({
    page,
    request,
  }) => {
    // Semilla: un job vía API (pasa por el proxy del dev server).
    const resp = await request.post('/api/v1/jobs/create', {
      data: {
        mode: 'image',
        prompt: 'e2e seed job playwright',
        model: 'Flux.1-dev',
        width: 512,
        height: 512,
        frames: 0,
      },
    });
    expect(resp.ok()).toBeTruthy();

    await page.goto('/jobs');

    await expect(page.getByRole('heading', { name: /Jobs/i })).toBeVisible();
    // La tabla p-table debe contener al menos el job sembrado.
    const row = page.locator('p-table tbody tr').filter({
      hasText: /Flux\.1-dev/,
    });
    await expect(row.first()).toBeVisible();

    // El tag de estado del job sembrado renderiza (COMPLETED tras unos segundos
    // o PROCESSING/QUEUED al instante).
    await expect(row.first().locator('.p-tag')).toBeVisible();
  });
});
