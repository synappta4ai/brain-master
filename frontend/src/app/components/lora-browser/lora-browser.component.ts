import { Component, inject, signal } from "@angular/core";
import { FormsModule } from "@angular/forms";
import { TranslatePipe, TranslateService } from "@ngx-translate/core";
import { ButtonModule } from "primeng/button";
import { InputTextModule } from "primeng/inputtext";
import { SelectModule } from "primeng/select";
import { TagModule } from "primeng/tag";

export interface LoraItem {
  id: string;
  name: string;
  categoryKey: string;
  recommendedWeight: number;
  triggerWords: string[];
  downloads: string;
  installed: boolean;
}

@Component({
  selector: "app-lora-browser",
  standalone: true,
  imports: [
    FormsModule,
    ButtonModule,
    InputTextModule,
    SelectModule,
    TagModule,
    TranslatePipe,
  ],
  template: `
    <div class="flex flex-col gap-5">
      <!-- Búsqueda y filtros -->
      <div
        class="glass-panel flex flex-wrap items-center justify-between gap-4 p-4"
      >
        <div class="min-w-56 flex-1">
          <input
            pInputText
            [(ngModel)]="searchQuery"
            [attr.placeholder]="'LORAS.SEARCH_PLACEHOLDER' | translate"
            class="w-full bg-input-bm"
          />
        </div>
        <div class="flex gap-2.5">
          <p-select
            [(ngModel)]="selectedCategory"
            [options]="categories"
            optionLabel="label"
            optionValue="value"
            styleClass="w-44"
          >
            <ng-template pTemplate="selectedItem" let-option>
              {{ option.label | translate }}
            </ng-template>
            <ng-template pTemplate="item" let-option>
              {{ option.label | translate }}
            </ng-template>
          </p-select>
          <p-button
            [label]="'LORAS.REFRESH' | translate"
            icon="pi pi-refresh"
            severity="secondary"
            [outlined]="true"
          />
        </div>
      </div>

      <!-- Grid de LoRAs -->
      <div class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(280px,1fr))]">
        @for (lora of loras(); track lora.id) {
          <div class="glass-card flex flex-col justify-between p-4">
            <div>
              <div class="mb-2 flex items-start justify-between">
                <h4 class="m-0 text-[15px] font-bold text-primary">
                  {{ lora.name }}
                </h4>
                <p-tag
                  [value]="lora.categoryKey | translate"
                  severity="secondary"
                  [rounded]="false"
                />
              </div>

              <p class="mb-3 text-xs text-muted">
                {{ "LORAS.DOWNLOADS" | translate }}:
                <strong>{{ lora.downloads }}</strong> ·
                {{ "LORAS.SUGGESTED_WEIGHT" | translate }}:
                <strong class="text-primary">{{
                  lora.recommendedWeight
                }}</strong>
              </p>

              <div class="mb-4">
                <span class="mb-1 block text-[11px] text-subtle"
                  >{{ "LORAS.TRIGGER_WORDS" | translate }}:</span
                >
                <div class="flex flex-wrap gap-1">
                  @for (tag of lora.triggerWords; track tag) {
                    <span
                      class="font-mono-bm border-subtle rounded border bg-main-bm px-1.5 py-0.5 text-[10px] text-primary"
                    >
                      {{ tag }}
                    </span>
                  }
                </div>
              </div>
            </div>

            <div class="border-subtle flex items-center gap-2 border-t pt-3">
              @if (!lora.installed) {
                <p-button
                  [label]="'LORAS.INSTALL' | translate"
                  icon="pi pi-download"
                  size="small"
                  styleClass="w-full"
                  (onClick)="toggleInstall(lora)"
                />
              } @else {
                <p-button
                  [label]="
                    ('LORAS.ACTIVE' | translate) +
                    ' (' +
                    lora.recommendedWeight +
                    ')'
                  "
                  icon="pi pi-check-circle"
                  size="small"
                  severity="success"
                  [outlined]="true"
                  styleClass="w-full"
                  (onClick)="toggleInstall(lora)"
                />
              }
            </div>
          </div>
        }
      </div>
    </div>
  `,
})
export class LoraBrowserComponent {
  private readonly translate = inject(TranslateService);

  searchQuery = "";
  selectedCategory = "all";

  /** Labels como keys i18n; se traducen en runtime (el Select muestra el label crudo). */
  categories = [
    { label: "LORAS.ALL_CATEGORIES", value: "all" },
    { label: "LORAS.STYLE", value: "style" },
    { label: "LORAS.CHARACTER", value: "character" },
    { label: "LORAS.LIGHTING", value: "lighting" },
  ];

  loras = signal<LoraItem[]>([
    {
      id: "lora_1",
      name: "Cyberpunk Neon Genesis",
      categoryKey: "LORAS.STYLE",
      recommendedWeight: 0.85,
      triggerWords: ["cyberpunk style", "neon lighting", "wet asphalt"],
      downloads: "142.5k",
      installed: true,
    },
    {
      id: "lora_2",
      name: "Cinematic 35mm Film Grain",
      categoryKey: "LORAS.LIGHTING",
      recommendedWeight: 0.65,
      triggerWords: ["35mm film", "anamorphic bokeh", "kodak 500t"],
      downloads: "98.2k",
      installed: false,
    },
    {
      id: "lora_3",
      name: "Anime Shinkai Sky Aesthetic",
      categoryKey: "LORAS.STYLE",
      recommendedWeight: 0.9,
      triggerWords: ["makoto shinkai style", "glowing clouds", "twilight"],
      downloads: "210.1k",
      installed: false,
    },
  ]);

  toggleInstall(lora: LoraItem) {
    this.loras.update((list) =>
      list.map((l) =>
        l.id === lora.id ? { ...l, installed: !l.installed } : l,
      ),
    );
  }
}
