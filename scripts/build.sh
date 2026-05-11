#!/usr/bin/env bash
# Build full hbmpanel binary: frontend → embed → go build.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "==> 1/3 build frontend (Next.js static export)"
cd "$ROOT/frontend"
if [ ! -d node_modules ]; then
  if command -v pnpm >/dev/null; then pnpm install; else npm install; fi
fi
if command -v pnpm >/dev/null; then pnpm build; else npm run build; fi

echo "==> 2/3 copy out/ → backend/internal/web/dist/"
rm -rf "$ROOT/backend/internal/web/dist"
mkdir -p "$ROOT/backend/internal/web/dist"
cp -r "$ROOT/frontend/out/." "$ROOT/backend/internal/web/dist/"

echo "==> 3/3 go build"
cd "$ROOT/backend"
mkdir -p "$ROOT/dist"
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
  -o "$ROOT/dist/hbmpanel-linux-amd64" ./cmd/hbmpanel
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" \
  -o "$ROOT/dist/hbmpanel-linux-arm64" ./cmd/hbmpanel

echo
echo "Built:"
ls -lh "$ROOT/dist/"
