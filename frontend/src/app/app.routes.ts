import { Routes } from '@angular/router';

/**
 * Rutas de Brain-Master v2.1.
 * Cada página se carga de forma diferida (loadComponent) para mantener
 * el bundle inicial pequeño (crítico para el budget de Angular).
 */
export const appRoutes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'studio' },
  {
    path: 'studio',
    loadComponent: () => import('./pages/studio/studio.component').then(m => m.StudioComponent),
    title: 'Brain-Master · Studio',
  },
  {
    path: 'director',
    loadComponent: () => import('./components/director/director.component').then(m => m.DirectorComponent),
    title: 'Brain-Master · Director Mode',
  },
  {
    path: 'editor',
    loadComponent: () => import('./components/timeline-editor/timeline-editor.component').then(m => m.TimelineEditorComponent),
    title: 'Brain-Master · Timeline Editor',
  },
  {
    path: 'loras',
    loadComponent: () => import('./components/lora-browser/lora-browser.component').then(m => m.LoraBrowserComponent),
    title: 'Brain-Master · LoRAs',
  },
  {
    path: 'jobs',
    loadComponent: () => import('./pages/jobs/jobs.component').then(m => m.JobsComponent),
    title: 'Brain-Master · Jobs',
  },
  { path: '**', redirectTo: 'studio' },
];
