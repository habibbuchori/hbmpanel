# PRD — Lightweight Modern App Panel

## Nama Sementara
**HBMPanel**  

---

# 1. Overview

HBMPanel adalah panel server modern dan ringan untuk developer yang ingin mengelola aplikasi modern seperti:

- Laravel
- Next.js
- Node.js
- Go/Fiber
- PHP Apps

dengan pendekatan:
- lightweight
- native Linux
- tidak bloated
- cocok VPS/LXC/mini PC
- modern UI
- fokus deployment aplikasi

HBMPanel bukan shared hosting panel seperti cPanel/Plesk.

---

# 2. Goals

## Primary Goals

- Mempermudah deploy aplikasi modern
- Menyediakan GUI ringan untuk server management
- Mengurangi kebutuhan CLI untuk task umum
- Menjadi alternatif ringan CloudPanel/Coolify
- Mendukung stack modern tanpa Docker wajib

---

# 3. Non Goals

HBMPanel tidak fokus pada:

- Reseller hosting
- Billing system
- Mail server management
- DNS cluster
- Shared hosting enterprise
- WHM/cPanel replacement
- Multi-tenant enterprise hosting

---

# 4. Target Users

## Primary

- Laravel developers
- Next.js developers
- Indie hackers
- VPS users
- Homelab users
- Proxmox/LXC users
- Small startup teams

## Secondary

- Digital agencies
- Self-hosted enthusiasts
- DevOps beginners

---

# 5. Core Technology Stack

## Server Stack

```text
Caddy
PHP 8.4
PostgreSQL
Redis
Node.js 22
PM2
Supervisor
OpenSSH (SFTP)
FileBrowser
```

---

## Panel Backend

**Decision: Fiber (Go)** — final.

Alasan:
- Single static binary (~30-50MB), zero runtime dependency
- Idle RAM <50MB (jauh di bawah target 300MB)
- Cross-compile mudah untuk amd64 + arm64
- Cocok dengan goal "one-shot installer" — panel tidak butuh PHP/Node terpasang untuk dirinya sendiri (PHP/Node tetap dipasang untuk aplikasi user yang dikelola)

Laravel API ditinggalkan sebagai opsi backend panel karena menambah kompleksitas installer (butuh PHP-CLI, composer, file permissions, FPM untuk panel sendiri).

---

## Panel Frontend

- Next.js 16 dengan **static export** (`output: 'export'`)
- Tailwind CSS
- shadcn/ui
- Di-embed ke Go binary via `embed.FS` agar 1 binary saja yang di-deploy

---

## Database

### Internal panel (config, users, sessions, audit log)
- **SQLite** (file di `/var/lib/hbmpanel/panel.db`) — no extra service

### Untuk aplikasi user yang dikelola
- PostgreSQL 16

---

## Realtime

- **Native Go WebSocket** (gorilla/websocket atau gofiber/websocket)
- SSE untuk log streaming sederhana

Laravel Reverb tidak dipakai karena backend bukan Laravel.

---

# 6. System Architecture

```text
┌──────────────────────────────────────────────┐
│  hbmpanel (single Go binary)                 │
│  ├─ Embedded Next.js static UI (embed.FS)    │
│  ├─ Fiber HTTP server (:8443)                │
│  ├─ SQLite (panel.db)                        │
│  └─ Modules:                                 │
│      ├── Caddy Config Generator              │
│      ├── PM2 Controller (exec)               │
│      ├── Supervisor Controller (exec)        │
│      ├── PostgreSQL Manager (libpq/psql)     │
│      ├── Redis Manager (redis-cli)           │
│      ├── File Manager Bridge → FileBrowser   │
│      ├── SFTP User Manager (useradd/chroot)  │
│      ├── Deployment Engine (git+build)       │
│      └── Log Streaming (WS/SSE)              │
└──────────────────────────────────────────────┘
                 │
                 ▼  systemd
        ┌────────────────────┐
        │  System Services   │
        │  Caddy, PHP-FPM,   │
        │  PostgreSQL, Redis │
        │  PM2, Supervisor   │
        │  OpenSSH, FileBrws │
        └────────────────────┘
```

