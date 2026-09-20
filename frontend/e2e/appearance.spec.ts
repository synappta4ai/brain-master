import { expect, test } from '@playwright/test';

/**
 * Estado inicial determinista: español, tema oscuro, paleta ámbar.
 * Se siembra solo en la primera carga del contexto (sessionStorage como
 * bandera) para poder verificar persistencia real tras `reload()`.
 */
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    if (sessionStorage.getItem('bm-seeded')) return;
    sessionStorage.setItem('bm-seeded', '1');
    localStorage.setItem('bm-lang', 'es');
    localStorage.setItem('bm-theme', 'dark');
    localStorage.setItem('bm-palette', 'amber');
  });
});

test.describe('Apariencia (tema, paleta e idioma)', () => {
  test('cambia tema oscuro ↔ claro desde el popover', async ({ page }) => {
    await page.goto('/studio');

    const html = page.locator('html');
    await expect(html).toHaveClass(/app-dark/);

    // Abrir el popover de apariencia (botón con icono de paleta).
    await page.locator('button:has(.pi-palette)').first().click();

    // Opciones del selectbutton (p-togglebutton) por su icono, independiente
    // del idioma activo.
    await page.locator('.p-popover p-togglebutton:has(.pi-sun)').click();
    await expect(html).toHaveClass(/app-light/);
    await expect(html).not.toHaveClass(/app-dark/);

    await page.locator('.p-popover p-togglebutton:has(.pi-moon)').click();
    await expect(html).toHaveClass(/app-dark/);
  });

  test('cambia la paleta y las variables CSS de PrimeNG en runtime', async ({
    page,
  }) => {
    await page.goto('/studio');

    await page.locator('button:has(.pi-palette)').first().click();

    // Swatch identificado por su aria-label (id de la paleta).
    await page.locator('.p-popover button[aria-label="emerald"]').click();

    await expect
      .poll(
        async () =>
          page.evaluate(() =>
            getComputedStyle(document.documentElement)
              .getPropertyValue('--p-primary-500')
              .trim(),
          ),
        { timeout: 5_000 },
      )
      .toBe('#10b981');

    // Persistido en localStorage.
    await expect
      .poll(() => page.evaluate(() => localStorage.getItem('bm-palette')))
      .toBe('emerald');
  });

  test('cambia el idioma y persiste tras recargar', async ({ page }) => {
    await page.goto('/studio');
    await expect(
      page.getByRole('button', { name: /Generar en GPU/i }),
    ).toBeVisible();

    await page.locator('button:has(.pi-palette)').first().click();
    await page
      .locator('.p-popover p-togglebutton', { hasText: 'EN' })
      .click();

    // Textos traducidos al instante.
    await expect(
      page.getByRole('button', { name: /Generate on GPU/i }),
    ).toBeVisible();

    // Persistencia real: tras recargar sigue en inglés (la siembra inicial
    // solo ocurre en la primera carga del contexto).
    await page.reload();
    await expect(
      page.getByRole('button', { name: /Generate on GPU/i }),
    ).toBeVisible();
    await expect
      .poll(() => page.evaluate(() => localStorage.getItem('bm-lang')))
      .toBe('en');
  });
});
