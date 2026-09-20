# Despliegue del Inference-Worker (plataforma GPU)

El worker es un servicio gRPC **autocontenido**: expone `:50051`, escribe
artefactos en `BM_OUTPUT_DIR` y descarga pesos bajo demanda a `HF_HOME`.
Todo el catálogo (24 modelos / 18 familias) se declara en
`inference-worker/engines/base.py` y se consulta vía `ListModels` (gRPC) o
`GET /api/v1/models` (REST en el gateway).

## Opciones de plataforma

| Plataforma | Cómo | Notas |
|---|---|---|
| **Runpod** | Pod GPU → template docker `brainmaster-worker:latest` | Monta un network volume en `/models-cache` para pesos persistentes. Abre solo el puerto del gateway (8080); el worker queda interno. |
| **Vast.ai** | Instance docker con `--gpus all` equivalente | Elige imagen CUDA 12.4; instala docker-compose y levanta el stack. |
| **Lambda / EC2 g5** | VM + NVIDIA Container Toolkit | `docker compose up -d --build` y listo. |
| **Fly.io** | `fly deploy` con GPU (a10g) | Solo worker; gateway/front en máquinas CPU separadas. |
| **Estación local** | NVIDIA Container Toolkit | Compose funciona igual; úsalo también en Windows con WSL2. |

## Requisitos de GPU

- Driver NVIDIA ≥ 550 (CUDA 12.4)
- VRAM mínima por modelo: ver campo `vram_gb` del catálogo (`/api/v1/models`)
  — SD-Tiny 2 GB · SD-1.5 4 GB · SDXL 8-12 GB · Flux-dev 24 GB ·
  video (Wan/LTX) 16-48 GB
- Disco: los pesos van de 2 GB (tiny) a 40+ GB (video 14B); el volume
  `hf-cache` evita re-descargas entre reinicios

## Pasos (cualquier plataforma)

```bash
# 1. Build de las 3 imágenes
docker compose build

# 2. Levantar el stack
docker compose up -d

# 3. Verificar
curl http://localhost:8080/api/v1/health   # {"status":"ok","worker_connected":true}
curl http://localhost:8080/api/v1/models   # catálogo dinámico completo
```

## Generar un job contra la plataforma

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"mode":"image","model_name":"SD-1.5","prompt":"a neon city at dusk","steps":20}'
```

El progreso llega por WebSocket (`/ws`) y el artefacto queda en el volume
`outputs`.

## Familias con código propietario (bridge V1)

Las familias `ltx25`, `minimax_h3`, `kandinsky5`, `krea2`, `longcat`,
`ideogram4`, `z_image`, `yue2` usan handlers propios de la V1 (WanGP/MMGP,
licencias SLA). Se activan montando el árbol `app/models` de la V1:

```yaml
  python-inference:
    volumes:
      - ./v1-models:/models:ro
```

Sin el volumen, esas familias aparecen con `available: false` en el catálogo
y el resto funciona con normalidad.

## Variables de entorno del worker

| Variable | Default | Uso |
|---|---|---|
| `GRPC_PORT` | `50051` | Puerto gRPC |
| `BM_OUTPUT_DIR` | `../backend/outputs` | Destino de artefactos |
| `HF_HOME` | `~/.cache/huggingface` | Cache de pesos |
| `BM_V1_MODELS_DIR` | `/models` | Raíz del árbol V1 (bridge) |
| `BM_FORCE_MOCK` | unset | `1` fuerza el motor mock (diagnóstico) |
