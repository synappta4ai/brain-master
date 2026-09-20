# 🧠 Brain-Master: AI Creative Studio (Hybrid Architecture)

**Brain-Master** es la reimplementación modular y desacoplada de la suite creativa multimedia **Maestro AI**, construida sobre una **Arquitectura Híbrida de 3 Capas**:
1. **Frontend:** Angular 21 + Tailwind CSS v4 + PrimeNG 21 (tema oscuro/claro, Signals & RxJS)
2. **API Gateway & Orquestador:** Go (HTTP REST, WebSocket Hub & Queue Manager)
3. **Motor de Inferencia IA:** Python (PyTorch, Diffusers, CUDA Kernels & VRAM Sentinel)

---

## 📁 Estructura del Proyecto

```
brain-master/
├── proto/                         # Contratos gRPC / Protocol Buffers
│   └── inference.proto            # Definición de servicios e interfaces Go <-> Python
├── backend/                       # Capa 2: Orquestador y API Gateway (Go)
│   ├── go.mod                     # Módulo de Go (compilación verificada)
│   ├── main.go                    # Servidor HTTP REST (puerto 8080) y Hub WebSocket
│   ├── brain-master-server.exe    # Binario ejecutable compilado
│   ├── outputs/                   # Directorio de salida para videos y medios generados
│   └── internal/
│       ├── api/handlers.go        # Endpoints REST (/api/v1/jobs, /api/v1/health)
│       ├── ws/hub.go              # WebSocket Hub para telemetría en tiempo real
│       └── queue/manager.go       # Cola concurrente de tareas GPU con ciclo de vida
├── inference-worker/              # Capa 3: Motor de Inferencia IA (Python)
│   ├── requirements.txt           # Dependencias de PyTorch, Diffusers, gRPC
│   ├── server.py                  # Ejecutor de inferencia y streaming de pasos
│   └── vram_manager.py            # Telemetría de GPU y liberación de VRAM (torch.cuda.empty_cache)
├── frontend/                      # Capa 1: Cliente Web (Angular 21)
│   ├── angular.json               # Configuración de compilación y servidor Angular
│   ├── proxy.conf.json            # Proxy de dev (/api y /ws → Gateway :8080)
│   ├── .postcssrc.json            # PostCSS con @tailwindcss/postcss (Tailwind v4)
│   ├── package.json               # Dependencias de Angular 21, RxJS 7.8 y Signals
│   ├── tsconfig.json              # Configuración TypeScript estricta
│   ├── src/
│   │   ├── proxy.conf.json        # Proxy de dev: /api y /ws hacia el Gateway :8080
│   │   ├── index.html             # HTML base con fuentes Inter y JetBrains Mono
│   │   ├── styles.css             # Tailwind v4 + paleta de marca + tokens dark/light + capas CSS
│   │   ├── main.ts                # Bootstrap de la aplicación standalone
│   │   └── app/
│   │       ├── app.config.ts      # Providers: Router, HttpClient, PrimeNG, i18n (@ngx-translate)
│   │       ├── app.routes.ts      # Rutas con lazy loading (Studio, Director, Editor, LoRAs, Jobs)
│   │       ├── app.component.ts   # Shell: nav por RouterLink, popover de apariencia, RouterOutlet
│   │       ├── pages/
│   │       │   ├── studio/        # 🎨 Generación + telemetría GPU + monitor en vivo
│   │       │   └── jobs/          # 📋 Historial persistido (p-table, filtros, cancelar)
│   │       ├── services/
│   │       │   ├── api.service.ts # Cliente HTTP tipado para el Gateway de Go
│   │       │   ├── websocket.service.ts # Flujo reactivo WebSocket para telemetría en vivo
│   │       │   ├── theme.service.ts # Tema + 6 paletas en runtime (presets PrimeNG)
│   │       │   └── language.service.ts # i18n: es / en / fr con detección del navegador
│   │       └── components/
│   │           ├── director/      # 🎬 Director Mode (Shot & Beat Planner)
│   │           ├── timeline-editor/ # ✂️ Timeline Editor Multipista
│   │           └── lora-browser/  # 🛒 CivitAI LoRA Browser & Installer
├── deploy/                        # Despliegues del worker en plataformas GPU
│   ├── README.md                  # ⭐ Registro maestro: 8 opciones (Colab, Kaggle, Runpod…)
│   ├── colab/                     # Notebook T4 gratis + connect-gateway.ps1
│   ├── kaggle/                    # Variante Kaggle (30 h/semana)
│   ├── cpu/                       # Dockerfile + compose CPU (Oracle Always Free)
│   ├── fly/                       # fly.toml (GPU a10g bajo demanda)
│   ├── ec2/                       # user-data.sh (EC2 g5 / Lambda / GCP)
│   └── build-push.sh              # Build + push de las 3 imágenes a registry
└── docker-compose.yml             # Orquestación de contenedores multi-servicio
```

