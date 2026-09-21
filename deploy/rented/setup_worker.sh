#!/usr/bin/env bash
# =============================================================================
# setup_worker.sh — arranca el Brain-Master worker en una VM alquilada
# (Runpod / Vast.ai / cualquier Ubuntu con GPU NVIDIA y driver funcionando)
#
# NO necesita túneles: la VM tiene IP pública y el worker escucha en [::].
# El gateway se conecta directo. Uso dentro de la VM (consola web o SSH):
#
#   curl -sL https://raw.githubusercontent.com/synappta4ai/brain-master/main/deploy/rented/setup_worker.sh | bash
#
# Al final imprime el comando exacto para el gateway y abre un log en
# /workspace/brain-master/worker.log. Re-ejecutar es idempotente: mata lo
# anterior y rearranca (útil tras un reinicio de la VM).
# =============================================================================
set -euo pipefail

BASE=/workspace/brain-master
[ -d /workspace ] || BASE=~/brain-master      # Vast usa /workspace; otros, $HOME
LOG="$BASE/worker.log"

echo "» Clonando/actualizando el repo en $BASE ..."
if [ -d "$BASE" ]; then
  git -C "$BASE" pull --ff-only || true
else
  git clone --depth 1 https://github.com/synappta4ai/brain-master.git "$BASE"
fi
cd "$BASE/inference-worker"

echo "» Dependencias (torch CUDA si la VM no lo trae) ..."
python -c "import torch" 2>/dev/null || \
  pip install -q torch --index-url https://download.pytorch.org/whl/cu124
pip install -q grpcio==1.84.0 grpcio-tools==1.84.0 "protobuf>=5.29.3" \
  diffusers>=0.36.0 transformers>=4.57.0 tokenizers accelerate safetensors \
  sentencepiece einops open_clip_torch imageio imageio-ffmpeg Pillow psutil

echo "» GPU detectada:"
nvidia-smi --query-gpu=name,memory.total --format=csv,noheader || echo "  ¡ojo! nvidia-smi no responde"

echo "» Arrancando worker + artifact server ..."
pkill -f '[p]ython server.py' 2>/dev/null || true   # [x] trick anti auto-kill
sleep 1
export BM_OUTPUT_DIR="$BASE/outputs" BM_ARTIFACT_PORT=50052
mkdir -p "$BM_OUTPUT_DIR"
nohup python server.py > "$LOG" 2>&1 &
sleep 8
tail -5 "$LOG"

PUBLIC_IP=$(curl -s https://ifconfig.me || hostname -I | awk '{print $1}')

cat <<EOF

============================================================
  WORKER ARRIBA. En tu máquina (donde corre el gateway):

  .\\deploy\\colab\\connect-gateway.ps1 -WorkerHost "$PUBLIC_IP:50051" \\
      -ArtifactBase "http://$PUBLIC_IP:50052"

  (sin túneles: conexión directa por IP; el artifact server
   escucha HTTP en :50052 y sirve $BM_OUTPUT_DIR)

  Logs del worker: tail -f $LOG
============================================================
EOF
