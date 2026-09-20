#!/bin/bash
# =============================================================================
# cloud-init / user-data para VM GPU (Ubuntu 22.04 + NVIDIA driver preinstalado)
#
# Válido para:
#   - AWS EC2 g5.xlarge con "Deep Learning AMI GPU Ubuntu 22.04" (drivers OK)
#   - Lambda Labs GPU (Ubuntu + drivers ya instalados)
#   - GCP/Azure con imagen GPU base
#
# En AWS: pega este archivo en "User data" al lanzar la instancia.
# El stack queda arriba en ~4 minutos. Abre en el security group:
#   8080/tcp (gateway REST+WS)  y  4200/tcp (front)  — solo tu IP.
# =============================================================================
set -euxo pipefail

# 1. Docker + compose (la DLAMI suele traerlos; por si acaso)
if ! command -v docker >/dev/null; then
  curl -fsSL https://get.docker.com | sh
fi
docker compose version || apt-get update && apt-get install -y docker-compose-plugin

# 2. NVIDIA Container Toolkit (necesario para --gpus all)
if ! command -v nvidia-smi >/dev/null; then
  echo "ADVERTENCIA: nvidia-smi no presente; instala el driver NVIDIA primero" >&2
fi
if ! docker info 2>/dev/null | grep -qi nvidia; then
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
    | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -fsSL https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list \
    | tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
  apt-get update && apt-get install -y nvidia-container-toolkit
  nvidia-ctk runtime configure --runtime=docker
  systemctl restart docker
fi

# 3. Código y stack
mkdir -p /opt/brain-master && cd /opt/brain-master
if [ ! -d brain-master ]; then
  git clone --depth 1 "${BM_REPO_URL:-https://github.com/TU_USUARIO/brain-master.git}" brain-master
fi
cd brain-master

# 4. Build + up (GPU)
docker compose build
docker compose up -d

# 5. Resumen de conexión
sleep 20
echo "=============================================================="
echo "Brain-Master desplegado en VM GPU:"
curl -s http://127.0.0.1:8080/api/v1/health || true
echo
curl -s http://127.0.0.1:8080/api/v1/models | head -c 400 || true
echo
echo "Gateway REST+WS: http://$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || echo <IP>):8080"
echo "Front:           http://<IP>:4200"
echo "=============================================================="
