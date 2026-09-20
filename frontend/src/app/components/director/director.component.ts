import { Component, inject, signal } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';

export interface ShotPlan {
  id: number;
  timecode: string;
  durationSeconds: number;
  cameraMovement: string;
  actionPrompt: string;
  speaker?: string;
}

@Component({
  selector: 'app-director',
  standalone: true,
  imports: [ButtonModule, TranslatePipe],
  template: `
    <div class="flex flex-col gap-5">
      <!-- Barra de control superior -->
      <div class="glass-panel flex flex-wrap items-center justify-between gap-4 p-5">
        <div>
          <h2 class="m-0 text-xl font-bold text-primary">{{ 'DIRECTOR.TITLE' | translate }}</h2>
          <p class="m-0 mt-1 text-[13px] text-muted">{{ 'DIRECTOR.SUBTITLE' | translate }}</p>
        </div>
        <div class="flex gap-2.5">
          <p-button [label]="'DIRECTOR.ANALYZE' | translate" icon="pi pi-wave-pulse" severity="secondary" [outlined]="true" (onClick)="analyzeAudio()" />
          <p-button [label]="'DIRECTOR.WRITE' | translate" icon="pi pi-sparkles" (onClick)="generateScreenplay()" />
        </div>
      </div>

      <!-- Análisis BPM & energía -->
      @if (bpm()) {
        <div class="glass-panel flex flex-wrap items-center gap-6 p-4">
          <div class="bg-elevated-bm rounded-lg px-4 py-2.5">
            <span class="block text-[11px] uppercase text-subtle">{{ 'DIRECTOR.BPM' | translate }}</span>
            <strong class="text-xl text-primary">{{ bpm() }} BPM</strong>
          </div>
          <div class="bg-elevated-bm rounded-lg px-4 py-2.5">
            <span class="block text-[11px] uppercase text-subtle">{{ 'DIRECTOR.STRUCTURE' | translate }}</span>
            <strong class="text-base text-primary">{{ 'DIRECTOR.STRUCTURE_VALUE' | translate }}</strong>
          </div>
          <div class="min-w-40 flex-1">
            <span class="mb-1.5 block text-[11px] text-subtle">{{ 'DIRECTOR.CUTS' | translate }}</span>
            <div class="flex h-2 gap-0.5 overflow-hidden rounded">
              <div class="w-1/4 bg-primary-400"></div>
              <div class="w-1/3 bg-primary-500"></div>
              <div class="w-2/5 bg-primary-600"></div>
            </div>
          </div>
        </div>
      }

      <!-- Plan de tomas -->
      <div class="glass-panel p-5">
        <h3 class="m-0 mb-4 text-base font-bold">{{ 'DIRECTOR.SHOTS' | translate }}</h3>

        <div class="flex flex-col gap-3">
          @for (shot of shots(); track shot.id) {
            <div class="glass-card grid grid-cols-1 items-center gap-4 p-3.5 md:grid-cols-[80px_140px_1fr_140px]">
              <div class="font-mono-bm text-[13px] font-semibold text-primary">
                {{ 'DIRECTOR.SHOT_NUMBER' | translate: { id: shot.id } }}
              </div>
              <div>
                <span class="block text-[11px] text-subtle">{{ shot.timecode }}</span>
                <span class="text-[13px] font-medium text-primary">{{ 'DIRECTOR.SHOT_DURATION' | translate: { n: shot.durationSeconds } }}</span>
              </div>
              <div>
                <div class="mb-0.5 text-xs font-semibold text-primary">{{ shot.cameraMovement }}</div>
                <div class="text-[13px]">{{ shot.actionPrompt }}</div>
              </div>
              <div>
                <p-button [label]="'DIRECTOR.GENERATE_SHOT' | translate" icon="pi pi-play" size="small" severity="secondary" [outlined]="true" styleClass="w-full" />
              </div>
            </div>
          }
        </div>
      </div>
    </div>
  `
})
export class DirectorComponent {
  bpm = signal<number | null>(124);

  shots = signal<ShotPlan[]>([
    {
      id: 1,
      timecode: '00:00 - 00:04',
      durationSeconds: 4,
      cameraMovement: 'Slow Dolly Zoom In',
      actionPrompt: 'Close-up of the protagonist looking out over a neon metropolis in pouring rain, cinematic lighting.',
      speaker: 'Singer 1'
    },
    {
      id: 2,
      timecode: '00:04 - 00:09',
      durationSeconds: 5,
      cameraMovement: 'Drone Orbit High',
      actionPrompt: 'Wide aerial establishing shot of holographic skyscrapers pulsing to the synthwave rhythm.'
    },
    {
      id: 3,
      timecode: '00:09 - 00:15',
      durationSeconds: 6,
      cameraMovement: 'Tracking Lateral Shot',
      actionPrompt: 'Futuristic sports car speeding across the elevated wet highway with reflections.',
      speaker: 'Singer 1'
    }
  ]);

  analyzeAudio() {
    this.bpm.set(128);
  }

  generateScreenplay() {
    this.bpm.update(b => (b ?? 120) + 4);
  }
}