Komunikasi panel ↔ system service dilakukan via:
- `os/exec` untuk perintah CLI (systemctl, pm2, psql, dll)
- Unix domain socket / TCP localhost untuk service yang mendukung (Redis, Postgres)
- Caddy admin API (`http://localhost:2019`) untuk reload config tanpa restart

---

# 7. Main Features

# 7.1 Authentication

## Features

- Login
- Logout
- Session management
- Password reset
- 2FA (future)

---

# 7.2 Dashboard

## Overview Widgets

- CPU usage
- RAM usage
- Disk usage
- Active sites
- Running services
- Redis usage
- PostgreSQL status
- PM2 process count

---

# 7.3 Site Management

## Features

### Create Site

User can choose:

- PHP/Laravel
- Node.js
- Static site
- Reverse proxy

---

## Site Configuration

### Fields

- Domain
- Root path
- PHP version
- Node port
- SSL toggle
- Environment variables

---

## Actions

- Start
- Stop
- Restart
- Delete
- Rebuild
- Open logs

---

# 7.4 Caddy Integration

## Features

- Auto generate Caddyfile
- SSL auto provisioning
- Reverse proxy management
- Domain binding
- HTTP → HTTPS redirect

---

## Example Generated Config

```caddyfile
example.com {
    root * /home/apps/example/public
    php_fastcgi unix//run/php/php8.4-fpm.sock
    file_server
}

app.example.com {
    reverse_proxy localhost:3000
}
```

---

# 7.5 Laravel Management

## Features

### GUI Actions

- Artisan commands
- Queue restart
- Cache clear
- Config clear
- Route clear
- Migrate database
- Seed database
- Optimize
- Horizon control

---

## Queue Management

- Start worker
- Stop worker
- Restart worker

---

## Scheduler

- Enable scheduler
- Monitor scheduler

---

# 7.6 Node.js / Next.js Management

## Features

### PM2 Integration

- Start app
- Stop app
- Restart app
- View logs
- Monitor memory
- Auto restart

---

## Build Actions

- npm install
- pnpm install
- build app
- standalone deploy

---

# 7.7 Database Management

# PostgreSQL

## Features

- Create database
- Delete database
- Create user
- Assign permissions
- Backup database
- Restore database

---

## Future

- Built-in SQL editor

---

# 7.8 Redis Management

## Features

- Restart Redis
- Flush cache
- Monitor memory
- View connected clients

---

# 7.9 File Manager

## Integrated Engine

- FileBrowser

---

## Features

- Upload file
- Download file
- Extract ZIP
- Edit file
- Delete file
- Change permissions
- Drag & drop upload

---

# 7.10 SFTP Management

## Features

- Create SFTP user
- Reset password
- Restrict folder access
- Assign site directory

---

## Backend

- OpenSSH
- Chroot jail

---

# 7.11 Log Viewer

## Supported Logs

- Caddy logs
- Laravel logs
- PM2 logs
- Supervisor logs
- System logs

---

## Features

- Live tail
- Search
- Download log

---

# 7.12 Terminal Access

## Features

- Browser terminal
- Restricted shell
- Site-based access

---

## Technology

- xterm.js
- WebSocket

---

# 7.13 Backup System

## Features

- Site backup
- Database backup
- Scheduled backup
- Restore backup

---

## Storage

- Local storage
- S3 compatible (future)

---

# 8. Deployment System

# Git Deployment

## Features

- Git clone
- Pull latest
- Branch selection
- Auto build
- Deploy hooks

---

# Future CI/CD

- GitHub integration
- GitLab integration
- Auto deployment

---

# 9. Security

## Requirements

- CSRF protection
- Rate limiting
- SSH key authentication
- SFTP isolation
- Secure secret storage
- Firewall recommendations

---

## Future

- Fail2Ban integration
- IP whitelist
- Audit logs

---

# 10. Performance Goals

## Target Resource Usage

### Idle

```text
RAM:
< 300MB panel core
```

---

## Small VPS Target

```text
1 vCPU
2GB RAM
```

must still usable.

---

# 11. Supported Environments

## Official

- Debian 12
- Ubuntu 24.04

---

## Recommended

- Proxmox LXC
- VPS
- Bare metal mini PC

---

# 12. Installation Method

## Prinsip Inti

**One-shot, zero-prompt, idempotent.**

