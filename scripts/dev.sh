#!/usr/bin/env bash
# Jalankan backend (port 8443) + frontend dev server (port 3000).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

trap 'kill 0' EXIT

(
  cd "$ROOT/backend"
  HBMPANEL_HOME="$ROOT/.dev-data" HBMPANEL_LOG="$ROOT/.dev-data/log" \
    go run ./cmd/hbmpanel init --home "$ROOT/.dev-data"
  HBMPANEL_HOME="$ROOT/.dev-data" HBMPANEL_LOG="$ROOT/.dev-data/log" \
    go run ./cmd/hbmpanel serve --port 8443
) &

(
  cd "$ROOT/frontend"
  if [ ! -d node_modules ]; then
    if command -v pnpm >/dev/null; then pnpm install; else npm install; fi
  fi
  if command -v pnpm >/dev/null; then pnpm dev; else npm run dev; fi
) &

wait
