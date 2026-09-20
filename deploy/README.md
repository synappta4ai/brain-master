# Registro de Despliegues GPU — Brain-Master Inference Worker

> Archivo maestro de opciones de despliegue para la capa 3 (worker Python de
> inferencia) y el stack completo. Cada opción indica **coste**, **cuándo
> usarla** y **pasos exactos**. Los artefactos de cada una viven en
> `deploy/<opcion>/`.

## Resumen ejecutivo

| # | Opción | Coste | Duración | GPU | Artefacto | Ideal para |
|---|--------|-------|----------|-----|-----------|------------|
| 1 | **Google Colab** | $0 | sesiones de horas | T4 16 GB | `deploy/colab/brain_master_worker.ipynb` | probar el stack completo gratis HOY |
| 2 | **Kaggle** | $0 | 30 h/semana | T4 x2 / P100 | `deploy/kaggle/brain_master_worker.py` | sesiones más largas que Colab, sin cuenta de pago |
| 3 | **Runpod** | ~$0.34/h (RTX 4090) | 24/7 alquilado | 4090 / A6000 / H100 | compose + `deploy/build-push.sh` | producción seria barata |
| 4 | **Vast.ai** | ~$0.20/h (3090) | 24/7 alquilado | gama alta barata | compose | la GPU más barata por hora |
| 5 | **EC2 / Lambda / GCP** | ~$0.7-1/h | 24/7 o spot | A10G, L4, V100 | `deploy/ec2/user-data.sh` | entornos corporativos/nube clásica |
| 6 | **Fly.io** | ~$2.5/h (a10g) | bajo demanda | A10G 24 GB | `deploy/fly/fly.toml` | desplegar con un solo comando `fly deploy` |
| 7 | **Oracle Always Free** (CPU) | $0 | 24/7 perpetuo | — (CPU) | `deploy/cpu/` | gateway+worker tiny 24/7 gratis |
| 8 | **Estación local** | costo eléctrico | 24/7 | tu GPU | `docker compose up` | si ya tienes RTX en casa |

**Recomendación por escenario**:
- ¿Probar todo gratis esta tarde? → **1. Colab**
- ¿Uso personal sostenido gratis? → **2. Kaggle** (30 h/semana) + **7. Oracle** para gateway 24/7
- ¿Producción seria a bajo coste? → **3. Runpod** o **4. Vast.ai** con autosave de pesos en network volume
- ¿Ya tienes GPU local? → **8. local** (compose, cero fricción)

---

## Arquitectura de despliegue dividido

Cuando el worker corre en la nube y el gateway en otra máquina:

```
┌─────────────┐   gRPC (tls://)    ┌──────────────────────────┐
│ Gateway Go  │ ─────────────────► │ Worker Python en la nube │
│ (Oracle/VM) │                    │  :50051 gRPC             │
│             │ ◄───────────────── │  :50052 artifact server  │
└──────┬──────┘  artifact_url      └──────────────────────────┘
       │                                  │ descarga pesos HF
       ▼                                  ▼
   Front Angular                    volumen persistente
   (Cloudflare Pages)                (hf-cache)
```

Variables que lo gobiernan (todas ya implementadas y testeadas):

| Variable | Dónde | Efecto |
|---|---|---|
| `PYTHON_WORKER_HOST=tls://host:443` | gateway | gRPC sobre TLS (túneles cloudflared / Fly) |
| `BM_WORKER_ARTIFACT_BASE=https://tunel` | gateway | publica `artifact_url` en cada job completado |
| `BM_ARTIFACT_PORT=50052` | worker | artifact server ON (0 = off) |
| `BM_ARTIFACT_TOKEN=secreto` | worker | exige token (header `X-Artifact-Token` o `?token=`) |
| `BM_OUTPUT_DIR=/ruta` | worker | carpeta expuesta por el artifact server |
| `BM_V1_MODELS_DIR=/models` | worker | activa familias bridge V1 (LTX-2.5, MiniMax-H3…) |

Sin `BM_WORKER_ARTIFACT_BASE`, el gateway sigue sirviendo `output_path`
local (modo monolito docker-compose, que comparte el volumen `outputs`).

---

## 1 · Google Colab (gratis, T4 16 GB)

