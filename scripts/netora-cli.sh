#!/usr/bin/env bash
# ==============================================================================
# Netora DNS — NOC Operator CLI Helper
# Installed to: /usr/local/bin/netora
# ==============================================================================

set -euo pipefail

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m'

show_banner() {
    echo -e "${CYAN}${BOLD}"
    echo "  _   _      _                    ____  _   _ ____  "
    echo " | \ | | ___| |_ ___  _ __ __ _  |  _ \| \ | / ___| "
    echo " |  \| |/ _ \ __/ _ \| '__/ _\` | | | | |  \| \___ \ "
    echo " | |\  |  __/ || (_) | | | (_| | | |_| | |\  |___) |"
    echo " |_| \_|\___|\__\___/|_|  \__,_| |____/|_| \_|____/ "
    echo "  Enterprise Carrier-Grade ISP Unbound Platform     "
    echo -e "${NC}"
}

cmd_status() {
    show_banner
    echo -e "${BOLD}--- Service Status ---${NC}"
    for svc in unbound netora-api postgresql; do
        if systemctl is-active --quiet "$svc" 2>/dev/null; then
            echo -e "  $svc: \t${GREEN}● ACTIVE (running)${NC}"
        else
            echo -e "  $svc: \t${RED}● INACTIVE (failed/stopped)${NC}"
        fi
    done

    echo ""
    echo -e "${BOLD}--- Auto-Updater Status ---${NC}"
    if systemctl is-active --quiet netora-dns-updater.timer 2>/dev/null; then
        echo -e "  Timer: \t${GREEN}● ACTIVE (every 5m)${NC}"
    else
        echo -e "  Timer: \t${YELLOW}● DISABLED${NC}"
    fi

    if [[ -x /usr/local/sbin/netora-dns-update ]]; then
        echo ""
        /usr/local/sbin/netora-dns-update status || true
    fi
}

cmd_update() {
    if [[ ! -x /usr/local/sbin/netora-dns-update ]]; then
        echo -e "${RED}Error: /usr/local/sbin/netora-dns-update not found!${NC}"
        exit 1
    fi
    echo -e "${CYAN}Triggering autonomous update from GitHub...${NC}"
    /usr/local/sbin/netora-dns-update update "$@"
}

cmd_rollback() {
    if [[ ! -x /usr/local/sbin/netora-dns-update ]]; then
        echo -e "${RED}Error: /usr/local/sbin/netora-dns-update not found!${NC}"
        exit 1
    fi
    echo -e "${YELLOW}Initiating atomic rollback...${NC}"
    /usr/local/sbin/netora-dns-update rollback "$@"
}

cmd_restart() {
    echo -e "${CYAN}Restarting Unbound and Netora API...${NC}"
    systemctl restart unbound
    systemctl restart netora-api
    echo -e "${GREEN}Services restarted successfully.${NC}"
}

cmd_health() {
    echo -e "${BOLD}Checking DNS Data Plane Resolution...${NC}"
    local dig_out
    if dig_out=$(dig @127.0.0.1 google.com +short +time=3 2>/dev/null) && [[ -n "$dig_out" ]]; then
        echo -e "  DNS Query (127.0.0.1:53): ${GREEN}PASS${NC} -> $dig_out"
    else
        echo -e "  DNS Query (127.0.0.1:53): ${RED}FAIL${NC}"
    fi

    echo -e "${BOLD}Checking Netora API Health...${NC}"
    local api_health
    if api_health=$(curl -sf http://127.0.0.1:8080/api/v1/health 2>/dev/null); then
        echo -e "  API Endpoint (:8080):     ${GREEN}PASS${NC} -> $api_health"
    else
        echo -e "  API Endpoint (:8080):     ${RED}FAIL${NC}"
    fi
}

cmd_logs() {
    local target="${1:-unbound}"
    case "$target" in
        unbound)
            journalctl -u unbound -f -n 50
            ;;
        api)
            journalctl -u netora-api -f -n 50
            ;;
        updater)
            journalctl -u netora-dns-updater.service -f -n 50
            ;;
        *)
            echo "Usage: netora logs [unbound|api|updater]"
            exit 1
            ;;
    esac
}

cmd_info() {
    local ip
    ip=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "127.0.0.1")
    show_banner
    echo -e "${BOLD}Netora DNS Access & Topology:${NC}"
    echo -e "  Web UI (NOC Dashboard): ${CYAN}http://${ip}:8080/${NC}"
    echo -e "  API Health Endpoint:     ${CYAN}http://${ip}:8080/api/v1/health${NC}"
    echo -e "  Prometheus Metrics:      ${CYAN}http://${ip}:8080/metrics${NC}"
    echo -e "  DNS Listeners:           ${CYAN}${ip}:53, 127.0.0.1:53${NC}"
    echo ""
    echo -e "${BOLD}Default Operator Credentials:${NC}"
    echo -e "  Username: ${GREEN}admin${NC}"
    echo -e "  Password: ${GREEN}admin123${NC} (or as configured in /etc/netora-dns/api.env)"
}

cmd_reset_password() {
    if [[ -x /usr/local/bin/netora-reset-password ]]; then
        /usr/local/bin/netora-reset-password "$@"
    elif [[ -x /usr/local/bin/netora-cli ]]; then
        local user="${1:-}"
        local pass="${2:-}"
        if [[ -z "$user" ]]; then
            read -rp "Enter username to reset [admin]: " user
            user="${user:-admin}"
        fi
        if [[ -z "$pass" ]]; then
            read -rsp "Enter new password for '${user}' (min 6 characters): " pass
            echo ""
            read -rsp "Confirm new password: " confirm_pass
            echo ""
            if [[ "$pass" != "$confirm_pass" ]]; then
                echo -e "${RED}Error: Passwords do not match!${NC}" >&2
                exit 1
            fi
        fi
        /usr/local/bin/netora-cli -mode reset-password -username "$user" -new-password "$pass"
    else
        echo -e "${RED}Error: /usr/local/bin/netora-cli or /usr/local/bin/netora-reset-password not found!${NC}" >&2
        exit 1
    fi
}

cmd_users() {
    if [[ -x /usr/local/bin/netora-cli ]]; then
        /usr/local/bin/netora-cli -mode list-users
    else
        echo -e "${RED}Error: /usr/local/bin/netora-cli not found!${NC}" >&2
        exit 1
    fi
}

usage() {
    echo "Netora DNS Carrier CLI Utility"
    echo "Usage: netora <command> [options]"
    echo ""
    echo "Commands:"
    echo "  status               Show overall status of Unbound, API, DB & Auto-Updater"
    echo "  health               Run instant DNS & API health verification checks"
    echo "  update               Pull and atomically apply latest revision from GitHub"
    echo "  rollback             Restore previous stable configuration backup"
    echo "  restart              Restart Unbound and Netora API services gracefully"
    echo "  logs [svc]           Follow live logs (unbound, api, or updater)"
    echo "  info                 Display Web UI URL, node endpoints, and credentials"
    echo "  reset-password [u]   Reset NOC operator password directly on server"
    echo "  users                List all operator accounts and database status"
    exit 1
}

case "${1:-}" in
    status)         cmd_status ;;
    update)         shift; cmd_update "$@" ;;
    rollback)       shift; cmd_rollback "$@" ;;
    restart)        cmd_restart ;;
    health)         cmd_health ;;
    logs)           shift; cmd_logs "$@" ;;
    info)           cmd_info ;;
    reset-password) shift; cmd_reset_password "$@" ;;
    users)          shift; cmd_users "$@" ;;
    *)              usage ;;
esac