---

## 🔌 Especificación de la API (Go Gateway en `:8080`)

### Endpoints REST

| Método | Endpoint | Descripción | Payload / Parámetros |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/health` | Estado del gateway, uptime y conexión del worker | N/A |
| `POST` | `/api/v1/jobs/create` | Encolar nueva tarea en la GPU | `{"mode": "video", "prompt": "...", "model": "Wan2.1-T2V-14B", "width": 1280, "height": 720, "frames": 81}` |
| `GET` | `/api/v1/jobs` | Listar todas las tareas y estados | N/A |
| `GET` | `/api/v1/jobs/detail` | Consultar estado de una tarea | Query param `?id={job_id}` |
| `POST` | `/api/v1/jobs/cancel` | Cancelar una tarea en caliente | Query param `?id={job_id}` |
| `GET` | `/api/v1/gpu/telemetry` | Telemetría GPU del worker Python (gRPC) | N/A |
| `GET` | `/api/v1/models` | Catálogo dinámico del worker (ListModels): modelos, VRAM, disponibilidad | N/A |
| `POST` | `/api/v1/prompts/enhance` | Mejorar prompt con el LLM del worker | `{"raw_prompt": "...", "target_model": "...", "style": "cinematic"}` |

> Sin persistencia externa: los jobs se guardan en `backend/brain-master.db` (SQLite puro-Go,
> sobreviven a reinicios del gateway y los `PROCESSING` huérfanos se reencolan solos).

### Pipeline gRPC real (Go ↔ Python)

El gateway ya no simula el trabajo: cuando el worker Python está arriba (`inference-worker/server.py`, gRPC `:50051`),
encola cada job vía `InferenceService.GenerateMedia` con **streaming de progreso real**, escribe el archivo de salida
en `backend/outputs/` y soporta **cancelación cooperativa** (`CancelGeneration`). Si el worker no responde, el gateway
arranca igual en **modo simulación** y el front sigue operativo. El worker expone además `EnhancePrompt` (heurística
de estudio local) y `GetGpuTelemetry` (VRAM vía PyTorch).

### WebSocket Hub

* **URL de Conexión:** `ws://127.0.0.1:8080/ws/telemetry`
* **Eventos Emitidos (`JOB_UPDATE`):**
  ```json
  {
    "type": "JOB_UPDATE",
    "data": {
      "id": "job_1789786498062144100",
      "mode": "video",
      "prompt": "A cinematic cyberpunk city in neon rain",
      "model": "Wan2.1-T2V-14B",
      "status": "PROCESSING",
      "progress": 60.0,
      "output_path": "/outputs/job_1789786498062144100.mp4"
    }
  }
  ```

---

## 🚀 Guía de Ejecución Local

### 1. Iniciar el API Gateway en Go
```powershell
cd brain-master/backend
go run main.go
# o ejecutar el binario compilado:
.\brain-master-server.exe
```
* Servidor activo en `http://127.0.0.1:8080`
* Health check: `http://127.0.0.1:8080/api/v1/health`

### 2. Ejecutar el Worker de Inferencia Python
```powershell
cd brain-master/inference-worker
python server.py
```

### 3. Iniciar el Frontend en Angular
```powershell
cd brain-master/frontend
npm install
npm start
```
* Acceso a la interfaz web: `http://localhost:4200`

---

## ☁️ Despliegue del Worker en la Nube — GPU gratis con Google Colab

El worker es autocontenido: gRPC en `:50051`, artefactos HTTP en `:50052`
y pesos descargados bajo demanda. Eso permite ejecutar la **capa 3 en la T4
de Colab (gratis)** mientras el gateway y el front siguen en tu máquina:

```
┌──────────────────────┐   gRPC (bore.pub:puerto)   ┌───────────────────────┐
│ Tu máquina (Windows)  │ ─────────────────────────► │ Colab T4 16 GB gratis  │
│ Gateway Go :8080      │ ◄───────────────────────── │ :50051 gRPC            │
│ Front Angular :4200   │   artifact_url (HTTP)      │ :50052 artifact server │
└──────────────────────┘                            └───────────────────────┘
```

### Flujo completo Colab paso a paso

> Artefactos: `deploy/colab/brain_master_worker.ipynb` (notebook) y
> `deploy/colab/connect-gateway.ps1` (conexión desde Windows).
> El registro con las 8 plataformas alternativas vive en `deploy/README.md`.

