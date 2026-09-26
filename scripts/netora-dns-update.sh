#!/usr/bin/env bash
# ==============================================================================
# Netora DNS — Autonomous Server-Side Auto-Update Agent
# Path: /usr/local/sbin/netora-dns-update (Source: scripts/netora-dns-update.sh)
# ==============================================================================

set -euo pipefail

UPDATER_VERSION="1.0.1"
CONFIG_FILE="/etc/netora-dns/updater.conf"
NODE_ID_FILE="/etc/netora-dns/node-id"
LOCK_FILE="/run/lock/netora-dns-update.lock"

# Default File System Hierarchy
NETORA_BASE="/opt/netora-dns"
REPO_DIR="${NETORA_BASE}/repository"
RELEASES_DIR="${NETORA_BASE}/releases"
BACKUPS_DIR="${NETORA_BASE}/backups"
STAGING_DIR="${NETORA_BASE}/staging"
STATE_DIR="/var/lib/netora-dns"
LOG_DIR="/var/log/netora-dns"

STATE_FILE="${STATE_DIR}/update-state.json"
LOG_FILE="${LOG_DIR}/update.log"

# Default Configuration Values
NODE_ID="dns-unknown"
CHANNEL="development"
REPO_URL="https://github.com/desienkz-slp/unbound.git"
GIT_BRANCH="main"
TOKEN_FILE="/etc/netora-dns/github.token"
AUTO_UPDATE_ENABLED="true"
MAINTENANCE_MODE="false"
BACKUP_RETENTION=5
HEALTH_CHECK_TIMEOUT=5
MAX_RELOAD_WAIT=10
OVERLAYS_DIR="/etc/netora-dns/overlays"

# Load Configuration if exists
if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$CONFIG_FILE"
fi

if [[ -f "$NODE_ID_FILE" ]]; then
    NODE_ID="$(tr -d '[:space:]' < "$NODE_ID_FILE")"
elif [[ "$NODE_ID" == "dns-unknown" ]]; then
    NODE_ID="$(hostname -s 2>/dev/null || echo "dns-node")"
fi

# Ensure base directories exist with secure permissions
mkdir -p "$NETORA_BASE" "$RELEASES_DIR" "$BACKUPS_DIR" "$STAGING_DIR" "$STATE_DIR" "$LOG_DIR"
chmod 750 "$NETORA_BASE" "$RELEASES_DIR" "$BACKUPS_DIR" "$STAGING_DIR" "$STATE_DIR" "$LOG_DIR"

# ------------------------------------------------------------------------------
# Logging & State Utilities
# ------------------------------------------------------------------------------
log_event() {
    local level="$1"
    local event="$2"
    local message="$3"
    local iso_time
    iso_time="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    
    # Human terminal output
    if [[ -t 1 ]]; then
        case "$level" in
            INFO)  echo -e "\033[0;32m[INFO]\033[0m  ${iso_time} | ${event}: ${message}" ;;
            WARN)  echo -e "\033[0;33m[WARN]\033[0m  ${iso_time} | ${event}: ${message}" ;;
            ERROR) echo -e "\033[0;31m[ERROR]\033[0m ${iso_time} | ${event}: ${message}" ;;
            *)     echo "[${level}] ${iso_time} | ${event}: ${message}" ;;
        esac
    fi

    # Append structured record to log file
    printf '{"time":"%s","level":"%s","node_id":"%s","event":"%s","message":"%s"}\n' \
        "$iso_time" "$level" "$NODE_ID" "$event" "$message" >> "$LOG_FILE"
}

get_current_revision() {
    if [[ -f "$STATE_FILE" ]] && command -v jq >/dev/null 2>&1; then
        jq -r '.current_revision // "none"' "$STATE_FILE"
    else
        echo "none"
    fi
}

