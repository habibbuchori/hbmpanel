#!/usr/bin/env bash
# =============================================================================
# HBMPanel One-Shot Installer
# -----------------------------------------------------------------------------
# Pasang Caddy, PHP-FPM, PostgreSQL, Redis, Node.js, PM2, Supervisor,
# FileBrowser, OpenSSH (chroot SFTP), lalu install panel `hbmpanel`.
#
# Usage:
#   bash <(curl -s https://install-panel.hbm.my.id)
#
# Non-interactive env vars:
#   HBMPANEL_DOMAIN    (opsional) — domain panel, auto-SSL bila di-set
#   HBMPANEL_EMAIL     (opsional) — email untuk Let's Encrypt
#   HBMPANEL_CHANNEL   (default: stable) — stable | beta
#   HBMPANEL_VERSION   (default: latest)
#   HBMPANEL_PORT      (default: 8443)
# =============================================================================
set -Eeuo pipefail

# ---------- konstanta ----------
PANEL_USER="hbmpanel"
PANEL_HOME="/var/lib/hbmpanel"
PANEL_BIN="/usr/local/bin/hbmpanel"
PANEL_LOG="/var/log/hbmpanel"
INSTALL_LOG="/var/log/hbmpanel-install.log"
MARKER_DIR="/var/lib/hbmpanel/.install-markers"
CHANNEL="${HBMPANEL_CHANNEL:-stable}"
VERSION="${HBMPANEL_VERSION:-latest}"
PANEL_PORT="${HBMPANEL_PORT:-8443}"
PANEL_DOMAIN="${HBMPANEL_DOMAIN:-}"
PANEL_EMAIL="${HBMPANEL_EMAIL:-}"
GH_REPO="habibbuchori/hbmpanel"

# ---------- warna ----------
if [ -t 1 ]; then
  C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'
  C_RED=$'\033[31m'; C_GREEN=$'\033[32m'
  C_YELLOW=$'\033[33m'; C_BLUE=$'\033[34m'; C_DIM=$'\033[2m'
else
  C_RESET=; C_BOLD=; C_RED=; C_GREEN=; C_YELLOW=; C_BLUE=; C_DIM=
fi

# ---------- logger ----------
log()    { printf "%s[hbmpanel]%s %s\n" "$C_BLUE" "$C_RESET" "$*" | tee -a "$INSTALL_LOG"; }
ok()     { printf "%s ✓ %s%s\n" "$C_GREEN" "$*" "$C_RESET" | tee -a "$INSTALL_LOG"; }
warn()   { printf "%s ! %s%s\n" "$C_YELLOW" "$*" "$C_RESET" | tee -a "$INSTALL_LOG"; }
err()    { printf "%s ✗ %s%s\n" "$C_RED" "$*" "$C_RESET" | tee -a "$INSTALL_LOG" >&2; }
die()    { err "$*"; exit 1; }

# ---------- marker (idempotency) ----------
mark_done() { mkdir -p "$MARKER_DIR"; touch "$MARKER_DIR/$1"; }
is_done()   { [ -f "$MARKER_DIR/$1" ]; }

run_step() {
  local name="$1"; shift
  if is_done "$name"; then
    log "${C_DIM}skip $name (already done)${C_RESET}"
    return 0
  fi
  log "$name ..."
  if "$@"; then
    mark_done "$name"
    ok "$name"
  else
    die "step '$name' gagal — lihat $INSTALL_LOG"
  fi
}

# ---------- error trap ----------
on_error() {
  local exit_code=$?
  local line_no=$1
  err "Installer gagal di baris $line_no (exit $exit_code)"
  err "Log lengkap: $INSTALL_LOG"
  exit "$exit_code"
}
trap 'on_error $LINENO' ERR

