# 📋 PENDIENTE — Mapa de trabajo restante (migración V1 → V2)

> Actualizado: 2026-09-20. La plataforma (capa de orquestación, despliegue
> GPU, catálogo, tests) está **completa y verificada**. Esto es lo que falta
> para que brain-master tenga TODAS las capacidades de la V1.
>
> Referencias: `README.md` (flujo Colab) · `deploy/README.md` (8 plataformas)
> · `REPOS.md` (estructura de repos) · `ARQUITECTURA_HIBRIDA_ANGULAR_GO_PYTHON.md`

---

## 0 · Pendientes inmediatos de plataforma (rápidos)

- [ ] **Commitear el fix del dropdown** — `frontend/src/app/pages/studio/studio.component.html`
      tiene cambios sin commit (plantillas `pTemplate="item"`/`"selectedItem"`
      de PrimeNG 21; arregla los labels duplicados del selector de modelos).
- [ ] **Publicar el repo a GitHub** — `brain-master/.git` existe con 2 commits
      pero **sin remote**. Pasos en `REPOS.md` §1.
- [ ] **CI nueva** — la CI de V1 se borró con la limpieza; crear
      `.github/workflows/ci.yml` con las 3 suites: `go test ./...`,
      `pytest`, `npm test` (+ build de imágenes docker al pasar).
- [ ] (Opcional) split en 3 repos por capa — `REPOS.md` §2.
- [ ] (Menor) la carpeta `ui/` vacía en el disco raíz está retenida por un
      proceso de Windows; se borra al reiniciar. No afecta nada.

## 1 · Motores de IA reales (Bloque 2 — el más grueso)

### 1.1 Validar difusión de imagen con pesos completos
- [ ] Descargar y validar **SD-1.5** a 512×512 con tiempos medidos (~4 GB)
- [ ] Validar **SDXL-Turbo / SDXL-Base** (~7-12 GB)
- [ ] Validar **Flux-schnell** y **Qwen-Image** (16-24 GB — mejor en T4/4090)
- Estado: la factoría (`inference-worker/engines/diffusion_factory.py`) ya
  los ejecuta; solo falta la primera generación real por modelo.
  En la T4 de Colab (`deploy/colab/`) es donde mejor se valida.

### 1.2 Primer motor de VIDEO real
- [ ] **LTX-Video** (viable en T4 16 GB) — habilitar `ltx` en la factoría,
      validar encoder MP4, streaming de frames y cancelación entre pasos
- [ ] **Wan 2.1 T2V 1.3B** (12-16 GB) y luego 14B con offload
- [ ] CogVideoX-5B, HunyuanVideo (24-48 GB)
- Nota: el contrato ya soporta video (proto `num_frames`/`fps`); los motores
  están en mock (`mock_engine.py`) — el punto de sustitución está marcado.

### 1.3 TTS real (V1 tenía 20 familias)
- [ ] **Chatterbox** (voz, la más usada) con streaming de audio
- [ ] IndexTTS2, ACE-Step (voz+music según V1)
- [ ] Biblioteca de personajes / voice clone (V1: voice clone service)

### 1.4 Música
- [ ] **YuE2** y ACE-Step music

### 1.5 Familias propietarias vía bridge V1
- [ ] **Restaurar `app/models` desde el historial de git** (se borró del
      disco con la limpieza de V1):
      `git show <commit-pre-limpieza> --stat -- app/models` para ver qué
      había y `git checkout <commit> -- app/models` para restaurarlo
- [ ] Montarlo en la plataforma GPU (`BM_V1_MODELS_DIR=/models`) y validar
      una familia: LTX-2.5 22B, MiniMax-H3, Kandinsky5, Krea2, LongCat,
      Ideogram4, Z-Image
- Estado: el bridge está codeado (`inference-worker/engines/v1_bridge.py`)
  y el catálogo ya las marca `available: false` hasta montar el volumen.

### 1.6 Pre/post-proceso (servicios V1 sin contraparte)
- [ ] SAM3, DWPose, depth (pre-proceso de video)
- [ ] RIFE (interpolación), upscaling, face refiner (post)
- [ ] MMAudio (sfx desde video), separación de voces

## 2 · Studio real (Bloque 3 — mayor volumen de UI)

Hoy: 1 form de 5 campos. V1 tenía ~50 modos. Migrar por oleadas:
- [ ] image-ref (imagen de referencia en el prompt)
- [ ] restyle / recast (re-estilizar o re-cast de un output previo)
- [ ] outpaint / blend
- [ ] control-video / continue-video
- [ ] retake (regenerar con misma seed)
- [ ] SFX, presets de resolución, post-processing por job
- [ ] outputs en batch + cola global con revisión
- Nota: cada modo = un panel nuevo en `pages/studio/` + campo(s) en el
  payload REST + soporte en el proto (`MediaGenerationRequest`) si aplica.

## 3 · Director Mode real (Bloque 4)

Hoy: demo con 3 shots hardcodeados (`components/director/`).
- [ ] Análisis de audio/BPM del track de fondo
- [ ] `story_ledger` de continuidad narrativa entre shots
- [ ] Planner de tomas + plan retry
- [ ] Encolar los shots como **jobs reales encadenados** (la infra de jobs,
      WebSocket y cancelación ya existe en el gateway)

## 4 · Editor multipista real (Bloque 5)

Hoy: mock estático (`components/timeline-editor/`).
- [ ] Timeline interactivo con clips reales (drag/trim/inspector)
- [ ] **Motor FFmpeg en el gateway Go** (unir/transcodificar/exportar)
- [ ] Round-trip con el backend (proyectos persistidos, re-apertura)
- [ ] Descarga segura del export

## 5 · Ecosistema (Bloque 6)

- [ ] **LoRAs/CivitAI reales** (hoy demo): API client, import con descarga,
      compatibilidad por modelo, gestión de pesos
- [ ] Galería con miniaturas y filtros
- [ ] Plugins (9 en V1), recipes
- [ ] Notificaciones push, recuperación de OOM desde la UI
- [ ] Dashboard de almacenamiento (hf-cache/outputs), acceso remoto

---

## Estado de referencia (para medir avance)

| Suite | Tests | Estado |
|---|---|---|
| Go (`backend`) | 27 | ✅ verdes |
| Python (`inference-worker`) | 39 | ✅ verdes |
| Angular (`frontend`) | 28 + 7 E2E | ✅ verdes |

Stack vivo verificado: worker `:50051`+`:50052`, gateway `:8080`
(`worker_connected: true`), front `:4200`, catálogo dinámico de 24 modelos
con dropdown corregido.
