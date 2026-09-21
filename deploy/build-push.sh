#!/usr/bin/env bash
# =============================================================================
# Build + push de las 3 imágenes a un registry (Docker Hub / GHCR)
#
#   ./deploy/build-push.sh [tag]
#
# Requiere: docker login previo. Variables:
#   REGISTRY  (obligatorio; ej: docker.io/tuusuario o ghcr.io/usuario)
#   PLATFORM  (default: linux/amd64 — añade,arm64 para Oracle A1 con la
#              imagen CPU; buildx lo maneja solo)
# =============================================================================
set -euo pipefail

TAG="${1:-latest}"
REGISTRY="${REGISTRY:-}"
PLATFORM="${PLATFORM:-linux/amd64}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Falla temprano y claro en vez de pushear a un registry basura:
if [[ -z "$REGISTRY" || "$REGISTRY" == *TU_USUARIO* ]]; then
  echo "✗ REGISTRY no configurado. Uso:" >&2
  echo "    REGISTRY=docker.io/tuusuario ./deploy/build-push.sh [tag]" >&2
  echo "  (o ghcr.io/usuario — requiere docker login previo)" >&2
  exit 1
fi

echo "» Registry: $REGISTRY  tag: $TAG  platform: $PLATFORM"

build_push() {
  local name="$1" ctx="$2" dockerfile="$3"
  echo "── $name"
  docker buildx build --platform "$PLATFORM" \
    -f "$ROOT/$dockerfile" -t "$REGISTRY/$name:$TAG" \
    --push "$ROOT/$ctx"
}

build_push "brainmaster-gateway"  "backend"           "backend/Dockerfile"
build_push "brainmaster-frontend" "frontend"          "frontend/Dockerfile"
build_push "brainmaster-worker"   "inference-worker"  "inference-worker/Dockerfile"

# Variante CPU del worker (para Oracle A1 / VPS ARM)
docker buildx build --platform "${PLATFORM},linux/arm64" \
  -f "$ROOT/deploy/cpu/Dockerfile.cpu" -t "$REGISTRY/brainmaster-worker:cpu-$TAG" \
  --push "$ROOT/inference-worker"

echo "✔ Imágenes publicadas en $REGISTRY"
docker images | grep brainmaster
