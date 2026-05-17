#!/bin/bash
# ============================================================
#  helpers.sh — Fungsi-fungsi umum untuk DNS Auto-Installer
# ============================================================

# === Warna ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# === Logging ===
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✅]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[⚠️]${NC} $1"; }
log_error()   { echo -e "${RED}[❌]${NC} $1"; }
log_step()    { echo -e "\n${CYAN}${BOLD}=== $1 ===${NC}"; }

# === Pre-flight Checks ===
check_root() {
  if [[ $EUID -ne 0 ]]; then
    log_error "Script harus dijalankan sebagai root. Gunakan: sudo $0"
    exit 1
  fi
}

check_os() {
  if [[ ! -f /etc/os-release ]]; then
    log_error "Tidak dapat mendeteksi OS."
    exit 1
  fi
  source /etc/os-release
  if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
    log_warn "OS terdeteksi: $ID. Script ini dirancang untuk Ubuntu/Debian."
    read -p "Lanjutkan? (y/n): " confirm
    [[ "$confirm" != "y" ]] && exit 1
  fi
  log_info "OS: $PRETTY_NAME"
}

check_port_free() {
  local port=$1
  if ss -tulnp | grep -q ":${port} "; then
    local proc
    proc=$(ss -tulnp | grep ":${port} " | awk '{print $NF}')
    log_warn "Port $port sudah dipakai oleh: $proc"
    return 1
  fi
  return 0
}

# === Deteksi Hardware ===
detect_cpu_cores() {
  local cores
  cores=$(nproc 2>/dev/null || echo 1)
  # Bulatkan ke power of 2 terdekat (Unbound perlu ini)
  local pow2=1
  while [[ $pow2 -lt $cores ]]; do
    pow2=$((pow2 * 2))
  done
  # Jangan lebih dari actual cores
  if [[ $pow2 -gt $cores ]]; then
    pow2=$((pow2 / 2))
    [[ $pow2 -lt 1 ]] && pow2=1
  fi
  echo "$pow2"
}

detect_ram_mb() {
  local ram_kb
  ram_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
  echo $((ram_kb / 1024))
}

# Hitung cache size optimal berdasarkan RAM
# Alokasi: ~10% RAM untuk msg-cache, 2x untuk rrset
calc_cache_sizes() {
  local ram_mb
  ram_mb=$(detect_ram_mb)

  if [[ $ram_mb -le 512 ]]; then
    MSG_CACHE="32m"
    RRSET_CACHE="64m"
    KEY_CACHE="16m"
    NEG_CACHE="16m"
  elif [[ $ram_mb -le 1024 ]]; then
    MSG_CACHE="64m"
    RRSET_CACHE="128m"
    KEY_CACHE="32m"
    NEG_CACHE="32m"
  elif [[ $ram_mb -le 2048 ]]; then
    MSG_CACHE="128m"
    RRSET_CACHE="256m"
    KEY_CACHE="64m"
    NEG_CACHE="64m"
  elif [[ $ram_mb -le 4096 ]]; then
    MSG_CACHE="256m"
    RRSET_CACHE="512m"
    KEY_CACHE="128m"
    NEG_CACHE="128m"
  elif [[ $ram_mb -le 8192 ]]; then
    MSG_CACHE="512m"
    RRSET_CACHE="1024m"
    KEY_CACHE="256m"
    NEG_CACHE="256m"
  else
    MSG_CACHE="1024m"
    RRSET_CACHE="2048m"
    KEY_CACHE="512m"
    NEG_CACHE="512m"
  fi

  log_info "RAM: ${ram_mb}MB → Cache: msg=${MSG_CACHE}, rrset=${RRSET_CACHE}"
}

# === File Utils ===
backup_file() {
  local file="$1"
  if [[ -f "$file" ]]; then
    local backup="${file}.bak.$(date +%Y%m%d%H%M%S)"
    cp "$file" "$backup"
    log_info "Backup: $file → $backup"
  fi
}

safe_download() {
  local url="$1"
  local dest="$2"
  local desc="${3:-file}"
  local max_retries=3
  local retry=0

  while [[ $retry -lt $max_retries ]]; do
    if curl -fsSL --connect-timeout 10 --max-time 60 "$url" -o "$dest" 2>/dev/null; then
      log_success "Download $desc berhasil"
      return 0
    fi
    retry=$((retry + 1))
    log_warn "Download $desc gagal (percobaan $retry/$max_retries)"
    sleep 2
  done

  log_error "Gagal download $desc dari $url setelah $max_retries percobaan"
  return 1
}

# === Crontab Utils ===
add_crontab_safe() {
  local job="$1"
  local current
  current=$(crontab -l 2>/dev/null || true)

  if echo "$current" | grep -qF "$job"; then
    log_info "Crontab sudah ada: ${job:0:60}..."
    return 0
  fi

  (echo "$current"; echo "$job") | crontab -
  log_success "Crontab ditambahkan: ${job:0:60}..."
}

remove_crontab() {
  local pattern="$1"
  local current
  current=$(crontab -l 2>/dev/null || true)

  if [[ -z "$current" ]]; then
    return 0
  fi

  echo "$current" | grep -vF "$pattern" | crontab - 2>/dev/null || true
}

# === Service Utils ===
safe_service_restart() {
  local service="$1"
  if systemctl is-active --quiet "$service" 2>/dev/null; then
    systemctl restart "$service"
  else
    systemctl start "$service"
  fi
}

# === Health Check ===
health_check_dns() {
  local server="${1:-127.0.0.1}"
  local domain="${2:-google.com}"
  local timeout=5

  log_info "Testing DNS resolve: $domain @${server}..."

  local result
  result=$(dig +short +time=$timeout @"$server" "$domain" 2>/dev/null)

  if [[ -n "$result" ]]; then
    local latency
    latency=$(dig +stats +time=$timeout @"$server" "$domain" 2>/dev/null | grep "Query time" | awk '{print $4}')
    log_success "DNS OK! $domain → $result (${latency}ms)"
    return 0
  else
    log_error "DNS gagal resolve $domain"
    return 1
  fi
}

# === Banner ===
show_banner() {
  echo -e "${CYAN}${BOLD}"
  echo "╔═══════════════════════════════════════════════╗"
  echo "║   🛡️  DNS Auto-Installer v2.0                 ║"
  echo "║   Anti-Bengong Edition                        ║"
  echo "║   Unbound + DoH/Recursive + Blocklist         ║"
  echo "╚═══════════════════════════════════════════════╝"
  echo -e "${NC}"
}