# =============================================================================
# 0. PRE-FLIGHT
# =============================================================================
preflight() {
  [ "$(id -u)" -eq 0 ] || die "harus dijalankan sebagai root"

  # OS detect
  if [ ! -r /etc/os-release ]; then die "OS tidak terdeteksi"; fi
  # shellcheck disable=SC1091
  . /etc/os-release
  case "$ID" in
    debian) [ "${VERSION_ID%%.*}" -ge 12 ] || die "butuh Debian 12+";;
    ubuntu) [ "${VERSION_ID%%.*}" -ge 24 ] || die "butuh Ubuntu 24.04+";;
    *) die "OS tidak didukung: $ID (butuh Debian 12 / Ubuntu 24.04)";;
  esac

  # Arch
  local arch; arch="$(uname -m)"
  case "$arch" in
    x86_64|aarch64) ;;
    *) die "arch tidak didukung: $arch";;
  esac

  # RAM
  local mem_kb; mem_kb=$(awk '/MemTotal/ {print $2}' /proc/meminfo)
  [ "$mem_kb" -ge 900000 ] || warn "RAM <1GB — panel akan jalan tapi performa terbatas"

  # Port
  for p in 80 443 "$PANEL_PORT"; do
    if ss -ltn "( sport = :$p )" 2>/dev/null | grep -q LISTEN; then
      warn "port $p sudah dipakai — pastikan tidak konflik"
    fi
  done

  mkdir -p "$MARKER_DIR" "$PANEL_LOG"
  touch "$INSTALL_LOG"
  ok "pre-flight ok ($ID $VERSION_ID, $arch)"
}

# =============================================================================
# 1. SYSTEM PREP
# =============================================================================
sys_apt_update() {
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq >>"$INSTALL_LOG" 2>&1
  apt-get install -y -qq \
    curl ca-certificates gnupg lsb-release \
    apt-transport-https debian-archive-keyring \
    ufw unzip jq sudo >>"$INSTALL_LOG" 2>&1
}

sys_timezone() {
  timedatectl set-timezone UTC || true
}

sys_swap() {
  local mem_kb swap_kb
  mem_kb=$(awk '/MemTotal/ {print $2}' /proc/meminfo)
  swap_kb=$(awk '/SwapTotal/ {print $2}' /proc/meminfo)
  if [ "$mem_kb" -lt 2000000 ] && [ "$swap_kb" -lt 100000 ]; then
    log "RAM kecil & swap belum ada → buat 2GB swapfile"
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile >/dev/null
    swapon /swapfile
    grep -q '/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
  fi
}

sys_firewall() {
  ufw --force default deny incoming >/dev/null
  ufw --force default allow outgoing >/dev/null
  ufw allow 22/tcp >/dev/null
  ufw allow 80/tcp >/dev/null
  ufw allow 443/tcp >/dev/null
  ufw allow "${PANEL_PORT}/tcp" >/dev/null
  ufw --force enable >/dev/null 2>&1 || true
}

# =============================================================================
# 2. CORE STACK INSTALL
# =============================================================================

install_caddy() {
  if command -v caddy >/dev/null; then return 0; fi
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
  apt-get update -qq >>"$INSTALL_LOG" 2>&1
  apt-get install -y -qq caddy >>"$INSTALL_LOG" 2>&1
  systemctl enable --now caddy >>"$INSTALL_LOG" 2>&1
}

install_php() {
  if command -v php8.4 >/dev/null; then return 0; fi
  curl -sSLo /tmp/sury.gpg https://packages.sury.org/php/apt.gpg
  install -m 0644 /tmp/sury.gpg /usr/share/keyrings/sury-php.gpg
  echo "deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ $(lsb_release -sc) main" \
    > /etc/apt/sources.list.d/sury-php.list
  apt-get update -qq >>"$INSTALL_LOG" 2>&1
  apt-get install -y -qq \
    php8.4-fpm php8.4-cli php8.4-pgsql php8.4-redis \
    php8.4-mbstring php8.4-xml php8.4-curl php8.4-zip \
    php8.4-bcmath php8.4-intl php8.4-gd >>"$INSTALL_LOG" 2>&1
  systemctl enable --now php8.4-fpm >>"$INSTALL_LOG" 2>&1
}

