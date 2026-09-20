import { Injectable, signal, effect, inject } from '@angular/core';
import { PrimeNG } from 'primeng/config';
import { definePreset } from '@primeuix/themes';
import Aura from '@primeuix/themes/aura';

export type ThemeMode = 'dark' | 'light';
export type PaletteId = 'amber' | 'emerald' | 'blue' | 'violet' | 'rose' | 'teal';

const THEME_KEY = 'bm-theme';
const PALETTE_KEY = 'bm-palette';

interface PaletteDef {
  id: PaletteId;
  /** Nombre del token de color en el sistema PrimeNG ({amber.500}, {emerald.500}...) */
  token: string;
  /** Muestra para el swatch del popover */
  swatch: string;
}

const PALETTES: PaletteDef[] = [
  { id: 'amber', token: 'amber', swatch: '#f59e0b' },
  { id: 'emerald', token: 'emerald', swatch: '#10b981' },
  { id: 'blue', token: 'blue', swatch: '#3b82f6' },
  { id: 'violet', token: 'violet', swatch: '#8b5cf6' },
  { id: 'rose', token: 'rose', swatch: '#f43f5e' },
  { id: 'teal', token: 'teal', swatch: '#14b8a6' }
];

/** Presets precomputados: definePreset es costoso, se hace una sola vez. */
const PRESETS = new Map<PaletteId, ReturnType<typeof definePreset>>(
  PALETTES.map(p => [p.id, definePreset(Aura, {
    semantic: {
      primary: {
        50: `{${p.token}.50}`,
        100: `{${p.token}.100}`,
        200: `{${p.token}.200}`,
        300: `{${p.token}.300}`,
        400: `{${p.token}.400}`,
        500: `{${p.token}.500}`,
        600: `{${p.token}.600}`,
        700: `{${p.token}.700}`,
        800: `{${p.token}.800}`,
        900: `{${p.token}.900}`,
        950: `{${p.token}.950}`
      }
    }
  })])
);

@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly primeng = inject(PrimeNG);

  /** Modo de color activo (afecta a PrimeNG y a Tailwind a la vez). */
  readonly theme = signal<ThemeMode>(this.initialTheme());

  /** Paleta primaria activa. */
  readonly palette = signal<PaletteId>(this.initialPalette());

  readonly palettes = PALETTES.map(({ id, swatch }) => ({ id, swatch }));

  constructor() {
    // Aplica la clase en <html>, persiste y sincroniza PrimeNG.
    effect(() => {
      const mode = this.theme();
      const el = document.documentElement;
      el.classList.remove('app-dark', 'app-light');
      el.classList.add(mode === 'dark' ? 'app-dark' : 'app-light');
      try {
        localStorage.setItem(THEME_KEY, mode);
      } catch { /* almacenamiento no disponible: ignorar */ }
    });

    // Cambia el preset de PrimeNG en runtime y persiste la paleta.
    effect(() => {
      const paletteId = this.palette();
      this.primeng.setThemeConfig({
        theme: { preset: PRESETS.get(paletteId), options: THEME_OPTIONS }
      });
      try {
        localStorage.setItem(PALETTE_KEY, paletteId);
      } catch { /* ignorar */ }
    });
  }

  toggle(): void {
    this.theme.update(m => (m === 'dark' ? 'light' : 'dark'));
  }

  setMode(mode: ThemeMode): void {
    this.theme.set(mode);
  }

  setPalette(id: PaletteId): void {
    this.palette.set(id);
  }

  get isDark(): boolean {
    return this.theme() === 'dark';
  }

  private initialTheme(): ThemeMode {
    try {
      const saved = localStorage.getItem(THEME_KEY);
      if (saved === 'dark' || saved === 'light') return saved;
    } catch { /* ignorar */ }
    return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: light)').matches
      ? 'light'
      : 'dark';
  }

  private initialPalette(): PaletteId {
    try {
      const saved = localStorage.getItem(PALETTE_KEY) as PaletteId | null;
      if (saved && PALETTES.some(p => p.id === saved)) return saved;
    } catch { /* ignorar */ }
    return 'amber';
  }
}

/** Opciones compartidas: deben coincidir con app.config.ts. */
export const THEME_OPTIONS = {
  darkModeSelector: '.app-dark',
  cssLayer: {
    name: 'primeng',
    order: 'theme, base, primeng, components, utilities'
  }
};
