#!/usr/bin/env bash

set -euo pipefail

APP_USER="${APP_USER:-pruvon}"
APP_GROUP="${APP_GROUP:-pruvon}"
APP_INSTALL_DIR="${APP_INSTALL_DIR:-/opt/pruvon}"
APP_RUNTIME_DIR="${APP_RUNTIME_DIR:-/var/lib/pruvon}"
APP_BINARY_DEST="${APP_BINARY_DEST:-${APP_INSTALL_DIR}/pruvon}"
APP_SYMLINK="${APP_SYMLINK:-/usr/local/bin/pruvon}"
CONFIG_PATH="${CONFIG_PATH:-/etc/pruvon.yml}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/dokku/data/pruvon-backup}"
LOG_DIR="${LOG_DIR:-/var/log/pruvon}"
SYSTEMD_UNIT_PATH="${SYSTEMD_UNIT_PATH:-/etc/systemd/system/pruvon.service}"
SUDOERS_PATH="${SUDOERS_PATH:-/etc/sudoers.d/pruvon}"
LOGROTATE_PATH="${LOGROTATE_PATH:-/etc/logrotate.d/pruvon}"
CRON_PATH="${CRON_PATH:-/etc/cron.daily/pruvon-backup}"

PURGE=0
REMOVE_USER=0

log() {
    printf '[uninstall] %s\n' "$*"
}

die() {
    printf '[uninstall] Error: %s\n' "$*" >&2
    exit 1
}

require_root() {
    if [[ "$(id -u)" -ne 0 ]]; then
        die "this script must run as root"
    fi
}

usage() {
    cat <<'EOF'
Usage: ./uninstall.sh [--purge] [--remove-user]

Options:
  --purge        Remove config, logs, backup data, and runtime state.
  --remove-user  Remove the pruvon system user and group after uninstall.
  -h, --help     Show this help text.
EOF
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --purge)
                PURGE=1
                ;;
            --remove-user)
                REMOVE_USER=1
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                die "unknown argument: $1"
                ;;
        esac
        shift
    done
}

stop_service() {
    if systemctl list-unit-files | grep -Fq 'pruvon.service'; then
        systemctl disable --now pruvon >/dev/null 2>&1 || true
    elif [[ -f "${SYSTEMD_UNIT_PATH}" ]]; then
        systemctl stop pruvon >/dev/null 2>&1 || true
    fi

    rm -f "${SYSTEMD_UNIT_PATH}"
    systemctl daemon-reload
}

remove_installed_files() {
    rm -f "${CRON_PATH}"
    rm -f "${LOGROTATE_PATH}"
    rm -f "${SUDOERS_PATH}"
    rm -f "${APP_SYMLINK}"
    rm -f "${APP_BINARY_DEST}"
    rmdir "${APP_INSTALL_DIR}" 2>/dev/null || true
}

purge_data() {
    [[ ${PURGE} -eq 1 ]] || return 0

    rm -f "${CONFIG_PATH}"
    rm -rf "${LOG_DIR}"
    rm -rf "${APP_RUNTIME_DIR}"
    rm -rf "${BACKUP_DIR}"
}

remove_user_if_requested() {
    [[ ${REMOVE_USER} -eq 1 ]] || return 0

    if id -u "${APP_USER}" >/dev/null 2>&1; then
        userdel "${APP_USER}" >/dev/null 2>&1 || true
    fi

    if getent group "${APP_GROUP}" >/dev/null 2>&1; then
        groupdel "${APP_GROUP}" >/dev/null 2>&1 || true
    fi
}

print_summary() {
    printf '\n'
    printf 'Pruvon uninstall completed.\n'
    if [[ ${PURGE} -eq 0 ]]; then
        printf 'Preserved: %s, %s, %s\n' "${CONFIG_PATH}" "${LOG_DIR}" "${BACKUP_DIR}"
        printf 'Run again with --purge to remove persistent data.\n'
    fi
    if [[ ${REMOVE_USER} -eq 0 ]]; then
        printf 'System user preserved: %s\n' "${APP_USER}"
        printf 'Run again with --remove-user to delete the service account.\n'
    fi
}

main() {
    require_root
    parse_args "$@"

    log "stopping and removing systemd unit"
    stop_service

    log "removing installed files"
    remove_installed_files

    if [[ ${PURGE} -eq 1 ]]; then
        log "purging config and data"
    fi
    purge_data

    if [[ ${REMOVE_USER} -eq 1 ]]; then
        log "removing service user"
    fi
    remove_user_if_requested

    print_summary
}

main "$@"
