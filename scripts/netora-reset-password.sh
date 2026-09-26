#!/usr/bin/env bash
# ==============================================================================
# Netora DNS - CLI Interactive & Automated Password Reset Utility
# ==============================================================================
set -e

USERNAME="${1}"
NEW_PASSWORD="${2}"

if [ -z "$USERNAME" ]; then
    echo "========================================================"
    echo "  Netora DNS - CLI Password Reset Utility"
    echo "========================================================"
    read -rp "Enter username to reset [admin]: " INPUT_USER
    USERNAME="${INPUT_USER:-admin}"
fi

if [ -z "$NEW_PASSWORD" ]; then
    read -rsp "Enter new password for '${USERNAME}' (min 6 characters): " NEW_PASSWORD
    echo ""
    read -rsp "Confirm new password: " CONFIRM_PASSWORD
    echo ""
    if [ "$NEW_PASSWORD" != "$CONFIRM_PASSWORD" ]; then
        echo "Error: Passwords do not match!" >&2
        exit 1
    fi
fi

if [ ${#NEW_PASSWORD} -lt 6 ]; then
    echo "Error: Password must be at least 6 characters long." >&2
    exit 1
fi

/usr/local/bin/netora-cli -mode reset-password -username "$USERNAME" -new-password "$NEW_PASSWORD"
