import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { TooltipModule } from 'primeng/tooltip';
import { SelectModule } from 'primeng/select';
import { TextareaModule } from 'primeng/textarea';
import { InputNumberModule } from 'primeng/inputnumber';
import { SelectButtonModule } from 'primeng/selectbutton';
import { TagModule } from 'primeng/tag';
import { ProgressBarModule } from 'primeng/progressbar';
import { CardModule } from 'primeng/card';
import {
  ApiService,
  CreateJobPayload,
  GpuTelemetry,
  ModelInfo,
} from '../../services/api.service';
import { WebSocketService } from '../../services/websocket.service';

interface ModelOption {
  label: string;
  value: string;
  mode: 'video' | 'image';
  /** true si el backend ejecuta un pipeline real (diffusers). */
  real: boolean;
  /** Pasos de inferencia recomendados para el modelo. */
  steps: number;
  badge?: string;
  /** false si el modelo requiere el volumen V1 no montado. */
  available?: boolean;
  /** VRAM recomendada en GB (del catálogo). */
  vramGb?: number;
}

@Component({
  selector: 'app-studio',
  standalone: true,
  imports: [
    DecimalPipe,
    FormsModule,
    TranslatePipe,
    ButtonModule,
    TooltipModule,
    SelectModule,
    TextareaModule,
    InputNumberModule,
    SelectButtonModule,
    TagModule,
    ProgressBarModule,
    CardModule,
  ],
  templateUrl: './studio.component.html',
})
export class StudioComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly wsService = inject(WebSocketService);
  private readonly translate = inject(TranslateService);

  readonly ws = this.wsService;

  // Form
  readonly prompt = signal('A cinematic shot of a futuristic cyberpunk city in neon rain, 4k, hyperrealistic');
  readonly negativePrompt = signal('low quality, blurry, deformed, cartoon');
  readonly selectedModel = signal('Wan2.1-T2V-14B');
  readonly mode = signal<'video' | 'image'>('video');
  readonly duration = signal(5);
  readonly fps = signal(16);
  readonly isSubmitting = signal(false);

  // Telemetría GPU (del worker Python vía gateway)
  readonly gpu = signal<GpuTelemetry | null>(null);

  readonly modeOptions = [
    { label: 'STUDIO.VIDEO', value: 'video' },
    { label: 'STUDIO.IMAGE', value: 'image' },
  ];

  /** Catálogo espejo (fallback si el worker no responde). */
  private readonly fallbackModels: ModelOption[] = [
    { label: 'Stable Diffusion 1.5 (Real)', value: 'SD-1.5', mode: 'image', real: true, steps: 20, badge: 'REAL' },
    { label: 'SDXL Turbo (Real, 1 paso)', value: 'SDXL-Turbo', mode: 'image', real: true, steps: 2, badge: 'REAL' },
    { label: 'Wan 2.1 Video (14B High Quality)', value: 'Wan2.1-T2V-14B', mode: 'video', real: false, steps: 20 },
    { label: 'MiniMax H3 Video', value: 'MiniMax-H3-High', mode: 'video', real: false, steps: 20 },
    { label: 'LTX-Video 2.5 (Fast Gen)', value: 'LTX-Video-2.5', mode: 'video', real: false, steps: 20 },
    { label: 'Flux 2 Klein (HQ Image)', value: 'Flux-2-Klein-9B', mode: 'image', real: false, steps: 20 },
  ];

  /** Catálogo vivo del worker (ListModels → /api/v1/models). */
  readonly models = signal<ModelOption[]>(this.fallbackModels);
  readonly modelsSource = signal<'worker' | 'fallback'>('fallback');

  readonly filteredModels = computed(() =>
    this.models().filter((m) => m.mode === this.mode() && m.available !== false),
  );

  readonly selectedModelSpec = computed(() =>
    this.models().find((m) => m.value === this.selectedModel()),
  );

  readonly frames = computed(() => this.duration() * this.fps());

  /** Al cambiar de modo, selecciona el primer modelo válido de ese modo. */
  onModeChange(newMode: 'video' | 'image'): void {
    this.mode.set(newMode);
    const current = this.models().find((m) => m.value === this.selectedModel());
    if (!current || current.mode !== newMode || current.available === false) {
      this.selectedModel.set(this.filteredModels()[0].value);
    }
  }

  ngOnInit(): void {
    this.refreshGpu();
    this.refreshModels();
  }

  /** Catálogo dinámico del worker; conserva el espejo local si falla. */
  refreshModels(): void {
    this.api.getModels().subscribe({
      next: (catalog) => {
        if (!catalog.models?.length) return;
        const mapped: ModelOption[] = catalog.models
          .filter((m) => m.mode === 'image' || m.mode === 'video')
          .map((m: ModelInfo) => ({
            label: m.name,
            value: m.id,
            mode: m.mode as 'video' | 'image',
            real: m.engine === 'diffusers',
            steps: m.steps,
            badge: m.engine === 'diffusers' ? 'REAL' : m.engine === 'v1_bridge' ? 'V1' : undefined,
            available: m.available,
            vramGb: m.vram_gb,
          }));
        this.models.set(mapped);
        this.modelsSource.set('worker');
        // El modelo seleccionado pudo desaparecer del catálogo real.
        if (!mapped.some((m) => m.value === this.selectedModel())) {
          this.selectedModel.set(this.filteredModels()[0].value);
        }
      },
      error: () => this.modelsSource.set('fallback'),
    });
  }

  refreshGpu(): void {
    this.api.getGpuTelemetry().subscribe({
      next: (tel) => this.gpu.set(tel),
      error: () => this.gpu.set(null),
    });
  }

  onEnhancePrompt(): void {
    if (!this.prompt()) return;
    this.api.enhancePrompt(this.prompt(), this.selectedModel(), 'cinematic').subscribe({
      next: (res) => this.prompt.set(res.enhanced_prompt),
      error: (err) => console.error('Enhance falló:', err),
    });
  }

  onGenerate(): void {
    if (!this.prompt()) return;
    this.isSubmitting.set(true);

    const payload: CreateJobPayload = {
      mode: this.mode(),
      prompt: this.prompt(),
      negative_prompt: this.negativePrompt(),
      model: this.selectedModel(),
      width: this.mode() === 'image' ? 512 : 1280,
      height: this.mode() === 'image' ? 512 : 720,
      frames: this.mode() === 'video' ? this.frames() : 0,
      fps: this.fps(),
      steps: this.selectedModelSpec()?.steps ?? 20,
    } as CreateJobPayload;

    this.api.createJob(payload).subscribe({
      next: () => this.isSubmitting.set(false),
      error: (err) => {
        console.error('Error al generar:', err);
        this.isSubmitting.set(false);
      },
    });
  }

  get statusSeverity(): 'success' | 'danger' {
    return this.wsService.isConnected() ? 'success' : 'danger';
  }
}
