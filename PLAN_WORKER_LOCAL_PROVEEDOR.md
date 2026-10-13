# Plan — Inference Worker local como un proveedor más de Synapta

> Objetivo: que `inference-worker` (capa 3, Python + CUDA) se instale en la
> máquina local y que **back-synapta lo consuma con la misma experiencia que
> cualquier proveedor externo** (Higgsfield, Gemini…): catálogo, generación,
> progreso real, cancelación, costos y logs — pero corriendo los pesos en
> nuestra propia GPU.
>
> Estado de partida: el pipeline ya existe (`DownloadedGenerator` → gRPC
> `GenerateMedia` con streaming) y funciona; este plan lo lleva a **nivel
> proveedor**: instalación reproducible, confiabilidad ante reinicios,
> observabilidad y costos en 0.

---

## Fase 0 — Diagnóstico del estado actual (hecho, referencia)

| Pieza | Estado |
|---|---|
| Contrato gRPC (`proto/inference.proto`) | ✅ 6 RPCs: `GenerateMedia` (stream), `CancelGeneration`, `TranscribeAudio`, `EnhancePrompt`, `GetGpuTelemetry`, `ListModels` |
| Catálogo de modelos | ✅ declarativo en `inference-worker/engines/base.py` (`MODEL_CATALOG`); el back lo lee **en vivo, sin caché** |
| Generación end-to-end | ✅ `DownloadedGenerator.Generate` (submit + goroutine que consume el stream) → artefacto re-hospedado en `/outputs` |
| Cancelación | ✅ `CancelGeneration` cooperativa (`asyncio.Event`) |
| Reconciler (2 min) | ⚠️ barre tareas no terminales, pero para `downloaded` no puede re-consultar el estado (gap F2) |
| Estimate/costos | ➖ los modelos propios no pasan por `EstimateHiggsfield`; hoy no hay costo (decisión de producto, F6) |
| Instalación local | ❌ manual (README); falta instalador, health-check y doctor |

## Fase 1 — Instalación local reproducible (day 0)

**Meta: `setup` en Windows y Linux deja el worker operativo y verificado.**

1. **Script de instalación** `inference-worker/scripts/setup_local.*` (`.ps1` + `.sh`):
   - Detecta GPU/driver: `nvidia-smi` → nombre, driver, VRAM; valida CUDA 12.1+.
   - Crea venv (Python 3.11) e instala `requirements.txt` (pin de versiones).
   - Genera `inference-worker/.env` desde un `env.example` nuevo:
     `BM_GRPC_PORT=50051`, `BM_ARTIFACT_PORT=50052`, `BM_OUTPUT_DIR`,
     `BM_FORCE_MOCK`, `BM_HF_TOKEN` (opcional), `BM_MAX_VRAM_GB`.
   - **Smoke test**: `BM_FORCE_MOCK=1 python server.py` → `ListModels` OK.
2. **`doctor`** (`scripts/doctor.py`): checks de GPU/VRAM/disco (pesos 10–40 GB),
   driver/CUDA, puertos libres (50051/50052), conectividad desde el back y
   catálogo alcanzable — salida tipo readyz para humanos.
3. **Autoarranque opcional**: tarea programada (Windows) / systemd unit
   (Linux) con `Restart=on-failure`, documentado en el README.
4. **Sincronizar el back**: en `back-synapta/.env` apuntar
   `BM_WORKER_ADDR=127.0.0.1:50051` y dejar `BM_WORKER_ARTIFACT_BASE` vacío
   (disco compartido). Verificación: `GET /api/v1/worker/status` →
   `worker_connected: true` + `GET /models?type=downloaded` con modelos reales.

**Criterio de cierre**: en una máquina limpia, `setup` + `doctor` + arranque
del back muestran el catálogo del worker y una generación de prueba
(`SD-Tiny-Test` o mock) completa en la UI.

## Fase 2 — Confiabilidad: JobStatus por id (la pieza que falta)

