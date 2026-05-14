# HBMPanel

[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black.svg)](https://nextjs.org/)

Lightweight modern app panel untuk Laravel, Next.js, Node.js, Go, dan static apps.
**One-shot installer** untuk Debian 12 / Ubuntu 24.04 / LXC fresh.

> Spesifikasi penuh ada di [`hbmpanel-prd.md`](./hbmpanel-prd.md).

## Stack

| Layer        | Tech                                            |
|--------------|-------------------------------------------------|
| Backend      | Go 1.23 + Fiber + JWT + bcrypt                  |
| Panel DB     | SQLite (modernc.org/sqlite, pure-Go)            |
| Frontend     | Next.js 15 (static export) + Tailwind           |
| Realtime     | WebSocket (gofiber/contrib)                     |
| Distribusi   | Single binary (~30–50 MB) — frontend di-embed   |

## Features

### Core (free)
| Modul                         | Status |
|-------------------------------|--------|
| Auth (login/JWT) + **2FA TOTP** + change password + CLI reset | ✅ |
| Dashboard overview (CPU/RAM/disk/sites/PM2/Redis/Postgres)    | ✅ |
| Site CRUD + actions (restart/enable/disable/env vars/logs)    | ✅ |
| Caddy integration (auto-Caddyfile + per-site log)             | ✅ |
| Laravel manager (artisan + presets: migrate/cache-clear/optimize) | ✅ |
| PM2 wrapper (start/stop/restart/list/logs)                    | ✅ |
| PostgreSQL (db & role create/delete)                          | ✅ |
| Redis (restart/flush/info)                                    | ✅ |
| Log viewer (live tail WS + search + download per-source)      | ✅ |
| Web terminal (PTY via WebSocket + xterm.js)                   | ✅ |
| SSH key manager (authorized_keys per system user)             | ✅ |
| Audit log (auto-recorded mutations)                           | ✅ |
| IP whitelist + rate-limited login + CSRF guard + fail2ban log | ✅ |

### Premium (license-gated)
| Modul                          | Status |
|--------------------------------|--------|
| SFTP user manager (chroot)     | ✅ |
| File manager (browse/upload/edit/extract) | ✅ |
| Backup (site tar.gz + pg_dump.gz, list/restore/download) | ✅ |
| Git deploy (clone/pull/run deploy command) | ✅ |

### Roadmap
- Scheduled backups (cron-like)
- S3 backup storage
- Built-in SQL editor
- GitHub/GitLab webhook auto-deploy
- Multi-server management

## Quick Start (development)

Prasyarat: Go 1.23+, Node.js 20+, npm.

```bash
# Jalankan keduanya (backend :8443, frontend :3000)
bash scripts/dev.sh

# Buka http://localhost:3000
# Login: admin / <password yang dicetak ke stdout oleh `hbmpanel init`>
```

CLI:
```bash
hbmpanel serve [--port 8443]              # jalankan server
hbmpanel init  [--home /var/lib/hbmpanel] # init data dir + admin awal
hbmpanel reset-password [--user admin]    # reset password (print baru ke stdout)
hbmpanel version
```

## Production Build

```bash
# Cross-compile linux amd64 + arm64
bash scripts/build.sh

# Output:
#   dist/hbmpanel-linux-amd64
#   dist/hbmpanel-linux-arm64
```

## Production Install (target VPS/LXC)

```bash
# Debian 12 / Ubuntu 24.04 fresh
bash <(curl -s https://install-panel.hbm.my.id/install.sh)

# Non-interactive
HBMPANEL_DOMAIN=panel.example.com \
HBMPANEL_EMAIL=admin@example.com \
bash <(curl -s https://install-panel.hbm.my.id/install.sh)
```

Yang dilakukan installer:
1. Pre-flight (OS, arch, port, RAM)
2. apt update + timezone + UFW + swap (bila <2GB RAM)
3. Install: Caddy · PHP 8.4 · PostgreSQL 16 · Redis · Node 22 · PM2 · Supervisor · FileBrowser · OpenSSH chroot
4. Download binary `hbmpanel` dari GitHub Releases
5. Register systemd unit `hbmpanel.service` (JWT secret di-generate via `openssl rand -hex 32`)
6. Caddy reverse proxy untuk panel (auto-SSL bila domain di-set)

Installer **idempotent** — aman dijalankan ulang.

## Architecture

```
hbmpanel/
├── installer/install.sh              # one-shot bootstrap installer (Bash)
├── backend/                          # Go (Fiber) + SQLite + embed.FS
│   ├── cmd/hbmpanel/main.go          # CLI entrypoint
│   └── internal/
│       ├── api/                      # Fiber router + middlewares
│       ├── auth/                     # JWT + bcrypt + TOTP
│       ├── db/                       # SQLite store + migrations
│       ├── modules/                  # caddy / pm2 / postgres / redis / site
│       │                             # laravel / logs / terminal / sshkeys
│       │                             # audit / ipwhitelist / license
│       │                             # sftp / filemanager / backup / gitdeploy
│       ├── web/                      # embed.FS untuk frontend static export
│       └── ws/                       # WebSocket log streamer
├── frontend/                         # Next.js 15 + Tailwind
│   └── app/dashboard/                # 13 pages
├── cloudflare-worker/                # license validation backend (Workers + KV)
├── packaging/systemd/hbmpanel.service
└── scripts/
    ├── build.sh                      # release build
    └── dev.sh                        # dev (backend + frontend)
```

## Security

- **Auth**: bcrypt password hashing, JWT (12h) di httpOnly Secure cookie SameSite=Strict, optional TOTP 2FA
- **CSRF**: `X-Requested-With` header check pada mutating methods
- **Brute-force**: rate-limit 10 req/min/IP di login endpoint
- **Audit**: semua mutating /api/* requests di-log ke `audit_log` table
- **Fail2ban**: failed login emit `[hbmpanel-auth] FAILED ip=<HOST> user=<X> reason=<Y>` (gampang dipakai sebagai fail2ban filter)
- **IP whitelist**: per-CIDR allowlist via settings (kosong = allow-all)
- **SFTP**: chroot via OpenSSH `Match Group sftpusers` + `ChrootDirectory`

## License

[GNU AGPL v3.0](./LICENSE) — kalau kamu host HBMPanel sebagai SaaS, modifikasi yang kamu buat juga harus open-source untuk user-mu.

Premium tier ada di repo ini juga tapi di-gate runtime via license module — bayar untuk dapat token validation + support + update legit. Lihat [`hbmpanel-prd.md`](./hbmpanel-prd.md) §16 untuk model monetisasi.

## Contributing

PR welcome. Sebelum submit:
- `go vet ./...` di `backend/` harus clean
- `npx tsc --noEmit` di `frontend/` harus clean
- Untuk fitur baru, ikuti pattern modul existing di `backend/internal/modules/`

## Support

- Issues: [GitHub Issues](https://github.com/habibbuchori/hbmpanel/issues)
- License & premium: [hbm.my.id](https://hbm.my.id)