**Paso 1 — Abrir el notebook en Colab**

1. Ve a [colab.research.google.com](https://colab.research.google.com) →
   **File → Upload notebook** → sube `deploy/colab/brain_master_worker.ipynb`.
2. **Runtime → Change runtime type → T4 GPU** (imprescindible, es gratis).

**Paso 2 — Darle acceso al código** (celda 2, dos opciones)

* **Opción A**: sube `inference-worker/` comprimido con el panel de archivos
  de Colab y descomprímelo en `/content/inference-worker`.
* **Opción B**: clona tu fork ajustando `REPO_URL` en la celda.

**Paso 3 — Ejecutar las celdas 1 → 4 en orden**

La celda 3 instala el stack de IA y los binarios de túnel; la celda 4 arranca
el worker y los dos túneles, e imprime las **dos líneas de conexión**:

```
PYTHON_WORKER_HOST=bore.pub:50051          ← gRPC (TCP puro vía bore)
BM_WORKER_ARTIFACT_BASE=https://xxx.trycloudflare.com   ← artefactos (cloudflared)
```

> ¿Por qué dos túneles? gRPC es HTTP/2 crudo: los quick-tunnels de
> cloudflared no lo transportan de forma fiable, así que el gRPC va por
> **bore** (TCP puro, sin registro) y solo los artefactos HTTP van por
> cloudflared.

**Paso 4 — Verificación automática (celdas 5 y 6)**

La celda 5 lista el catálogo (`cuda=True`, T4) y genera una imagen real con
`SD-Tiny-Test`; la celda 6 conecta **a través de `bore.pub`** (el mismo
camino que usará el gateway) y valida el artifact server por HTTPS.

**Paso 5 — Conectar tu gateway (en tu máquina)**

```powershell
.\deploy\colab\connect-gateway.ps1 -WorkerHost "bore.pub:50051" `
                                    -ArtifactBase "https://xxx.trycloudflare.com"
```

El script reinicia el gateway con `PYTHON_WORKER_HOST` y
`BM_WORKER_ARTIFACT_BASE`, verifica `worker_connected: true` y el catálogo,
y lanza un **job de prueba** esperando su `artifact_url` descargable.
Con todo verde: abre `http://localhost:4200` y genera — la GPU es la T4.

**Reconexión**

La sesión free dura horas y se corta al cerrar la pestaña. Al volver:
re-ejecuta la **celda 4** (puerto/URL nuevos) y vuelve a correr el `.ps1`.
El gateway restaura el historial de jobs desde SQLite; los pesos ya
descargados viven mientras la sesión no se recicle.

**Solución de problemas**

| Síntoma | Causa / arreglo |
|---|---|
| `worker_connected: false` | Túnel de bore caído → re-ejecuta la celda 4 y el ps1 |
| Job `FAILED: sin memoria` | Modelo > T4 (Wan-14B, Flux-dev) → usa SD-1.5/SDXL-Turbo/LTX |
| `artifact_url` no descarga | Túnel cloudflared caducado → celda 4 y actualizar `BM_WORKER_ARTIFACT_BASE` |
| Primera generación lenta | Descarga de pesos (SD-Tiny ~100 MB, SD-1.5 ~4 GB) — es una sola vez por sesión |

**Variables de entorno que gobiernan el despliegue dividido**

| Variable | Dónde | Efecto |
|---|---|---|
| `PYTHON_WORKER_HOST=bore.pub:50051` | gateway | gRPC del gateway hacia el worker remoto (soporta prefijo `tls://`) |
| `BM_WORKER_ARTIFACT_BASE=https://tunel` | gateway | publica `artifact_url` en los jobs COMPLETED |
| `BM_ARTIFACT_PORT=50052` | worker | artifact server ON (0 = off) |
| `BM_ARTIFACT_TOKEN=secreto` | worker | exige token en `X-Artifact-Token` o `?token=` |

---

## 🧪 Pruebas y Verificación con PowerShell / cURL

### 1. Test de Health Check
```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/health" | ConvertTo-Json
```

### 2. Encolar una Tarea de Video en la GPU
```powershell
$body = @{
    mode = "video"
    prompt = "A cinematic shot of a futuristic cyberpunk city in neon rain, 4k"
    model = "Wan2.1-T2V-14B"
    width = 1280
    height = 720
    frames = 81
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/jobs/create" -Method Post -Body $body -ContentType "application/json" | ConvertTo-Json
```

### 3. Consultar Trabajos Encolados y Completados
```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/jobs" | ConvertTo-Json
```