install_postgres() {
  if command -v psql >/dev/null; then return 0; fi
  install -d /usr/share/postgresql-common/pgdg
  curl -sSLo /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
    https://www.postgresql.org/media/keys/ACCC4CF8.asc
  echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $(lsb_release -sc)-pgdg main" \
    > /etc/apt/sources.list.d/pgdg.list
  apt-get update -qq >>"$INSTALL_LOG" 2>&1
  apt-get install -y -qq postgresql-16 >>"$INSTALL_LOG" 2>&1
  systemctl enable --now postgresql >>"$INSTALL_LOG" 2>&1
}

install_redis() {
  if command -v redis-cli >/dev/null; then return 0; fi
  apt-get install -y -qq redis-server >>"$INSTALL_LOG" 2>&1
  systemctl enable --now redis-server >>"$INSTALL_LOG" 2>&1
}

install_node() {
  if command -v node >/dev/null && node -v | grep -q '^v22\.'; then return 0; fi
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash - >>"$INSTALL_LOG" 2>&1
  apt-get install -y -qq nodejs >>"$INSTALL_LOG" 2>&1
  corepack enable >>"$INSTALL_LOG" 2>&1 || true
}

install_pm2() {
  if command -v pm2 >/dev/null; then return 0; fi
  npm install -g pm2 >>"$INSTALL_LOG" 2>&1
  pm2 startup systemd -u root --hp /root >>"$INSTALL_LOG" 2>&1 || true
}

install_supervisor() {
  if command -v supervisorctl >/dev/null; then return 0; fi
  apt-get install -y -qq supervisor >>"$INSTALL_LOG" 2>&1
  systemctl enable --now supervisor >>"$INSTALL_LOG" 2>&1
}

install_filebrowser() {
  if command -v filebrowser >/dev/null; then return 0; fi
  curl -fsSL https://raw.githubusercontent.com/filebrowser/get/master/get.sh | bash >>"$INSTALL_LOG" 2>&1
}

install_openssh_chroot() {
  apt-get install -y -qq openssh-server >>"$INSTALL_LOG" 2>&1
  # Tambah group sftp jika belum ada
  getent group sftpusers >/dev/null || groupadd sftpusers
  # Tambah Match block bila belum ada
  if ! grep -q '^Match Group sftpusers' /etc/ssh/sshd_config; then
    cat >>/etc/ssh/sshd_config <<'EOF'

# --- HBMPanel chroot SFTP ---
Match Group sftpusers
    ChrootDirectory /home/%u
    ForceCommand internal-sftp
    AllowTcpForwarding no
    X11Forwarding no
EOF
  fi
  systemctl restart ssh >>"$INSTALL_LOG" 2>&1
}

# =============================================================================
# 3. PANEL INSTALL
# =============================================================================
panel_user() {
  id "$PANEL_USER" >/dev/null 2>&1 || \
    useradd --system --home "$PANEL_HOME" --shell /usr/sbin/nologin "$PANEL_USER"
  install -d -o "$PANEL_USER" -g "$PANEL_USER" "$PANEL_HOME" "$PANEL_LOG"
}

panel_download() {
  local arch tag url
  case "$(uname -m)" in
    x86_64)  arch="amd64";;
    aarch64) arch="arm64";;
  esac

  if [ "$VERSION" = "latest" ]; then
    tag=$(curl -fsSL "https://api.github.com/repos/${GH_REPO}/releases/latest" \
      | jq -r '.tag_name' 2>/dev/null || echo "")
    if [ -z "$tag" ] || [ "$tag" = "null" ]; then
      warn "tidak bisa fetch release terbaru dari ${GH_REPO} — installer akan skip download binary."
      warn "saat development: build binary manual dan letakkan di $PANEL_BIN"
      return 0
    fi
  else
    tag="$VERSION"
  fi

  url="https://github.com/${GH_REPO}/releases/download/${tag}/hbmpanel-linux-${arch}.tar.gz"
  log "download $url"
  curl -fSL "$url" -o /tmp/hbmpanel.tar.gz
  tar -xzf /tmp/hbmpanel.tar.gz -C /tmp/
  install -m 0755 /tmp/hbmpanel "$PANEL_BIN"
  rm -f /tmp/hbmpanel.tar.gz /tmp/hbmpanel
}