update_state_file() {
    local status="$1"
    local curr_rev="$2"
    local prev_rev="$3"
    local last_err="$4"
    local health="$5"
    local duration="$6"
    local iso_time
    iso_time="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

    cat <<EOF > "${STATE_FILE}.tmp"
{
  "node_id": "${NODE_ID}",
  "channel": "${CHANNEL}",
  "current_revision": "${curr_rev}",
  "previous_revision": "${prev_rev}",
  "update_time": "${iso_time}",
  "update_status": "${status}",
  "update_duration_sec": ${duration},
  "last_error": "${last_err}",
  "health_status": "${health}",
  "auto_update_enabled": ${AUTO_UPDATE_ENABLED},
  "maintenance_mode": ${MAINTENANCE_MODE}
}
EOF
    mv -f "${STATE_FILE}.tmp" "$STATE_FILE"
    chmod 644 "$STATE_FILE"
}

# ------------------------------------------------------------------------------
# Git Authentication Helper (In-Memory Only, Never Committed or Exposed)
# ------------------------------------------------------------------------------
get_authenticated_repo_url() {
    local token=""
    if [[ -f "$TOKEN_FILE" ]]; then
        # Check permissions: must be strict
        local perms
        perms="$(stat -c "%a" "$TOKEN_FILE" 2>/dev/null || echo "600")"
        if [[ "$perms" != "600" && "$perms" != "400" ]]; then
            chmod 600 "$TOKEN_FILE" 2>/dev/null || true
        fi
        token="$(tr -d '[:space:]' < "$TOKEN_FILE")"
    fi

    if [[ -n "$token" && "$REPO_URL" =~ ^https:// ]]; then
        # Insert token into clone URL in memory
        local clean_url="${REPO_URL#https://}"
        echo "https://${token}@${clean_url}"
    else
        echo "$REPO_URL"
    fi
}

# ------------------------------------------------------------------------------
# Lock Management (flock)
# ------------------------------------------------------------------------------
acquire_lock() {
    exec 200>"$LOCK_FILE"
    if ! flock -n 200; then
        log_event "WARN" "update_locked" "Another update process is already running. Exiting safely."
        exit 0
    fi
}

# ------------------------------------------------------------------------------
# Action: Check
# ------------------------------------------------------------------------------
action_check() {
    log_event "INFO" "check_started" "Checking latest revision on ${REPO_URL} (${GIT_BRANCH})"
    
    local auth_url
    auth_url="$(get_authenticated_repo_url)"
    
    local remote_sha
    if ! remote_sha="$(GIT_TERMINAL_PROMPT=0 git ls-remote "$auth_url" "refs/heads/${GIT_BRANCH}" 2>/dev/null | awk '{print $1}')" || [[ -z "$remote_sha" ]]; then
        log_event "ERROR" "check_failed" "Failed to retrieve remote revision from GitHub (offline or invalid credentials)"
        return 2
    fi

    local current_sha
    current_sha="$(get_current_revision)"

    echo "Node ID:          $NODE_ID"
    echo "Current Revision: $current_sha"
    echo "Latest Revision:  $remote_sha"

    if [[ "$current_sha" == "$remote_sha" ]]; then
        echo "Status:           UP_TO_DATE"
        log_event "INFO" "check_completed" "Node is up-to-date at revision ${remote_sha}"
        return 0
    else
        echo "Status:           UPDATE_AVAILABLE"
        log_event "INFO" "check_completed" "New revision available: ${remote_sha} (current: ${current_sha})"
        return 1
    fi
}

# ------------------------------------------------------------------------------
# Action: Health Check
# ------------------------------------------------------------------------------
action_health() {
    local target_ip="127.0.0.1"
    local passed=0
    local total=4

    log_event "INFO" "health_check_started" "Running comprehensive DNS data-plane health checks"

    # 1. Localhost Basic Query
    if dig "@${target_ip}" -p 53 localhost +short +time="${HEALTH_CHECK_TIMEOUT}" +tries=1 >/dev/null 2>&1 || \
       dig "@${target_ip}" -p 53 example.com +short +time="${HEALTH_CHECK_TIMEOUT}" +tries=2 >/dev/null 2>&1; then
        passed=$(( passed + 1 ))
    else
        log_event "ERROR" "health_probe_failed" "Localhost basic query probe failed"
    fi

    # 2. DNSSEC Validation Probe (Authenticated Data flag expected)
    local dnssec_resp
    dnssec_resp="$(dig "@${target_ip}" -p 53 cloudflare.com +dnssec +time="${HEALTH_CHECK_TIMEOUT}" +tries=2 2>/dev/null || true)"
    if echo "$dnssec_resp" | grep -q "flags:.*ad.*QUERY"; then
        passed=$(( passed + 1 ))
    else
        log_event "WARN" "health_probe_warn" "DNSSEC AD bit check failed or unverified"
    fi

    # 3. DNSSEC Bogus Rejection Probe (Must return SERVFAIL)
    local bogus_resp
    bogus_resp="$(dig "@${target_ip}" -p 53 dnssec-failed.org +dnssec +time="${HEALTH_CHECK_TIMEOUT}" +tries=2 2>/dev/null || true)"
    if echo "$bogus_resp" | grep -q "status: SERVFAIL"; then
        passed=$(( passed + 1 ))
    else
        log_event "WARN" "health_probe_warn" "Bogus DNSSEC domain was not rejected with SERVFAIL"
    fi

    # 4. Commercial CDN Endpoint Probe (Shopee / Akamai CDN)
    if dig "@${target_ip}" -p 53 cf.shopee.co.id +short +time="${HEALTH_CHECK_TIMEOUT}" +tries=2 >/dev/null 2>&1 || \
       dig "@${target_ip}" -p 53 google.com +short +time="${HEALTH_CHECK_TIMEOUT}" +tries=2 >/dev/null 2>&1; then
        passed=$(( passed + 1 ))
    else
        log_event "ERROR" "health_probe_failed" "CDN / Public resolution probe failed"
    fi

    if [[ "$passed" -ge 3 ]]; then
        log_event "INFO" "health_check_passed" "DNS Health Check: PASS (${passed}/${total} probes succeeded)"
        echo "DNS Health Check: PASS (${passed}/${total} probes succeeded)"
        return 0
    else
        log_event "ERROR" "health_check_failed" "DNS Health Check: FAIL (${passed}/${total} probes succeeded)"
        echo "DNS Health Check: FAIL (${passed}/${total} probes succeeded)"
        return 1
    fi
}

# ------------------------------------------------------------------------------
# Action: Staging & Validation
# ------------------------------------------------------------------------------
validate_candidate_config() {
    local release_dir="$1"
    local stage_dir="$2"

    log_event "INFO" "validation_started" "Validating candidate release files and Unbound configuration"

    # Check required files in release
    local required_paths=("conf/unbound-base.conf" "pkg" "scripts")
    for req in "${required_paths[@]}"; do
        if [[ ! -e "${release_dir}/${req}" ]]; then
            log_event "ERROR" "validation_failed" "Missing required repository path: ${req}"
            return 1
        fi
    done

    # Prepare staging candidate directory
    rm -rf "$stage_dir"
    mkdir -p "$stage_dir"
    cp -r /etc/unbound/* "$stage_dir/" 2>/dev/null || true
    
    # Overwrite or update configuration with candidate release files
    if [[ -d "${release_dir}/conf/conf.d" ]]; then
        mkdir -p "${stage_dir}/conf.d"
        cp -r "${release_dir}/conf/conf.d/"* "${stage_dir}/conf.d/" 2>/dev/null || true
    fi

    # If the release contains an explicit unbound.conf or compiled netora-cli
    if [[ -f "${release_dir}/conf/unbound.conf" ]]; then
        cp "${release_dir}/conf/unbound.conf" "${stage_dir}/unbound.conf"
    elif [[ -x "${release_dir}/bin/netora-cli" ]]; then
        "${release_dir}/bin/netora-cli" -mode render -write-dir "$stage_dir" -node-id "$NODE_ID" 2>/dev/null || true
    fi

    # Apply node-specific overlays (if exists)
    if [[ -d "$OVERLAYS_DIR" ]] && [[ "$(ls -A "$OVERLAYS_DIR" 2>/dev/null)" ]]; then
        log_event "INFO" "applying_overlays" "Applying node-specific configuration overlays from ${OVERLAYS_DIR}"
        cp -r "${OVERLAYS_DIR}/"* "$stage_dir/" 2>/dev/null || true
    fi

    # 1. Anti-Open Resolver Guardrail: Reject any accidental 0.0.0.0/0 allow
    if grep -r -E "access-control:[[:space:]]*0\.0\.0\.0/0[[:space:]]+allow" "$stage_dir" >/dev/null 2>&1; then
        log_event "ERROR" "validation_failed" "CRITICAL: Candidate configuration contains '0.0.0.0/0 allow' (Open Resolver detected!). Aborting."
        return 1
    fi

    # 2. Syntax Validation with unbound-checkconf
    # Rewrite include paths in temporary test file so candidate conf.d files are validated in staging
    sed "s|/etc/unbound/conf\.d|${stage_dir}/conf.d|g" "${stage_dir}/unbound.conf" > "${stage_dir}/unbound-test.conf"
    local checkconf_out
    if ! checkconf_out="$(unbound-checkconf "${stage_dir}/unbound-test.conf" 2>&1)"; then
        log_event "ERROR" "validation_failed" "unbound-checkconf syntax error in candidate: ${checkconf_out}"
        return 1
    fi

    log_event "INFO" "validation_passed" "Configuration and security validation passed successfully"
    return 0
}

# ------------------------------------------------------------------------------
# Action: Backup & Retention
# ------------------------------------------------------------------------------
create_backup() {
    local current_rev="$1"
    local timestamp
    timestamp="$(date -u +"%Y%m%d_%H%M%S")"
    local backup_path="${BACKUPS_DIR}/${current_rev}_${timestamp}"

    log_event "INFO" "backup_started" "Creating rollback point at ${backup_path}"
    mkdir -p "$backup_path"
    
    # Backup active /etc/unbound and state
    cp -r /etc/unbound/* "$backup_path/" 2>/dev/null || true
    if [[ -f "$STATE_FILE" ]]; then
        cp "$STATE_FILE" "${backup_path}/update-state.json"
    fi

    echo "$backup_path"

    # Prune old backups beyond retention
    local total_backups
    total_backups="$(find "$BACKUPS_DIR" -mindepth 1 -maxdepth 1 -type d | wc -l)"
    if [[ "$total_backups" -gt "$BACKUP_RETENTION" ]]; then
        find "$BACKUPS_DIR" -mindepth 1 -maxdepth 1 -type d -printf '%T+ %p\n' | \
            sort | head -n "$(( total_backups - BACKUP_RETENTION ))" | \
            awk '{print $2}' | xargs -r rm -rf
        log_event "INFO" "backup_pruned" "Pruned older backups to preserve retention limit (${BACKUP_RETENTION})"
    fi
}

# ------------------------------------------------------------------------------
# Action: Rollback
# ------------------------------------------------------------------------------
action_rollback() {
    acquire_lock
    local specific_backup="${1:-}"
    local backup_target=""

    if [[ -n "$specific_backup" && -d "$specific_backup" ]]; then
        backup_target="$specific_backup"
    elif [[ -n "$specific_backup" && -d "${BACKUPS_DIR}/${specific_backup}" ]]; then
        backup_target="${BACKUPS_DIR}/${specific_backup}"
    else
        # Pick the most recent backup
        backup_target="$(find "$BACKUPS_DIR" -mindepth 1 -maxdepth 1 -type d -printf '%T+ %p\n' | sort -r | head -n 1 | awk '{print $2}')"
    fi

    if [[ -z "$backup_target" || ! -d "$backup_target" ]]; then
        log_event "ERROR" "rollback_failed" "No valid backup available to perform rollback"
        echo "Error: No backup found."
        return 1
    fi

    log_event "WARN" "rollback_started" "Restoring configuration from backup: ${backup_target}"

    # Clean out dynamic conf.d to eliminate any orphan broken candidate files
    rm -rf /etc/unbound/conf.d
    # Atomic restore to /etc/unbound
    cp -r "${backup_target}/"* /etc/unbound/ 2>/dev/null || true
    
    # Reload Unbound
    if ! unbound-control reload >/dev/null 2>&1; then
        systemctl restart unbound || true
    fi

    # Verify recovery
    if action_health >/dev/null 2>&1; then
        local curr_rev
        curr_rev="$(get_current_revision)"
        update_state_file "ROLLED_BACK" "restored-from-${backup_target##*/}" "$curr_rev" "Automatic rollback triggered" "PASS" 0
        log_event "INFO" "rollback_completed" "Rollback executed successfully. DNS service restored and healthy."
        echo "Rollback successful. DNS restored from ${backup_target}."
        return 0
    else
        log_event "ERROR" "rollback_failed" "Rollback completed but health check failed!"
        echo "Rollback completed with health check failures. Immediate manual intervention required."
        return 1
    fi
}

# ------------------------------------------------------------------------------
# Action: Update Execution
# ------------------------------------------------------------------------------
action_update() {
    acquire_lock
    local start_time
    start_time="$(date +%s)"
    local force_flag="false"
    local requested_rev=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --force) force_flag="true"; shift ;;
            --revision) requested_rev="$2"; shift 2 ;;
            *) shift ;;
        esac
    done

    # Check Automation / Maintenance guardrails
    if [[ "$MAINTENANCE_MODE" == "true" ]]; then
        log_event "INFO" "update_skipped" "Update skipped: Node is currently in MAINTENANCE mode"
        echo "Maintenance mode is ON. Update skipped."
        return 0
    fi

    if [[ "$AUTO_UPDATE_ENABLED" != "true" && "$force_flag" != "true" ]]; then
        log_event "INFO" "update_skipped" "Update skipped: Auto-update is disabled (Use --force to override)"
        echo "Auto-update is disabled. Update skipped."
        return 0
    fi

    # Step 1: Check Target Revision
    local auth_url
    auth_url="$(get_authenticated_repo_url)"
    local target_sha="$requested_rev"

    if [[ -z "$target_sha" ]]; then
        if ! target_sha="$(GIT_TERMINAL_PROMPT=0 git ls-remote "$auth_url" "refs/heads/${GIT_BRANCH}" 2>/dev/null | awk '{print $1}')" || [[ -z "$target_sha" ]]; then
            log_event "ERROR" "update_failed" "Cannot reach GitHub or invalid credentials. Preserving current DNS configuration."
            local curr
            curr="$(get_current_revision)"
            update_state_file "OFFLINE_RETRY" "$curr" "$curr" "GitHub unreachable" "PASS" 0
            return 2
        fi
    fi

    local current_sha
    current_sha="$(get_current_revision)"

    if [[ "$current_sha" == "$target_sha" && "$force_flag" != "true" ]]; then
        log_event "INFO" "update_not_needed" "Installed revision (${current_sha}) matches target (${target_sha}). No update required."
        echo "Node is already up to date (${current_sha})."
        return 0
    fi

    log_event "INFO" "update_started" "Initiating update: ${current_sha} -> ${target_sha} on branch ${GIT_BRANCH}"

    # Step 2: Download / Fetch into Releases
    local target_release_dir="${RELEASES_DIR}/${target_sha}"
    local candidate_stage_dir="${STAGING_DIR}/${target_sha}"

    rm -rf "$target_release_dir"
    mkdir -p "$target_release_dir"

    if ! GIT_TERMINAL_PROMPT=0 git clone --depth=1 --branch "$GIT_BRANCH" "$auth_url" "$target_release_dir" >/dev/null 2>&1; then
        log_event "ERROR" "download_failed" "Failed to clone release ${target_sha} from GitHub. Discarding release."
        rm -rf "$target_release_dir"
        return 1
    fi
    log_event "INFO" "update_downloaded" "Successfully fetched release ${target_sha}"

    # Step 3: Validate candidate configuration
    if ! validate_candidate_config "$target_release_dir" "$candidate_stage_dir"; then
        log_event "ERROR" "update_aborted" "Candidate configuration validation failed. Active production configuration preserved."
        rm -rf "$candidate_stage_dir"
        local duration=$(( $(date +%s) - start_time ))
        update_state_file "VALIDATION_FAILED" "$current_sha" "$current_sha" "Configuration validation failed" "PASS" "$duration"
        echo "Update failed validation. Production DNS preserved."
        return 1
    fi

    # Step 4: Backup current active state
    local backup_point
    backup_point="$(create_backup "$current_sha")"

    # Step 5: Atomic Activation
    log_event "INFO" "activation_started" "Activating candidate configuration into /etc/unbound"
    # Copy validated files safely
    cp -r "${candidate_stage_dir}/"* /etc/unbound/
    rm -rf "$candidate_stage_dir"

    # Update current symlink
    ln -sfn "$target_release_dir" "${NETORA_BASE}/current"

    # Synchronize Web UI distribution if present in release
    if [[ -d "${target_release_dir}/deploy/ui" ]] && [[ "$(ls -A "${target_release_dir}/deploy/ui" 2>/dev/null)" ]]; then
        mkdir -p /var/www/netora-ui
        cp -r "${target_release_dir}/deploy/ui/"* /var/www/netora-ui/ 2>/dev/null || true
    fi

    # Ensure dynamic version.json is placed in Web UI and state directory
    local version_file="${target_release_dir}/version.json"
    local version_str="v1.0.44"
    local commit_cnt=44
    if [[ -f "$version_file" ]] && command -v jq >/dev/null 2>&1; then
        version_str="$(jq -r '.version // "v1.0.44"' "$version_file")"
        commit_cnt="$(jq -r '.commit_count // 44' "$version_file")"
    fi
    local short_rev="${target_sha:0:7}"

    mkdir -p /var/www/netora-ui /var/lib/netora-dns
    cat <<EOF > /var/www/netora-ui/version.json
{
  "version": "${version_str}",
  "commit_count": ${commit_cnt},
  "build": ${commit_cnt},
  "git_commit": "${short_rev}",
  "git_branch": "${GIT_BRANCH}",
  "node_id": "${NODE_ID}",
  "update_time": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF
    cp -f /var/www/netora-ui/version.json /var/lib/netora-dns/version.json 2>/dev/null || true

    # Update API binary if present
    if [[ -f "${target_release_dir}/deploy/bin/netora-api-linux-amd64" ]]; then
        cp -f "${target_release_dir}/deploy/bin/netora-api-linux-amd64" /usr/local/bin/netora-api 2>/dev/null && chmod +x /usr/local/bin/netora-api || true
        systemctl restart netora-api 2>/dev/null || true
    fi

    # Update CLI binary if present
    if [[ -f "${target_release_dir}/deploy/bin/netora-cli-linux-amd64" ]]; then
        cp -f "${target_release_dir}/deploy/bin/netora-cli-linux-amd64" /usr/local/bin/netora-cli 2>/dev/null && chmod +x /usr/local/bin/netora-cli || true
    fi

    # Step 6: Unbound Reload
    log_event "INFO" "reloading_unbound" "Reloading Unbound DNS service"
    if ! unbound-control reload >/dev/null 2>&1; then
        log_event "ERROR" "reload_failed" "unbound-control reload returned non-zero. Initiating immediate rollback."
        action_rollback "$backup_point"
        return 1
    fi

    # Step 7: Post-Activation Health Check
    sleep 1
    if ! action_health >/dev/null 2>&1; then
        log_event "ERROR" "health_check_failed" "Post-activation DNS health check failed! Triggering automatic rollback."
        action_rollback "$backup_point"
        return 1
    fi

    # Step 8: Finalize State
    local duration=$(( $(date +%s) - start_time ))
    update_state_file "SUCCESS" "$target_sha" "$current_sha" "none" "PASS" "$duration"
    log_event "INFO" "update_completed" "Update to revision ${target_sha} successfully completed in ${duration}s"
    echo "Update to ${target_sha} completed successfully in ${duration}s."
    return 0
}

