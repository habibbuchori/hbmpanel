#!/usr/bin/env bash
# =============================================================================
# HBMPanel Uninstall Script
# Removes all components installed by install.sh
# =============================================================================
set -u

# Colors
C_RED=$'\033[31m'
C_YELLOW=$'\033[33m'
C_GREEN=$'\033[32m'
C_RESET=$'\033[0m'

die()    { printf "%s✗ %s%s\n" "$C_RED" "$*" "$C_RESET" >&2; exit 1; }
warn()   { printf "%s! %s%s\n" "$C_YELLOW" "$*" "$C_RESET"; }
ok()     { printf "%s✓ %s%s\n" "$C_GREEN" "$*" "$C_RESET"; }

[ "$(id -u)" -eq 0 ] || die "harus dijalankan sebagai root"

echo ""
echo "╔════════════════════════════════════════════════╗"
echo "║  HBMPanel Uninstall                            ║"
echo "╚════════════════════════════════════════════════╝"
echo ""
warn "Ini akan menghapus SEMUA komponen HBMPanel"
warn "Data akan HILANG PERMANEN"
read -p "Lanjut? (yes/no): " confirm
[ "$confirm" = "yes" ] || die "dibatalkan"

echo ""

# Stop services
systemctl stop hbmpanel 2>/dev/null || true
systemctl disable hbmpanel 2>/dev/null || true
ok "stop hbmpanel service"

# Remove systemd unit
rm -f /etc/systemd/system/hbmpanel.service
systemctl daemon-reload
ok "remove systemd unit"

# Remove binary
rm -f /usr/local/bin/hbmpanel
ok "remove binary"

# Remove data & config
rm -rf /var/lib/hbmpanel
rm -rf /var/log/hbmpanel*
ok "remove data & logs"

# Remove packages
apt-get purge -y \
  caddy \
  php8.4-fpm php8.4-cli php8.4-pgsql php8.4-redis php8.4-mbstring php8.4-xml php8.4-curl php8.4-zip php8.4-bcmath php8.4-intl php8.4-gd \
  postgresql postgresql-contrib \
  redis-server \
  nodejs npm \
  supervisor \
  openssh-server \
  2>/dev/null || true
ok "remove packages"

# Remove repos
rm -f /etc/apt/sources.list.d/sury-php.list /usr/share/keyrings/sury-php.gpg
rm -f /etc/apt/sources.list.d/pgdg.list
apt-get update -qq
ok "remove repos"

echo ""
echo "╔════════════════════════════════════════════════╗"
echo "║  Uninstall complete                            ║"
echo "╚════════════════════════════════════════════════╝"
