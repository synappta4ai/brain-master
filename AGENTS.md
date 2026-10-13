# AGENTS.md — brain-master

Instrucciones para agentes de IA en este repo. El corazón acá es
**`inference-worker/`**: la capa 3 que ejecuta los pesos en GPU y corre **a la
par de los otros dos componentes del workspace** (front-synapta :4200 y
back-synapta :8099). Su README detallado:
[`inference-worker/README.md`](inference-worker/README.md). Plan para
instalarlo en local y que el back lo use como un proveedor más:
[`PLAN_WORKER_LOCAL_PROVEEDOR.md`](PLAN_WORKER_LOCAL_PROVEEDOR.md).

## Qué vive acá

| Carpeta | Rol | Puerto |
|---|---|---|
| `inference-worker/` | **Motor de inferencia** (gRPC) + artifact server HTTP | :50051 / :50052 |
| `backend/` | Gateway Go legado (jobs, WebSocket hub, SQLite) del studio propio | :8080 |
| `frontend/` | Studio legado Angular (Studio, Director, Timeline, LoRAs) | :4200 |
| `deploy/` | Recetas de despliegue GPU (Colab, Kaggle, Runpod, Vast, EC2, Fly, Oracle, local) | — |
| `proto/` | `inference.proto` — **contrato gRPC fuente de verdad** | — |

## Reglas críticas

1. **Contrato gRPC**: cambiar `proto/inference.proto` implica regenerar
   TAMBIÉN `back-synapta/internal/worker/proto/` (el back de Synapta mantiene
   un espejo del código generado). No editar los `*_pb2*.py` a mano.
2. **Catálogo de modelos = `inference-worker/engines/base.py`
   (`MODEL_CATALOG`)**: spec declarativa (repo HF, engine, steps, guidance,
   dtype, vram_gb). Back-synapta lista estos modelos **en vivo, sin caché**:
   si el worker no responde, no aparecen. Nunca meter fallbacks ni datos mock
   en el catálogo.
3. **Agregar un modelo**: entrada en `MODEL_CATALOG` + pipeline en
   `engines/diffusion_factory.py` (o su propio engine). Sin tocar el
   back-synapta.
4. **Estado del worker = stateless**: no persistir catálogos ni jobs acá;
   los jobs de Synapta se trackean del lado del back (streaming de eventos).
5. **Artefactos**: `BM_OUTPUT_DIR` (default `../backend/outputs`). Para
   workers remotos, `artifact_server.py` (:50052) los publica y el back los
   baja vía `BM_WORKER_ARTIFACT_BASE`. Un job sin artefacto es un job
   **FAILED** — nunca éxito falso.
6. **Logs y binarios locales** (`*.log`, `__pycache__`, `.pytest_cache`,
   `backend/*.exe`) no se commitean.

## Ejecución

```bash
# Real (necesita GPU NVIDIA + CUDA 12.1+): genera de verdad
cd inference-worker && pip install -r requirements.txt && python server.py

# Mock determinista (sin GPU, sin descargas de pesos):
BM_ARTIFACT_PORT=50052 py -3 inference-worker/run_mock.py

# Tests:
cd inference-worker && python -m pytest

# Front legado:
cd frontend && npm install && npm start   # :4200
# Gateway legado:
cd backend && go run main.go              # :8080
```

## Hardware y despliegue del worker

- **Mínimo**: RTX 3060 12 GB · **Recomendado**: RTX 3090/4090 24 GB (video
  1080p, modelos 14B). Pesos: 10–40 GB por modelo (SSD NVMe).
- **Local (tu GPU)**: costo eléctrico, privacidad total. Es la opción 8 de
  `deploy/README.md`.
- **Hardware dedicado alquilado** (RunPod/Vast ~$0.20-0.35/h por 24 GB):
  Docker con `--gpus all`, pesos en volume, conexión segura vía Tailscale o
  túnel.
- **Gratis para probar**: Colab T4 / Kaggle (notebook en `deploy/colab/`,
  gRPC por **bore.pub** TCP puro + artefactos por **cloudflared** — los quick
  tunnels de cloudflared no transportan gRPC HTTP/2 de forma fiable).
- **Evitar**: hyperscalers sin descuento (10× más caros).

Ver `deploy/README.md` (registro maestro con las 8 opciones y pasos exactos)
y `inference-worker/README.md` (requisitos, engines e integración Synapta).

## Integración con Synapta (recordatorio)

back-synapta es un consumidor más del worker: `BM_WORKER_ADDR`
(`tls://…` para túneles) + `BM_WORKER_ARTIFACT_BASE` en su `.env`. Cualquier
cambio de contrato o de semántica de eventos (LOADING_MODEL → GENERATING →
COMPLETED) impacta `back-synapta/internal/worker/` y
`internal/modules/agency/video/downloaded.go` — verificar el E2E cross-language
antes de cerrar.