**Artefactos**: `deploy/colab/brain_master_worker.ipynb` + `deploy/colab/connect-gateway.ps1`

1. Abre el notebook en [colab.research.google.com](https://colab.research.google.com)
   (File → Upload notebook), selecciona runtime **T4 GPU** y ejecuta las
   celdas 1-4.
2. Túneles: **bore** (TCP puro, sin registro) para gRPC → `bore.pub:<puerto>`;
   **cloudflared** para el artifact server HTTP.
3. La celda 4 imprime las variables **y las publica sola en un topic efímero
   de ntfy.sh** (auto-announce, cero copy/paste):
   ```
   PYTHON_WORKER_HOST=bore.pub:<puerto>
   BM_WORKER_ARTIFACT_BASE=https://<tunel>.trycloudflare.com
   ntfy.topic = bm-<hex>
   ```
4. En tu máquina, dos opciones (ambas reinician el gateway, verifican health,
   catálogo y lanzan un job de prueba con `artifact_url` descargable):
   - **Watcher automático (recomendado)**: `py -3 deploy/colab/watch-announce.py`
     (o `watch-announce.ps1`) escucha el topic fijo `bm-brain-master-tunnels-v1`
     — donde la celda 4 publica — y reconecta el gateway solo ante cada
     announce nuevo. Cero comandos tras cada reconexión de Colab.
   - **Manual**: `connect-gateway.ps1 -NtfyTopic "<topic>"` con el topic
     impreso, o `-WorkerHost "bore.pub:PORT" -ArtifactBase "https://..."`.

**Límites**: sesión de ~horas (se corta al cerrar la pestaña). Al reconectar
re-ejecuta la celda 4 (nuevo puerto/URL): con el watcher corriendo la
reconexión es automática; con el ps1, corre el comando de nuevo. Las celdas
5-6 verifican el catálogo y los túneles desde fuera, igual que lo hará el
gateway. El smoke test genera una imagen SD-Tiny-Test real en la T4.

## 2 · Kaggle (gratis, 30 h/semana, T4 x2)

**Artefacto**: `deploy/kaggle/brain_master_worker.py`

Igual que Colab pero: acelerador GPU en Settings, Internet ON, y ejecuta
`%run brain_master_worker.py`. Sesiones hasta 12 h (9 h GPU). Persistencia:
`/kaggle/working` se guarda como output del notebook (los artefactos
generados quedan descargables desde Kaggle incluso tras apagarse).

## 3 · Runpod (producción barata)

1. `./deploy/build-push.sh` publica las imágenes a tu registry (Docker Hub).
2. En Runpod: **Pods → Deploy** → imagen `REGISTRY/brainmaster-worker:latest`.
3. Monta un **Network Volume** en `/models-cache` (pesos persistentes entre
   pods) y expón los puertos 50051 + 50052.
4. En el gateway: `PYTHON_WORKER_HOST=<pod-ip>:50051` (gRPC directo, sin
   TLS) y `BM_WORKER_ARTIFACT_BASE=https://<pod-proxy>`.

Alternativa serverless: **Runpod Serverless** no sirve aún porque el
protocolo es gRPC streaming (no HTTP request/response); usar pods siempre-on
o **auto-stop** (el gateway tiene reintentos y el pod arranca en ~40 s).

## 4 · Vast.ai

Igual que Runpod pero: crea instancia con plantilla docker
`REGISTRY/brainmaster-worker:latest`, on-start script =
`docker compose up -d` del repo clonado, y usa el *proxy* de Vast para los
puertos. Suele ser un 30-40 % más barato que Runpod para 3090/4090.

## 5 · EC2 / Lambda Labs / GCP (VM con user-data)

**Artefacto**: `deploy/ec2/user-data.sh`

1. Lanza AMI "Deep Learning Ubuntu 22.04" (g5.xlarge, o A10G de Lambda).
2. Pega `user-data.sh` como User data (ajusta `BM_REPO_URL`).
3. En ~4 min el stack entero está arriba (docker, toolkit NVIDIA, compose).
4. Security group: 8080 + 4200 solo desde tu IP.

Coste spot: una g5.xlarge spot ronda $0.30-0.45/h. Para probar y apagar es
lo más cómodo en nube clásica.

## 6 · Fly.io (un comando)

**Artefacto**: `deploy/fly/fly.toml`

```bash
cd deploy/fly
fly launch --no-deploy --name brainmaster-worker --copy-config
fly secrets set BM_ARTIFACT_TOKEN=$(openssl rand -hex 16)
fly deploy
```

El gateway conecta con `PYTHON_WORKER_HOST=tls://brainmaster-worker.fly.dev:50051`
(Fly termina TLS por nosotros; el cliente Go ya soporta `tls://`).
GPU a10g ≈ $2.5/h — úsalo solo cuando necesites potencia y apaga la máquina
después (`fly machine stop`).

## 7 · Oracle Cloud Always Free (24/7 gratis, CPU)

**Artefactos**: `deploy/cpu/compose.cpu.yml` + `deploy/cpu/Dockerfile.cpu`

1. Crea cuenta Oracle Free → instancia **Ampere A1** (4 OCPU / 24 GB RAM).
2. Docker + `git clone` del repo.
3. ```bash
   docker compose -f docker-compose.yml -f deploy/cpu/compose.cpu.yml up -d
   ```
4. Abre 8080/4200 en la consola de seguridad.

Corre 24/7 para siempre con $0. Viable: catálogo dinámico, jobs con
SD-Tiny-Test (~30 s/imagen), TTS pequeños. No viable: video ni SDXL.
Si el tráfico lo pide, este mismo box hace de gateway para el worker GPU
remoto (opción 1-6) combinando lo mejor de ambos mundos.

## 8 · Estación local (tu GPU)

```bash
docker compose up -d --build
```

Requisitos: NVIDIA driver + [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html).
En Windows/WSL2 funciona igual. Sin Docker, el modo dev nativo sigue siendo:
`py -3 server.py` (worker) + `./gateway.exe` + `ng serve`.

---

## Matriz de modelos por plataforma

| Familia | VRAM | Colab T4 | Kaggle T4 | 4090 24 GB | A10G 24 GB | CPU |
|---|---|---|---|---|---|---|
| SD-Tiny-Test | 2 GB | ✅ | ✅ | ✅ | ✅ | ✅ (~30 s) |
| SD-1.5 | 4 GB | ✅ | ✅ | ✅ | ✅ | ⚠️ min |
| SDXL-Turbo/Base | 8-12 GB | ✅ | ✅ | ✅ | ✅ | ❌ |
| Qwen-Image / Flux-schnell | 16-24 GB | ⚠️ justo | ⚠️ justo | ✅ | ✅ | ❌ |
| Flux-dev | 24 GB | ❌ | ❌ | ✅ | ⚠️ justo | ❌ |
| LTX-Video / Wan-1.3B | 12-16 GB | ✅ | ✅ | ✅ | ✅ | ❌ |
| Wan-14B / Hunyuan / CogVideoX-5B | 24-48 GB | ❌ | ❌ | ⚠️ offload | ⚠️ offload | ❌ |
| Chatterbox TTS / ACE-Step | 6-12 GB | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| Familias bridge V1 (LTX-2.5, H3, K5…) | 24-80 GB | ❌ | ❌ | ⚠️ | ⚠️ | ❌ |

El worker consulta `nvidia-smi`/CUDA en runtime y `ListModels` refleja el
dispositivo real; el front oculta lo no disponible.

---

## Checklist de operación

- [ ] `curl http://<gateway>/api/v1/health` → `worker_connected: true`
- [ ] `curl http://<gateway>/api/v1/models` → catálogo con `available` correcto
- [ ] Crear un job de prueba y confirmar `artifact_url` descargable
- [ ] Si el worker es efímero (Colab/Kaggle): script/cron que refresque las
      variables cuando cambie el túnel
- [ ] Volumen persistente para `hf-cache` (los pesos NO se re-descargan)
- [ ] `BM_ARTIFACT_TOKEN` en worker + proxy que lo reenvíe si el artifact
      server queda expuesto a internet

## Próximos pasos naturales

1. **CI/CD** (GitHub Actions): tests → build-push automático → redeploy del pod.
2. **Cola persistente remota**: el worker efímero ya sobrevive a reinicios
   por la restauración de SQLite del gateway; evaluar pull-queue si se
   escalan a varios workers.
3. **Autoscaling Runpod** con webhook de "primer job" para arrancar el pod.
