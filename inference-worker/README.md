# 🧠 Brain-Master: Worker de Inferencia IA (Python + CUDA)

Este microservicio constituye la **Capa 3** de la arquitectura híbrida de **Brain-Master**. Es un servicio _headless_ (sin interfaz web acoplada) responsable de ejecutar los modelos de difusión de video, generación de imágenes, transcripción de audio y orquestación de LLMs en hardware GPU.

---

## 🎯 ¿Qué hace este servicio?

- **Generación de Video:** Ejecuta pipelines de difusión como **Wan 2.1 (T2V/I2V 14B)**, **MiniMax H3**, **LTX-Video 2.5** y **Hunyuan**.
- **Generación de Imagen:** Pipeline de **Flux 2 Klein 9B**, Krea y Qwen Image.
- **Audio y Voz:** Transcripción/diarización con **Whisper**, generación musical con **YuE2** y clonación de voz con **Seed-VC**.
- **Gestión de VRAM ([`vram_manager.py`](brain-master/inference-worker/vram_manager.py)):** Telemetría continua de memoria de GPU y liberación proactiva con `torch.cuda.empty_cache()` para prevenir errores de _Out of Memory (OOM)_.

---

## 🖥️ Requisitos de Hardware

- **GPU:** NVIDIA con arquitectura Ampere, Ada Lovelace o Blackwell (RTX 3060 12GB mínimo; **RTX 3090 / 4090 de 24 GB VRAM recomendada** para video 1080p y modelos 14B).
- **Drivers:** NVIDIA Driver 535+ (Linux) o 550+ (Windows) con soporte CUDA 12.1+ o CUDA 13.
- **Almacenamiento:** Disco SSD NVMe (los pesos de modelos de video rondan entre 10 GB y 40 GB).

---

## 🚀 ¿Dónde se PUEDE y DEBERÍA desplegar?

El worker de inferencia requiere hardware especializado con aceleración CUDA. A continuación se presentan las mejores alternativas recomendadas según el escenario de uso:

### 1. 🏆 Serverless GPU (La opción más recomendada para SaaS y Startups)

> **¿Dónde?** [Modal.com](https://modal.com), [RunPod Serverless](https://www.runpod.io/serverless-gpu), [Baseten](https://www.baseten.co) o [Beam.cloud](https://beam.cloud).

- **¿Por qué deberías usarlo?**
  - **Costo $0 cuando no hay usuarios activos:** La GPU se apaga automáticamente cuando la cola está vacía.
  - **Escalado automático:** Si 10 usuarios generan videos al mismo tiempo, la plataforma levanta 10 GPUs en paralelo.
  - **Sin mantenimiento:** Cero administración de drivers de CUDA, parches de sistema operativo o reinicios de máquinas.
- **Esquema de Conexión:**
  ```
  Angular (Vercel/Web) ➔ Go Gateway (VPS Hetzner $5/mes) ➔ Modal/RunPod Serverless (Pagas solo los segundos de GPU usados)
  ```

---

### 2. ⚡ Instancia Cloud GPU Dedicada (Recomendada para tráfico continuo y baja latencia)

> **¿Dónde?** [RunPod Dedicated](https://www.runpod.io), [Vast.ai](https://vast.ai), [Lambda Labs](https://lambdalabs.com) o [TensorDock](https://tensordock.com).

- **¿Por qué deberías usarlo?**
  - **Modelos siempre en VRAM:** Los pesos están precargados en memoria, ofreciendo **cero tiempo de inicio (_zero cold-start_)**.
  - **Precios económicos:** Una RTX 4090 de 24 GB cuesta ~$0.25 a $0.35 / hora (~$180 a $250 / mes continuo).
- **Cómo desplegarlo:**
  - Desplegar el worker mediante Docker con soporte para GPU:
    ```bash
    docker run -d --gpus all -p 50051:50051 -v /models:/app/models brain-master-worker:latest
    ```
  - Conectar de forma segura con el Gateway de Go utilizando una VPN privada como **Tailscale** o **WireGuard**.

---

### 3. 🏠 Servidor Local On-Premise / Edge (Recomendado para testing, estudio y privacidad total)

> **¿Dónde?** En tu propia Workstation o servidor local con GPU NVIDIA (RTX 3090 / 4080 / 4090).

- **¿Por qué deberías usarlo?**
  - **Costo recurrente $0** en infraestructura de nube.
  - **Privacidad absoluta:** Ningún video, audio o prompt sale de tu red local.
  - Ideal para creadores de contenido, productoras audiovisuales o entornos de desarrollo.
- **Acceso remoto seguro:** Puedes exponer el worker hacia un backend en la nube utilizando **Tailscale** o **Cloudflare Tunnels** sin abrir puertos en el router.

---

### 4. 🏢 Clúster Kubernetes + KEDA / Ray (Recomendado para Grandes Empresas)

> **¿Dónde?** Clúster privado de Kubernetes con **NVIDIA GPU Operator** y auto-escalado basado en eventos (**KEDA**).

- **¿Por qué deberías usarlo?**
  - Para plataformas con miles de usuarios concurrentes y políticas empresariales estrictas de SLA y almacenamiento compartido (NFS/Ceph).

---

### ❌ ¿Dónde NO se recomienda desplegar?

- **Hyperscalers tradicionales sin descuento (AWS EC2 g5, GCP A3, Azure NV):** Tienen costos de $1.50 a $4.50+ por hora (hasta **10 veces más caros** que RunPod, Vast.ai o Modal para la misma GPU). Solo se justifican si tu empresa cuenta con créditos promocionales o normativas de compliance bancario.

---

## 🛠️ Ejecución Local y Desarrollo

### 1. Instalación de dependencias

```powershell
cd brain-master/inference-worker
uv pip install -r requirements.txt
```

### 2. Iniciar el worker

```powershell
python server.py
```

### 3. Salida esperada en consola:

```text
2026-09-19 00:00:00 [INFO] [InferenceEngine] Inicializado motor de IA.
🧠 [Brain-Master] Worker de Inferencia Python listo.
Telemetría: {'device_name': 'NVIDIA GeForce RTX 4090', 'total_vram_mb': 24564, 'free_vram_mb': 23100, ...}
```

---

## 🐳 Dockerfile de Producción (Ejemplo)

```dockerfile
FROM nvidia/cuda:12.4.1-runtime-ubuntu22.04

WORKDIR /app

# Instalar Python y dependencias de sistema
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.11 \
    python3-pip \
    ffmpeg \
    git \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 50051

CMD ["python3", "server.py"]
```
