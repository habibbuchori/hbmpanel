# HBMPanel

Lightweight modern app panel untuk Laravel, Next.js, Node.js, Go, dan static apps.
**One-shot installer** untuk Debian 12 / Ubuntu 24.04 / LXC fresh.

> Spesifikasi penuh ada di [`hbmpanel-prd.md`](./hbmpanel-prd.md).

## Struktur Repo

```
hbmpanel/
├── installer/install.sh           # one-shot bootstrap installer (Bash)
├── backend/                       # Go (Fiber) + SQLite + embed.FS
│   ├── cmd/hbmpanel/main.go       # CLI entrypoint (serve / init / version)
│   └── internal/
│       ├── api/                   # Fiber router + static mount
│       ├── auth/                  # JWT + bcrypt
│       ├── config/                # struct config
│       ├── db/                    # SQLite store + migrations
│       ├── modules/
│       │   ├── caddy/             # Caddyfile generator + reload
│       │   ├── pm2/               # PM2 wrapper
│       │   ├── postgres/          # psql wrapper
│       │   ├── redis/             # redis-cli wrapper
│       │   ├── site/              # CRUD site + Caddy integration
│       │   ├── sftp/              # placeholder
│       │   ├── supervisor/        # placeholder
│       │   └── system/            # stats + systemctl
│       ├── web/                   # embed.FS untuk frontend static export
│       └── ws/                    # WebSocket log streamer
├── frontend/                      # Next.js 15 + Tailwind + shadcn-style
│   └── app/
│       ├── page.tsx               # /        — login
│       └── dashboard/
│           ├── page.tsx           # /dashboard           — overview
│           ├── sites/page.tsx     # /dashboard/sites     — CRUD site
│           ├── services/page.tsx  # /dashboard/services  — systemctl
│           ├── database/page.tsx  # /dashboard/database  — postgres
│           ├── logs/page.tsx      # /dashboard/logs      — WS tail
│           └── settings/page.tsx
├── packaging/systemd/hbmpanel.service
├── scripts/
│   ├── build.sh                   # build full release binary
│   └── dev.sh                     # jalankan backend + frontend dev
└── hbmpanel-prd.md
```

## Stack

| Layer        | Tech                                            |
|--------------|-------------------------------------------------|
| Backend      | Go 1.23 + Fiber + JWT + bcrypt                  |
| Panel DB     | SQLite (modernc.org/sqlite, pure-Go)            |
| Frontend     | Next.js 15 (static export) + Tailwind + shadcn  |
| Realtime     | WebSocket (gorilla via gofiber/contrib)         |
| Distribusi   | Single binary (~30-50MB) — frontend di-embed    |

## Quick Start (development)

Prasyarat: Go 1.23+, Node.js 20+, (rekomendasi) pnpm.

```bash
# Jalankan keduanya (backend :8443, frontend :3000)
bash scripts/dev.sh

# Buka http://localhost:3000
# Login: admin / <password yang dicetak ke stdout oleh `init`>
```

## Production Build

```bash
# Build single-binary Linux untuk amd64 + arm64
bash scripts/build.sh

# Output:
#   dist/hbmpanel-linux-amd64
#   dist/hbmpanel-linux-arm64
```

## Production Install (target VPS/LXC)

```bash
# Di server Debian 12 / Ubuntu 24.04 fresh
bash <(curl -s https://install-panel.hbm.my.id)

# Non-interactive
HBMPANEL_DOMAIN=panel.example.com \
HBMPANEL_EMAIL=admin@example.com \
bash <(curl -s https://install-panel.hbm.my.id)
```

Yang dilakukan installer:
1. Pre-flight (OS, arch, port, RAM)
2. apt update + timezone + UFW + swap (bila <2GB RAM)
3. Install: Caddy · PHP 8.4 · PostgreSQL 16 · Redis · Node 22 · PM2 · Supervisor · FileBrowser · OpenSSH chroot
4. Download binary `hbmpanel` dari GitHub Releases
5. Register systemd unit `hbmpanel.service`
6. Caddy reverse proxy untuk panel (auto-SSL bila domain di-set)

Installer **idempotent** — aman dijalankan ulang.

## MVP Status

| Modul                | Status            |
|----------------------|-------------------|
| Auth (login/JWT)     | scaffolded ✓      |
| System stats         | scaffolded ✓      |
| Service control      | scaffolded ✓      |
| Site CRUD + Caddy    | scaffolded ✓      |
| PM2                  | scaffolded ✓      |
| PostgreSQL           | scaffolded ✓      |
| Redis                | scaffolded ✓      |
| Log streaming (WS)   | scaffolded ✓      |
| SFTP                 | placeholder       |
| Supervisor           | placeholder       |
| File Manager         | bridge ke FileBrowser (MVP v0.2) |
| Web Terminal         | MVP v0.3          |
| Backup               | MVP v0.3          |
| Git deployment       | MVP v0.3          |

## License

TBD — AGPL atau Apache 2.0 (lihat PRD §16).
