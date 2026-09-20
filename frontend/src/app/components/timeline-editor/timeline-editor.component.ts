import { Component, computed, signal } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';

export interface TimelineClip {
  id: string;
  title: string;
  start: number; // segundos
  duration: number; // segundos
  color: string;
}

export interface TimelineTrack {
  id: string;
  name: string;
  type: 'video' | 'audio' | 'overlay';
  clips: TimelineClip[];
}

@Component({
  selector: 'app-timeline-editor',
  standalone: true,
  imports: [ButtonModule, TranslatePipe],
  template: `
    <div class="flex flex-col gap-4">
      <!-- Cabecera del editor -->
      <div class="glass-panel flex flex-wrap items-center justify-between gap-4 p-4">
        <div class="flex items-center gap-4">
          <h2 class="m-0 text-lg font-bold text-primary">{{ 'EDITOR.TITLE' | translate }}</h2>
          <span class="font-mono-bm bg-main-bm rounded px-2.5 py-1 text-[13px] text-primary">
            TC: {{ timecode() }} / 00:00:30:00
          </span>
        </div>
        <div class="flex gap-2">
          <p-button
            [label]="(isPlaying() ? 'EDITOR.PAUSE' : 'EDITOR.PLAY') | translate"
            [icon]="isPlaying() ? 'pi pi-pause' : 'pi pi-play'"
            (onClick)="togglePlay()" />
          <p-button [label]="'EDITOR.SPLIT' | translate" icon="pi pi-sliders-h" severity="secondary" [outlined]="true" />
          <p-button [label]="'EDITOR.EXPORT' | translate" icon="pi pi-upload" severity="secondary" [outlined]="true" />
        </div>
      </div>

      <!-- Monitor de preview y media pool -->
      <div class="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <div class="glass-panel flex min-h-70 flex-col items-center justify-center p-4">
          <div class="border-subtle bg-elevated-bm relative flex aspect-video w-full max-w-md items-center justify-center overflow-hidden rounded-lg border">
            <span class="text-muted text-[13px]">🎥 {{ 'EDITOR.PREVIEW' | translate }}</span>
            <div class="font-mono-bm text-primary absolute bottom-2 left-3 text-[11px]">
              Track 1: Active
            </div>
            <!-- Playhead sobre el preview -->
            <div class="bg-primary absolute top-0 h-full w-0.5" [style.left.%]="playheadPct()"></div>
          </div>
        </div>

        <div class="glass-panel p-4">
          <h4 class="m-0 mb-3 text-sm font-bold">📁 {{ 'EDITOR.MEDIA_POOL' | translate }}</h4>
          <div class="flex max-h-60 flex-col gap-2 overflow-y-auto">
            @for (clip of mediaPool; track clip.title) {
              <div class="glass-card flex justify-between px-3 py-2 text-xs">
                <span>{{ clip.title }}</span>
                <span class="text-muted">{{ clip.duration.toFixed(1) }}s</span>
              </div>
            }
          </div>
        </div>
      </div>

      <!-- Timeline multipista -->
      <div class="glass-panel overflow-x-auto p-4">
        <div class="flex min-w-[700px] flex-col gap-2.5">
          <!-- Regla de tiempo -->
          <div class="border-subtle font-mono-bm text-subtle ml-30 flex h-6 border-b text-[11px]">
            @for (mark of rulerMarks(); track mark) {
              <div class="w-1/5">{{ mark }}</div>
            }
          </div>

          <!-- Playhead sobre las pistas -->
          <div class="relative">
            <div class="bg-primary pointer-events-none absolute top-0 z-10 h-[calc(100%+4px)] w-0.5"
                 [style.left.%]="playheadPct()"></div>

            @for (track of tracks(); track track.id) {
              <div class="mb-2.5 flex items-center gap-3">
                <div class="bg-elevated-bm w-27 rounded px-2 py-2 text-xs font-semibold text-muted">
                  {{ track.name }}
                </div>
                <div class="border-subtle bg-main-bm relative flex h-11 flex-1 items-center rounded border">
                  @for (clip of track.clips; track clip.id) {
                    <div
                      class="ellipsis-clip absolute h-[34px] cursor-grab overflow-hidden rounded px-2 py-1 text-[11px] font-semibold text-white/95 shadow-md"
                      [style.left.%]="(clip.start / totalDuration) * 100"
                      [style.width.%]="(clip.duration / totalDuration) * 100"
                      [style.background]="clip.color">
                      {{ clip.title }}
                    </div>
                  }
                </div>
              </div>
            }
          </div>
        </div>
      </div>
    </div>
  `
})
export class TimelineEditorComponent {
  readonly totalDuration = 30;

  isPlaying = signal<boolean>(false);
  currentTime = signal<number>(4);

  tracks = signal<TimelineTrack[]>([
    {
      id: 'v1',
      name: '🎥 Video 1',
      type: 'video',
      clips: [
        { id: 'c1', title: 'Shot #1 (Cyberpunk)', start: 0, duration: 4, color: '#f59e0b' },
        { id: 'c2', title: 'Shot #2 (Aerial)', start: 4, duration: 5, color: '#d97706' },
        { id: 'c3', title: 'Shot #3 (Highway)', start: 9, duration: 6, color: '#b45309' }
      ]
    },
    {
      id: 'a1',
      name: '🎵 Audio (Beat)',
      type: 'audio',
      clips: [
        { id: 'a1', title: 'Synthwave_Beat_124BPM.wav', start: 0, duration: 25, color: '#10b981' }
      ]
    },
    {
      id: 't1',
      name: '✨ Títulos / FX',
      type: 'overlay',
      clips: [{ id: 't1', title: 'Lower Third Title', start: 1, duration: 3, color: '#3b82f6' }]
    }
  ]);

  mediaPool = [
    { title: '🎬 Shot_01_Cyberpunk.mp4', duration: 4 },
    { title: '🎬 Shot_02_Aerial.mp4', duration: 5 },
    { title: '🎵 Synthwave_Master_Track.wav', duration: 30 }
  ];

  playheadPct = computed(() => (this.currentTime() / this.totalDuration) * 100);

  timecode = computed(() => {
    const s = Math.floor(this.currentTime());
    return `00:00:${s.toString().padStart(2, '0')}:00`;
  });

  rulerMarks = computed(() => {
    const marks: string[] = [];
    for (let i = 0; i <= 20; i += 5) {
      marks.push(`00:${i.toString().padStart(2, '0')}`);
    }
    return marks;
  });

  togglePlay() {
    this.isPlaying.update(v => !v);
  }
}
