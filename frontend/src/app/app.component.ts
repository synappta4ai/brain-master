import { Component, inject } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive, NavigationEnd, Router } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { filter } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { TooltipModule } from 'primeng/tooltip';
import { SelectButtonModule } from 'primeng/selectbutton';
import { PopoverModule } from 'primeng/popover';
import { TagModule } from 'primeng/tag';
import { WebSocketService } from './services/websocket.service';
import { ThemeService } from './services/theme.service';
import { LanguageService } from './services/language.service';

interface NavItem {
  route: string;
  key: string;
  icon: string;
}

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    FormsModule,
    TranslatePipe,
    ButtonModule,
    TooltipModule,
    SelectButtonModule,
    PopoverModule,
    TagModule,
  ],
  templateUrl: './app.component.html',
})
export class AppComponent {
  readonly themeService = inject(ThemeService);
  readonly languageService = inject(LanguageService);

  private readonly wsService = inject(WebSocketService);
  private readonly translate = inject(TranslateService);
  private readonly router = inject(Router);

  /** Conexión WebSocket + estado del último job para el header. */
  readonly ws = this.wsService;

  readonly navItems: NavItem[] = [
    { route: '/studio', key: 'NAV.STUDIO', icon: 'pi pi-sparkles' },
    { route: '/director', key: 'NAV.DIRECTOR', icon: 'pi pi-video' },
    { route: '/editor', key: 'NAV.EDITOR', icon: 'pi pi-sliders-h' },
    { route: '/loras', key: 'NAV.LORAS', icon: 'pi pi-box' },
    { route: '/jobs', key: 'NAV.JOBS', icon: 'pi pi-list-check' },
  ];

  readonly themeModeOptions = [
    { label: 'SETTINGS.DARK', value: 'dark', icon: 'pi pi-moon' },
    { label: 'SETTINGS.LIGHT', value: 'light', icon: 'pi pi-sun' },
  ];

  constructor() {
    this.wsService.connect();

    // Re-translate al cambiar el idioma: el título de la página sigue al idioma.
    this.router.events
      .pipe(
        filter((e) => e instanceof NavigationEnd),
        takeUntilDestroyed(),
      )
      .subscribe(() => window.scrollTo({ top: 0 }));
  }

  get statusSeverity(): 'success' | 'danger' {
    return this.wsService.isConnected() ? 'success' : 'danger';
  }
}
