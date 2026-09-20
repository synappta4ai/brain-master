import { TestBed } from '@angular/core/testing';
import { TranslateService } from '@ngx-translate/core';

import { LanguageService } from './language.service';

describe('LanguageService', () => {
  let usedLangs: string[];
  let browserLang: string;
  let store: Record<string, string>;

  beforeEach(() => {
    // El entorno de ejecución no garantiza localStorage: usamos un fake.
    store = {};
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => (k in store ? store[k] : null),
      setItem: (k: string, v: string) => {
        store[k] = v;
      },
      clear: () => {
        store = {};
      },
    });
    usedLangs = [];
    browserLang = 'de'; // idioma no soportado por defecto

    const stub = {
      addLangs: () => undefined,
      use: (l: string) => {
        usedLangs.push(l);
      },
      getBrowserLang: () => browserLang,
    };

    TestBed.configureTestingModule({
      providers: [{ provide: TranslateService, useValue: stub }],
    });
  });

  afterEach(() => vi.unstubAllGlobals());

  function create(): LanguageService {
    return TestBed.inject(LanguageService);
  }

  it('usa es por defecto cuando el navegador pide un idioma no soportado', () => {
    expect(create().current()).toBe('es');
    expect(usedLangs).toContain('es');
  });

  it('usa el idioma del navegador cuando está soportado', () => {
    browserLang = 'fr';
    expect(create().current()).toBe('fr');
  });

  it('prioriza el idioma persistido sobre el del navegador', () => {
    store['bm-lang'] = 'en';
    expect(create().current()).toBe('en');
  });

  it('ignora un valor persistido inválido y cae a la detección', () => {
    store['bm-lang'] = 'zz';
    expect(create().current()).toBe('es');
  });

  it('use() cambia el idioma, llama a translate y persiste', () => {
    const svc = create();
    svc.use('fr');
    expect(svc.current()).toBe('fr');
    expect(usedLangs).toContain('fr');
    expect(store['bm-lang']).toBe('fr');
  });

  it('use() ignora idiomas no soportados', () => {
    const svc = create();
    svc.use('de' as never);
    expect(svc.current()).toBe('es');
    expect(store['bm-lang']).toBeUndefined();
  });

  it('expone las 3 opciones con nombre nativo', () => {
    const langs = create().languages;
    expect(langs.map((l) => l.code)).toEqual(['es', 'en', 'fr']);
    expect(langs.every((l) => l.label.length > 0)).toBe(true);
  });
});