**Problema real de hoy**: tras un reinicio del back, el stream gRPC muere y la
tarea `downloaded` queda en `running` para siempre — el reconciler no puede
re-consultar porque el proto no expone `JobStatus(job_id)`.

1. **Proto**: agregar al servicio
   `rpc GetJobStatus (JobStatusRequest) returns (JobStatusResponse);`
   (estado, progreso, error, artifact_name) + regenerar
   `inference_pb2*.py` **y** `back-synapta/internal/worker/proto/` (espejo).
2. **Worker**: en `server.py` ya existe el mapa de jobs activos → devolver
   estado en vivo; para jobs de sesiones previas, persistir un journal ligero
   (`jobs/<job_id>.json`: estado + nombre de artefacto) que sobreviva
   reinicios.
3. **Back**: `internal/worker/client.go` suma `GetJobStatus`;
   `DownloadedGenerator.GetStatus` lo usa (stream vivo si existe, si no RPC)
   y el reconciler recupera tareas huérfanas (COMPLETED → re-hospedar
   artefacto; FAILED → marcar).
4. **Tests**: extender el E2E cross-language
   (`TestE2EWorker`) con el caso "back reinicia a mitad del job y lo recupera
   por GetJobStatus".

**Criterio de cierre**: matar el back durante una generación → al volver,
el reconciler la termina o la marca FAILED — nunca `running` eterno.

## Fase 3 — Experiencia de proveedor en la app (paridad con Higgsfield)

1. **Costos en 0**: para `downloaded`, escribir `cost_source='local_worker'`
   con `cost_usd=0` (o costo eléctrico configurable) — hoy el front espera
   gastar solo para proveedores; definir con producto si el badge muestra
   "GPU propia".
2. **Estimate**: el catálogo ya informa `vram_gb` y pasos; opcionalmente
   exponer un `EstimateLocal` (duración esperada por steps) para el chip de
   la UI.
3. **Cancelación desde la UI**: verificar el flujo completo
   (`CancelTask` → `CancelGeneration`) en cola del studio.
4. **Seed/ratio/resolución**: mapear los campos del request de Synapta al
   proto para modelos que los soporten (hoy `steps`/`cfg_scale` van en 0 y
   aplica defaults del catálogo).

**Criterio de cierre**: en el studio, generar con un modelo local se ve,
cuesta y cancela igual que con un proveedor externo.

## Fase 4 — Observabilidad y operación

1. **Métricas por job** en el worker: duración, VRAM pico, modelo → log
   estructurado + endpoint de telemetría ya existente (`GetGpuTelemetry`)
   visible en `GET /api/v1/worker/status`.
2. **Rate/cola**: un solo job a la vez por VRAM; devolver error claro si se
   satura (el front ya muestra fallos).
3. **Actualizador de pesos**: carpeta `models/` con cache HF persistente;
   comando `pull <modelo>` que pre-descarga pesos (evita cold start en la
   primera generación).

## Fase 5 — Documentación y handoff

1. Actualizar `inference-worker/README.md` (sección "Instalación local" con
   setup/doctor/autoarranque) y el `AGENTS.md` de brain-master.
2. Documentar en `back-synapta/README.md` el modo "worker local como
   proveedor" (variables, verification, troubleshooting).
3. Checklist de la sesión de trabajo diario (worker arriba → back arriba →
   `readyz` + `worker/status` en verde).

---

## Riesgos y decisiones abiertas

| Riesgo / decisión | Nota |
|---|---|
| VRAM insuficiente para modelos 14B | El catálogo ya declara `vram_gb`; filtrar/listar no disponibles y sugerir variantes 1.3B |
| Concurrencia | 1 job a la vez; encolar en el worker o rechazar con error claro (decidir) |
| ¿Costo 0 o costo eléctrico? | Decisión de producto (Fase 3.1) |
| Windows vs Linux | Setup en ambos; Docker + GPU solo tiene sentido en Linux |
| Cambio de proto | Rompe compatibilidad: regenerar ambos lados y desplegar worker+back juntos |
