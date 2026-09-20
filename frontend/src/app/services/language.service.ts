import { Injectable, inject, signal } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';

export type Lang = 'es' | 'en' | 'fr';

export interface LangOption {
  code: Lang;
  /** Nombre nativo del idioma (útil para menús futuros). */
  label: string;
}

const LANG_KEY = 'bm-lang';
const SUPPORTED: Lang[] = ['es', 'en', 'fr'];

@Injectable({ providedIn: 'root' })
export class LanguageService {
  private readonly translate = inject(TranslateService);

  readonly languages: LangOption[] = [
    { code: 'es', label: 'Español' },
    { code: 'en', label: 'English' },
    { code: 'fr', label: 'Français' }
  ];

  readonly current = signal<Lang>(this.initialLang());

  constructor() {
    this.translate.addLangs(SUPPORTED);
    this.translate.use(this.current());
  }

  use(lang: Lang): void {
    if (!SUPPORTED.includes(lang)) return;
    this.current.set(lang);
    this.translate.use(lang);
    try {
      localStorage.setItem(LANG_KEY, lang);
    } catch { /* ignorar */ }
  }

  private initialLang(): Lang {
    try {
      const saved = localStorage.getItem(LANG_KEY) as Lang | null;
      if (saved && SUPPORTED.includes(saved)) return saved;
    } catch { /* ignorar */ }
    const browser = this.translate.getBrowserLang();
    return browser === 'en' || browser === 'fr' ? browser : 'es';
  }
}
