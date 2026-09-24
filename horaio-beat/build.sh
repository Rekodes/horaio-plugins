#!/usr/bin/env bash
# Compila horaio-beat para las cinco plataformas en dist/.
#
#   build.sh [versión]      por defecto, "dev"
#
# Binarios estáticos (CGO_ENABLED=0) y reproducibles (-trimpath), sin tabla de
# símbolos (-s -w). Lo usan `make beat-build` y el CI.
set -euo pipefail
cd "$(dirname "$0")"

version="${1:-dev}"
targets=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64)

rm -rf dist && mkdir -p dist
for target in "${targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  ext=""
  [[ "$os" == windows ]] && ext=".exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X main.version=${version}" \
    -o "dist/horaio-beat-${os}-${arch}${ext}" .
done

(cd dist && shasum -a 256 horaio-beat-* > SHA256SUMS 2>/dev/null || sha256sum horaio-beat-* > SHA256SUMS)