# ------------------------------------------------------------------------------
# Action: Status
# ------------------------------------------------------------------------------
action_status() {
    local curr_rev="none"
    local prev_rev="none"
    local last_update="never"
    local status="UNKNOWN"
    local health="UNKNOWN"
    local has_rollback="NO"

    if [[ -f "$STATE_FILE" ]] && command -v jq >/dev/null 2>&1; then
        curr_rev="$(jq -r '.current_revision // "none"' "$STATE_FILE")"
        prev_rev="$(jq -r '.previous_revision // "none"' "$STATE_FILE")"
        last_update="$(jq -r '.update_time // "never"' "$STATE_FILE")"
        status="$(jq -r '.update_status // "UNKNOWN"' "$STATE_FILE")"
        health="$(jq -r '.health_status // "UNKNOWN"' "$STATE_FILE")"
    fi

    if [[ -d "$BACKUPS_DIR" ]] && [[ "$(find "$BACKUPS_DIR" -mindepth 1 -maxdepth 1 -type d | wc -l)" -gt 0 ]]; then
        has_rollback="YES"
    fi

    local latest_rev="querying..."
    local auth_url
    auth_url="$(get_authenticated_repo_url)"
    latest_rev="$(GIT_TERMINAL_PROMPT=0 git ls-remote "$auth_url" "refs/heads/${GIT_BRANCH}" 2>/dev/null | awk '{print $1}')" || latest_rev="unavailable"

    echo "================================================================================"
    echo "                       Netora DNS Update Status                                 "
    echo "================================================================================"
    echo "Node ID:                  ${NODE_ID}"
    echo "Channel:                  ${CHANNEL}"
    echo "Auto-Update:              ${AUTO_UPDATE_ENABLED}"
    echo "Maintenance Mode:         ${MAINTENANCE_MODE}"
    echo "Current Revision:         ${curr_rev}"
    echo "Latest Available:         ${latest_rev}"
    echo "Status:                   ${status}"
    echo "Last Update Time:         ${last_update}"
    echo "Previous Revision:        ${prev_rev}"
    echo "Last Health Check:        ${health}"
    echo "Rollback Available:       ${has_rollback}"
    echo "Backups Retained:         $(find "$BACKUPS_DIR" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)"
    echo "Log File:                 ${LOG_FILE}"
    echo "================================================================================"
}