User cukup menjalankan satu perintah pada Debian 12 / Ubuntu 24.04 / LXC yang masih fresh, dan setelah selesai:
- Panel tersedia di `https://<server-ip>:8443` (atau domain bila di-set)
- Seluruh tool dasar (Caddy, PHP-FPM, PostgreSQL, Redis, Node.js, PM2, Supervisor, FileBrowser, OpenSSH chroot) sudah terpasang dan running
- Kredensial admin awal di-print sekali ke stdout dan disimpan di `/root/.hbmpanel-info`

## One-line installer

```bash
bash <(curl -s https://install-panel.hbm.my.id)
```

## Non-interactive (advanced)

```bash
HBMPANEL_DOMAIN=panel.example.com \
HBMPANEL_EMAIL=admin@example.com \
HBMPANEL_CHANNEL=stable \
bash <(curl -s https://install-panel.hbm.my.id)
```

## Tahapan Installer

1. **Pre-flight** — deteksi OS (Debian 12 / Ubuntu 24.04), arch (amd64/arm64), root, port 80/443/8443 free, RAM ≥ 1GB
2. **System prep** — apt update, timezone, swap (bila RAM <2GB), UFW basic rules
3. **Core stack install** — Caddy, PHP 8.4 FPM, PostgreSQL 16, Redis, Node.js 22, PM2, Supervisor, FileBrowser, OpenSSH (semua dari official repo)
4. **Panel install** — download binary `hbmpanel` dari GitHub Releases, install systemd unit, generate secrets, init SQLite
5. **Reverse proxy** — Caddy config untuk panel, auto-SSL via Let's Encrypt bila domain di-set
6. **Finish** — print credentials + URL akses

## Desain Installer

- **Idempotent**: setiap step cek dulu sebelum bertindak (cek `command -v`, cek status systemd unit, cek file marker)
- **Atomic per-step**: tiap step ada fungsi `step_X` dengan rollback marker
- **No prompts**: semua decision via env var atau auto-detect
- **Versioned**: installer pin versi tool tertentu agar reproducible
- **Logging**: setiap step log ke `/var/log/hbmpanel-install.log`

---

# 13. Future Roadmap

# Phase 1 — MVP

- Authentication
- Dashboard
- Site management
- Caddy integration
- PM2 integration
- PostgreSQL management
- File manager
- SFTP management

---

# Phase 2

- Docker support
- Multi PHP version
- Redis manager
- Backup scheduler
- Web terminal

---

# Phase 3

- Multi server management
- Team collaboration
- Monitoring dashboard
- Git auto deployment
- Notifications

---

# Phase 4

- Kubernetes integration
- High availability
- Cluster mode
- Plugin ecosystem

---

# 14. Differentiator

| LitePanel | Traditional Panels |
|---|---|
| Lightweight | Heavy |
| Modern stack focused | Shared hosting focused |
| Native app deployment | Legacy hosting workflow |
| Caddy-first | Apache/Nginx legacy |
| PM2 integrated | Node.js secondary |
| Developer workflow | Hosting workflow |

---

# 15. Success Metrics

## Technical

- Install < 10 minutes
- Idle RAM < 300MB
- Site deploy < 2 minutes

---

## Product

- 1-click app deploy
- Easy onboarding
- Minimal Linux knowledge required

---

# 16. Open Source Strategy

## License Suggestion

- AGPL
atau
- Apache 2.0

---

## Monetization (Future Optional)

- Cloud backup
- Managed hosting
- Premium plugins
- Team features

---

# 17. MVP Priority Order

## Highest Priority

1. Authentication
2. Site management
3. Caddy integration
4. PM2 integration
5. PostgreSQL manager
6. File manager
7. SFTP manager

---

# 18. UI Philosophy

## Design Principles

- Minimal
- Fast
- Clean
- Dark mode first
- Developer oriented
- No unnecessary menu

---

# 19. Competitor Analysis

| Product | Weakness |
|---|---|
| CloudPanel | PHP-centric |
| Coolify | Docker-heavy |
| CyberPanel | Heavy |
| aaPanel | Too generic |
| Plesk | Expensive/heavy |

---

# 20. Final Vision

Menjadi panel lightweight modern terbaik untuk:
- Laravel
- Next.js
- Node.js
- Modern self-hosted apps

dengan fokus:
- simplicity
- speed
- low resource usage
- modern deployment workflow.
