# =============================================================================
# Brain-Master Worker en Kaggle (GPU gratis: T4 x2 / P100, 30 h/semana)
#
# Alternativa cuando Colab agota el límite de GPU de tu cuenta.
#
# Uso:
#   1. kaggle.com -> New Notebook -> Settings -> Accelerator: GPU T4 x2
#   2. Settings -> Internet: ON
#   3. File -> Upload -> sube este archivo y en una celda ejecuta:
#        %run brain_master_worker.py
#   4. El script clona el repo público, instala deps, levanta el worker y
#      publica los túneles en el topic ntfy FIJO: tu watch-announce.py local
#      (deploy/colab/watch-announce.py) reconecta el gateway solo. Cero
#      copy/paste: basta con dejar esta celda viva.
#
# Diferencias con Colab: sesión máx 12 h (9 h de GPU); /kaggle/working se
# guarda como output del notebook al terminar.
# =============================================================================

import json
import os
import re
import subprocess
import sys
import time
from urllib.request import Request, urlopen

WORKER_DIR = "/kaggle/working/brain-master/inference-worker"
REPO_URL = "https://github.com/synappta4ai/brain-master.git"
NTFY_TOPIC = "bm-brain-master-tunnels-v1"   # FIJO: lo consume watch-announce.py
OUT = "/kaggle/working"


def sh(cmd):
    subprocess.run(cmd, shell=isinstance(cmd, str), check=True)


# ---------------------------------------------------------------- 1. código
if not os.path.isdir(WORKER_DIR):
    sh(["git", "clone", "--depth", "1", REPO_URL, "/kaggle/working/brain-master"])
os.chdir(WORKER_DIR)

# ---------------------------------------------------------- 2. dependencias
sh([sys.executable, "-m", "pip", "install", "-q",
    "grpcio==1.84.0", "grpcio-tools==1.84.0", "protobuf>=5.29.3",
    "diffusers>=0.36.0", "transformers>=4.57.0", "tokenizers",
    "accelerate", "safetensors", "sentencepiece", "einops",
    "open_clip_torch", "imageio", "imageio-ffmpeg", "Pillow",
    "psutil"])

# ------------------------------------------------- 3. binarios de túneles
# gRPC va por bore (TCP puro; cloudflared solo hace HTTP y rompe streams
# gRPC). Artefactos por cloudflared (HTTP).
if not os.path.exists("/usr/local/bin/cloudflared"):
    sh(["curl", "-sL", "-o", "/usr/local/bin/cloudflared",
        "https://github.com/cloudflare/cloudflared/releases/"
        "latest/download/cloudflared-linux-amd64"])
    sh(["chmod", "+x", "/usr/local/bin/cloudflared"])
if not os.path.exists("/usr/local/bin/bore"):
    sh(["curl", "-sL", "-o", "/tmp/bore.tar.gz",
        "https://github.com/ekzhang/bore/releases/download/v0.6.0/"
        "bore-v0.6.0-x86_64-unknown-linux-musl.tar.gz"])
    sh(["tar", "-xzf", "/tmp/bore.tar.gz", "-C", "/usr/local/bin"])
    sh(["chmod", "+x", "/usr/local/bin/bore"])


def wait_for(path, pattern, tries=40):
    for _ in range(tries):
        time.sleep(1)
        m = re.search(pattern, open(path).read())
        if m:
            return m
    raise RuntimeError(f"patrón no apareció en {path}:\n" + open(path).read())


def announce(host, base):
    body = json.dumps({"PYTHON_WORKER_HOST": host,
                       "BM_WORKER_ARTIFACT_BASE": base}).encode()
    req = Request(f"https://ntfy.sh/{NTFY_TOPIC}", data=body,
                  headers={"Title": "brain-master worker UP (Kaggle)",
                           "Priority": "high"})
    try:
        urlopen(req, timeout=15).read()
        print(f"anunciado en ntfy.sh/{NTFY_TOPIC} -> watch-announce reconecta solo")
    except Exception as e:  # el announce no debe matar la sesión
        print(f"AVISO: announce falló ({e}); conecta manual con connect-gateway.ps1")


def start_all():
    """Levanta worker + túneles, anuncia y devuelve (procs, host, base)."""
    sh("pkill -f 'python server.py' 2>/dev/null; "
       "pkill -f 'bore local' 2>/dev/null; "
       "pkill -f cloudflared 2>/dev/null; true")
    time.sleep(2)
    os.environ["BM_OUTPUT_DIR"] = f"{OUT}/outputs"
    os.makedirs(f"{OUT}/outputs", exist_ok=True)

    worker = subprocess.Popen([sys.executable, "server.py"],
                              stdout=open(f"{OUT}/worker.log", "a"),
                              stderr=subprocess.STDOUT)
    time.sleep(6)

    bore = subprocess.Popen(["bore", "local", "50051", "--to", "bore.pub"],
                            stdout=open(f"{OUT}/bore.log", "w"),
                            stderr=subprocess.STDOUT)
    bore_port = wait_for(f"{OUT}/bore.log", r"listening at bore\.pub:(\d+)").group(1)

    cf = subprocess.Popen(["cloudflared", "tunnel", "--url",
                           "http://127.0.0.1:50052", "--no-autoupdate"],
                          stdout=open(f"{OUT}/cf.log", "w"),
                          stderr=subprocess.STDOUT)
    art_url = wait_for(f"{OUT}/cf.log",
                       r"https://[a-z0-9-]+\.trycloudflare\.com").group(0)

    host = "bore.pub:" + bore_port
    announce(host, art_url)
    return worker, bore, cf, host, art_url


worker, bore, cf, host, art_url = start_all()
print("=" * 62)
print("PYTHON_WORKER_HOST=" + host)
print("BM_WORKER_ARTIFACT_BASE=" + art_url)
print(f"ntfy.topic = {NTFY_TOPIC}")
print("=" * 62)

# Mantén la celda viva (Kaggle corta si no hay actividad) y relanza lo que
# muera: worker o cualquiera de los túneles.
try:
    while True:
        time.sleep(30)
        if worker.poll() is not None or bore.poll() is not None or cf.poll() is not None:
            print("algo murió; relanzando worker + túneles...")
            worker, bore, cf, host, art_url = start_all()
            print("re-anunciado:", host)
except KeyboardInterrupt:
    print("deteniendo...")
    for p in (worker, bore, cf):
        p.terminate()