# ------------------------------------------------------------------------------
# Entrypoint Router
# ------------------------------------------------------------------------------
case "${1:-status}" in
    status)
        action_status
        ;;
    check)
        action_check
        ;;
    update)
        shift
        action_update "$@"
        ;;
    rollback)
        shift
        action_rollback "${1:-}"
        ;;
    health)
        action_health
        ;;
    version)
        echo "Netora DNS Updater v${UPDATER_VERSION}"
        echo "Installed Revision: $(get_current_revision)"
        ;;
    maintenance)
        shift
        case "${1:-status}" in
            on)
                sed -i 's/^MAINTENANCE_MODE=.*/MAINTENANCE_MODE="true"/' "$CONFIG_FILE" 2>/dev/null || true
                log_event "INFO" "maintenance_enabled" "Maintenance mode enabled by operator"
                echo "Maintenance mode is now ON."
                ;;
            off)
                sed -i 's/^MAINTENANCE_MODE=.*/MAINTENANCE_MODE="false"/' "$CONFIG_FILE" 2>/dev/null || true
                log_event "INFO" "maintenance_disabled" "Maintenance mode disabled by operator"
                echo "Maintenance mode is now OFF."
                ;;
            *)
                echo "Usage: netora-dns-update maintenance <on|off>"
                ;;
        esac
        ;;
    pause)
        sed -i 's/^AUTO_UPDATE_ENABLED=.*/AUTO_UPDATE_ENABLED="false"/' "$CONFIG_FILE" 2>/dev/null || true
        log_event "INFO" "auto_update_paused" "Auto-update paused by operator"
        echo "Auto-update paused."
        ;;
    resume)
        sed -i 's/^AUTO_UPDATE_ENABLED=.*/AUTO_UPDATE_ENABLED="true"/' "$CONFIG_FILE" 2>/dev/null || true
        log_event "INFO" "auto_update_resumed" "Auto-update resumed by operator"
        echo "Auto-update resumed."
        ;;
    *)
        echo "Usage: netora-dns-update {status|check|update|rollback|health|version|maintenance|pause|resume}"
        exit 1
        ;;
esac
