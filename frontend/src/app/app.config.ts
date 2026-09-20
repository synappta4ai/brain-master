import { ApplicationConfig, inject, provideZoneChangeDetection } from '@angular/core';
import { provideHttpClient } from '@angular/common/http';
import { provideRouter, withInMemoryScrolling } from '@angular/router';
import { appRoutes } from './app.routes';
import { providePrimeNG } from 'primeng/config';
import { definePreset } from '@primeuix/themes';
import Aura from '@primeuix/themes/aura';
import { provideTranslateService, provideTranslateLoader } from '@ngx-translate/core';
import { TranslateHttpLoader, provideTranslateHttpLoader } from '@ngx-translate/http-loader';
import { THEME_OPTIONS } from './services/theme.service';

// Preset "Golden Hour" (ámbar) — la paleta cambia en runtime vía ThemeService.
const GoldenHour = definePreset(Aura, {
  semantic: {
    primary: {
      50: '{amber.50}',
      100: '{amber.100}',
      200: '{amber.200}',
      300: '{amber.300}',
      400: '{amber.400}',
      500: '{amber.500}',
      600: '{amber.600}',
      700: '{amber.700}',
      800: '{amber.800}',
      900: '{amber.900}',
      950: '{amber.950}'
    }
  }
});

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(appRoutes, withInMemoryScrolling({ scrollPositionRestoration: 'top' })),
    provideHttpClient(),
    // PrimeNG 21 usa animaciones CSS (@primeuix/motion): no requiere
    // provideAnimationsAsync ni @angular/animations.
    providePrimeNG({
      theme: {
        preset: GoldenHour,
        options: THEME_OPTIONS
      },
      ripple: true
    }),
    // i18n: JSON en public/assets/i18n/{lang}.json
    // El loader se pasa explícito al servicio: si no, provideTranslateService
    // registra TranslateNoOpLoader y pisa el HttpLoader.
    provideTranslateHttpLoader({ prefix: '/assets/i18n/' }),
    provideTranslateService({
      loader: provideTranslateLoader(TranslateHttpLoader),
      fallbackLang: 'en',
      lang: 'es'
    }),
  ]
};
