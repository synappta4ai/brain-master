# =============================================================================
# Brain-Master Worker en Kaggle (GPU gratis: T4 x2 / P100, 30 h/semana)
#
# Uso:
#   1. kaggle.com → New Notebook → Settings → Accelerator: GPU T4
#   2. Settings → Internet: ON
#   3. Sube el código: File → Upload → carpetas `inference-worker` y `outputs`,
#      o clona el repo con la celda de abajo.
#   4. Ejecuta:  %run brain_master_worker.py
#   5. Copia las dos líneas PYTHON_WORKER_HOST / BM_WORKER_ARTIFACT_BASE que
#      imprime y configúralas en el gateway.
#
# Diferencias con Colab: Kaggle corta la sesión a las 12 h (9 h de GPU por
# notebook) y la carpeta /kaggle/working se persiste como output.
# =============================================================================

import os
import re
import subprocess
import sys
import time

WORKER_DIR = "/kaggle/working/brain-master/inference-worker"
REPO_URL = "https://github.com/TU_USUARIO/brain-master.git"  # ← AJUSTA

# ---------------------------------------------------------------- 1. código
if not os.path.isdir(WORKER_DIR):
    subprocess.run(["git", "clone", "--depth", "1", REPO_URL,
                    "/kaggle/working/brain-master"], check=True)
os.chdir(WORKER_DIR)

# ---------------------------------------------------------- 2. dependencias
subprocess.run([sys.executable, "-m", "pip", "install", "-q",
                "grpcio==1.84.0", "grpcio-tools==1.84.0", "protobuf>=5.29.3",
                "diffusers>=0.36.0", "transformers>=4.57.0", "tokenizers",
                "accelerate", "safetensors", "sentencepiece", "einops",
                "open_clip_torch", "imageio", "imageio-ffmpeg", "Pillow",
                "psutil"], check=True)

# ------------------------------------------------------------ 3. cloudflared
if not os.path.exists("/usr/local/bin/cloudflared"):
    subprocess.run(["curl", "-sL", "-o", "/usr/local/bin/cloudflared",
                    "https://github.com/cloudflare/cloudflared/releases/"
                    "latest/download/cloudflared-linux-amd64"], check=True)
    subprocess.run(["chmod", "+x", "/usr/local/bin/cloudflared"], check=True)

# --------------------------------------------------------- 4. worker + túneles
os.environ["BM_OUTPUT_DIR"] = "/kaggle/working/outputs"
os.makedirs("/kaggle/working/outputs", exist_ok=True)

worker = subprocess.Popen([sys.executable, "server.py"],
                          stdout=open("/kaggle/working/worker.log", "a"),
                          stderr=subprocess.STDOUT)
time.sleep(6)


def tunnel(port: int) -> str:
    log = open(f"/kaggle/working/tunnel_{port}.log", "w")
    subprocess.Popen(["cloudflared", "tunnel", "--url", f"http://127.0.0.1:{port}",
                      "--no-autoupdate"], stdout=log, stderr=log)
    for _ in range(30):
        time.sleep(1)
        m = re.search(r"https://[a-z0-9-]+\.trycloudflare\.com",
                      open(f"/kaggle/working/tunnel_{port}.log").read())
        if m:
            return m.group(0)
    raise RuntimeError(f"túnel {port} no levantó")


grpc_url = tunnel(50051)
art_url = tunnel(50052)

print("=" * 62)
print("PYTHON_WORKER_HOST=" + grpc_url.replace("https://", "") + ":443")
print("BM_WORKER_ARTIFACT_BASE=" + art_url)
print("=" * 62)

# Mantén la celda viva (Kaggle corta si no hay actividad):
try:
    while True:
        if worker.poll() is not None:
            print("worker murió; relanzando...")
            worker = subprocess.Popen([sys.executable, "server.py"],
                                      stdout=subprocess.STDOUT,
                                      stderr=subprocess.STDOUT)
        time.sleep(30)
except KeyboardInterrupt:
    worker.terminate()
