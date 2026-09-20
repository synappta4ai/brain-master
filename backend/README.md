# 🚀 Brain-Master: API Gateway y Orquestador (Go)

Este servicio constituye la **Capa 2** de la arquitectura híbrida de **Brain-Master**. Está desarrollado en **Go** para proporcionar un servidor HTTP de ultra-alta velocidad, gestión de concurrencia ligera, encolado de tareas para la GPU y transmisión de telemetría en tiempo real por WebSockets hacia la interfaz en Angular.

---

## 🎯 Responsabilidades del Servicio

1. **API Gateway REST:** Expone rutas optimizadas con CORS habilitado para gestión de proyectos, catálogo de LoRAs y encolado de generaciones.
2. **WebSocket Hub (`/ws/telemetry`):** Distribuye eventos de progreso (`JOB_UPDATE`), estado de la GPU y alertas en tiempo real a todos los clientes web conectados.
3. **Gestor de Cola de Tareas (`Job Queue`):** Controla el flujo de trabajo hacia la GPU de forma ordenada y no bloqueante, manejando estados de ciclo de vida (`QUEUED` ➔ `PROCESSING` ➔ `COMPLETED` / `FAILED`).
4. **Servidor de Medios y Streaming:** Entrega de videos (`/outputs/*.mp4`) e imágenes con soporte para solicitudes por rango HTTP (*byte-range requests*).
5. **Orquestación Multimedia:** Puente de comunicación con el worker de inferencia en Python y ensamblado de línea de tiempo con FFmpeg.

---

## 📁 Estructura Interna del Paquete

```
brain-master/backend/
├── go.mod                      # Definición de dependencias del módulo Go
├── main.go                     # Punto de entrada, inicialización de Hub y enrutador HTTP
├── brain-master-server.exe     # Binario compilado listo para ejecución
├── outputs/                    # Directorio de videos y medios renderizados
│   └── .gitkeep
├── internal/
│   ├── api/
│   │   └── handlers.go         # Controladores REST (/api/v1/health, /api/v1/jobs) y middleware CORS
│   ├── ws/
│   │   └── hub.go              # WebSocket Hub concurrente (registro, desconexión y broadcast)
│   └── queue/
│       └── manager.go          # Cola de tareas concurrentes y worker loop hacia la GPU
└── README.md                   # Este documento
```

---

## ⚙️ Variables de Entorno

| Variable | Tipo | Valor por Defecto | Descripción |
| :--- | :--- | :--- | :--- |
| `PORT` | `string` | `8080` | Puerto en el que escuchará el servidor HTTP y WebSocket |
| `PYTHON_WORKER_HOST` | `string` | `localhost:50051` | Dirección del microservicio de inferencia en Python |
| `OUTPUTS_DIR` | `string` | `./outputs` | Ruta local donde se almacenan los videos renderizados |

---

## 🔌 Especificación de la API REST

### 1. Health Check
* **Ruta:** `GET /api/v1/health`
* **Respuesta Exitosa (`200 OK`):**
  ```json
  {
    "service": "brain-master-gateway",
    "status": "healthy",
    "timestamp": "2026-09-19T02:54:28Z"
  }
  ```

---

### 2. Encolar Trabajo de Generación (Studio / Director)
* **Ruta:** `POST /api/v1/jobs/create`
* **Headers:** `Content-Type: application/json`
* **Payload:**
  ```json
  {
    "mode": "video",
    "prompt": "A cinematic shot of a futuristic cyberpunk city in neon rain, 4k",
    "negative_prompt": "low quality, blurry",
    "model": "Wan2.1-T2V-14B",
    "width": 1280,
    "height": 720,
    "frames": 81
  }
  ```
* **Respuesta (`202 Accepted`):**
  ```json
  {
    "id": "job_1789786498062144100",
    "mode": "video",
    "prompt": "A cinematic shot of a futuristic cyberpunk city in neon rain, 4k",
    "model": "Wan2.1-T2V-14B",
    "width": 1280,
    "height": 720,
    "frames": 81,
    "status": "QUEUED",
    "progress": 0,
    "created_at": "2026-09-19T00:00:00Z"
  }
  ```

---

### 3. Listar Trabajos
* **Ruta:** `GET /api/v1/jobs`
* **Respuesta (`200 OK`):** Retorna la lista completa de tareas con su progreso y ruta de salida.

---

### 4. Consultar Detalle de un Trabajo
* **Ruta:** `GET /api/v1/jobs/detail?id={job_id}`

---

## 📡 Protocolo WebSocket (`/ws/telemetry`)

* **URL de Conexión:** `ws://127.0.0.1:8080/ws/telemetry`
* **Estructura del Mensaje Emitido:**
  ```json
  {
    "type": "JOB_UPDATE",
    "data": {
      "id": "job_1789786498062144100",
      "mode": "video",
      "prompt": "Cyberpunk metropolis",
      "model": "Wan2.1-T2V-14B",
      "status": "PROCESSING",
      "progress": 70.0,
      "output_path": "/outputs/job_1789786498062144100.mp4"
    }
  }
  ```

---

## 🛠️ Compilación y Ejecución Local

### 1. Ejecutar en modo desarrollo
```powershell
cd brain-master/backend
go run main.go
```

### 2. Compilar binario de producción
```powershell
cd brain-master/backend
go mod tidy
go build -o brain-master-server.exe .
```

### 3. Ejecutar binario compilado
```powershell
.\brain-master-server.exe
```

---

## 🧪 Comandos de Prueba (PowerShell / cURL)

### Verificar Estado del Servidor
```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/health" | ConvertTo-Json
```

### Crear y Encolar una Tarea de Video
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

### Consultar Trabajos Activos y Completados
```powershell
Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/v1/jobs" | ConvertTo-Json
```

---

## 🐳 Dockerfile de Producción (Ejemplo)

```dockerfile
# Etapa 1: Compilación
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /brain-master-gateway .

# Etapa 2: Imagen final ligera (Scratch / Alpine)
FROM alpine:3.19

WORKDIR /app
RUN apk add --no-cache ffmpeg ca-certificates

COPY --from=builder /brain-master-gateway .
RUN mkdir -p /app/outputs

EXPOSE 8080

CMD ["./brain-master-gateway"]
```