panel_systemd() {
  cat >/etc/systemd/system/hbmpanel.service <<EOF
[Unit]
Description=HBMPanel
After=network.target

[Service]
Type=simple
User=root
ExecStart=${PANEL_BIN} serve --port ${PANEL_PORT}
Restart=on-failure
RestartSec=3
Environment=HBMPANEL_HOME=${PANEL_HOME}
Environment=HBMPANEL_LOG=${PANEL_LOG}

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  if [ -x "$PANEL_BIN" ]; then
    systemctl enable --now hbmpanel >>"$INSTALL_LOG" 2>&1
  else
    warn "binary $PANEL_BIN belum ada — systemd unit terdaftar tapi tidak di-start"
  fi
}

panel_init() {
  if [ -x "$PANEL_BIN" ]; then
    "$PANEL_BIN" init --home "$PANEL_HOME" >>"$INSTALL_LOG" 2>&1 || true
  fi
}

# =============================================================================
# 4. CADDY REVERSE PROXY
# =============================================================================
caddy_panel_config() {
  local caddyfile="/etc/caddy/Caddyfile.d/hbmpanel.caddy"
  install -d /etc/caddy/Caddyfile.d
  # pastikan Caddyfile utama meng-import folder
  if ! grep -q 'import Caddyfile.d/\*' /etc/caddy/Caddyfile 2>/dev/null; then
    echo 'import Caddyfile.d/*.caddy' >> /etc/caddy/Caddyfile
  fi

  if [ -n "$PANEL_DOMAIN" ]; then
    cat >"$caddyfile" <<EOF
${PANEL_DOMAIN} {
    reverse_proxy localhost:${PANEL_PORT}
}
EOF
  else
    cat >"$caddyfile" <<EOF
:${PANEL_PORT} {
    tls internal
    reverse_proxy localhost:${PANEL_PORT}
}
EOF
  fi
  systemctl reload caddy >>"$INSTALL_LOG" 2>&1 || systemctl restart caddy >>"$INSTALL_LOG" 2>&1
}

# =============================================================================
# 5. FINISH
# =============================================================================
finish() {
  local ip url
  ip="$(hostname -I | awk '{print $1}')"
  if [ -n "$PANEL_DOMAIN" ]; then
    url="https://${PANEL_DOMAIN}"
  else
    url="https://${ip}:${PANEL_PORT}"
  fi

  cat <<EOF | tee /root/.hbmpanel-info

${C_BOLD}${C_GREEN}╔══════════════════════════════════════════════════════════╗
║              HBMPanel installation complete              ║
╚══════════════════════════════════════════════════════════╝${C_RESET}

  URL    : ${C_BOLD}${url}${C_RESET}
  Log    : ${INSTALL_LOG}
  Data   : ${PANEL_HOME}
  Binary : ${PANEL_BIN}

  Cek status :  systemctl status hbmpanel
  Cek log    :  journalctl -u hbmpanel -f

  Kredensial admin awal akan dicetak oleh \`hbmpanel init\`
  pada saat first-run, dan disimpan di:
    ${PANEL_HOME}/initial-credentials.txt

EOF
}

# =============================================================================
# MAIN
# =============================================================================
main() {
  log "${C_BOLD}HBMPanel installer${C_RESET}"
  preflight

  run_step "01-apt-update"          sys_apt_update
  run_step "02-timezone"            sys_timezone
  run_step "03-swap"                sys_swap
  run_step "04-firewall"            sys_firewall

  run_step "10-caddy"               install_caddy
  run_step "11-php"                 install_php
  run_step "12-postgres"            install_postgres
  run_step "13-redis"               install_redis
  run_step "14-node"                install_node
  run_step "15-pm2"                 install_pm2
  run_step "16-supervisor"          install_supervisor
  run_step "17-filebrowser"         install_filebrowser
  run_step "18-openssh-chroot"      install_openssh_chroot

  run_step "20-panel-user"          panel_user
  run_step "21-panel-download"      panel_download
  run_step "22-panel-systemd"       panel_systemd
  run_step "23-panel-init"          panel_init

  run_step "30-caddy-panel"         caddy_panel_config

  finish
}

main "$@"
