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

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
GENERATED_ADMIN_PASSWORD=""

log() {
    printf '[install] %s\n' "$*"
}

die() {
    printf '[install] Error: %s\n' "$*" >&2
    exit 1
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

require_root() {
    if [[ "$(id -u)" -ne 0 ]]; then
        die "this script must run as root"
    fi
}

detect_nologin_shell() {
    if [[ -x /usr/sbin/nologin ]]; then
        printf '%s\n' /usr/sbin/nologin
        return
    fi

    if [[ -x /sbin/nologin ]]; then
        printf '%s\n' /sbin/nologin
        return
    fi

    if [[ -x /usr/bin/false ]]; then
        printf '%s\n' /usr/bin/false
        return
    fi

    printf '%s\n' /bin/false
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)
            printf '%s\n' amd64
            ;;
        aarch64|arm64)
            printf '%s\n' arm64
            ;;
        *)
            printf '%s\n' ""
            ;;
    esac
}

detect_binary_source() {
    local arch

    if [[ -n "${PRUVON_BINARY:-}" ]]; then
        [[ -f "${PRUVON_BINARY}" ]] || die "PRUVON_BINARY points to a missing file: ${PRUVON_BINARY}"
        printf '%s\n' "${PRUVON_BINARY}"
        return
    fi

    arch="$(detect_arch)"

    for candidate in \
        "${SCRIPT_DIR}/pruvon" \
        "${SCRIPT_DIR}/pruvon-linux-${arch}" \
        "${SCRIPT_DIR}/dist/pruvon-linux-${arch}" \
        "${SCRIPT_DIR}/builds/pruvon-linux-${arch}"
    do
        if [[ -n "${candidate}" && -f "${candidate}" ]]; then
            printf '%s\n' "${candidate}"
            return
        fi
    done

    die "could not find a pruvon binary. Set PRUVON_BINARY or place a built binary next to install.sh"
}

detect_config_template() {
    local candidate

    if [[ -n "${PRUVON_CONFIG_TEMPLATE:-}" ]]; then
        [[ -f "${PRUVON_CONFIG_TEMPLATE}" ]] || die "PRUVON_CONFIG_TEMPLATE points to a missing file: ${PRUVON_CONFIG_TEMPLATE}"
        printf '%s\n' "${PRUVON_CONFIG_TEMPLATE}"
        return
    fi

    for candidate in \
        "${SCRIPT_DIR}/pruvon.yml.example" \
        "${SCRIPT_DIR}/config.yaml.example"
    do
        if [[ -f "${candidate}" ]]; then
            printf '%s\n' "${candidate}"
            return
        fi
    done

    die "could not find a config example file. Expected pruvon.yml.example or config.yaml.example"
}

generate_admin_password() {
    dd if=/dev/urandom bs=24 count=1 2>/dev/null | base64 | tr -d '\n'
    printf '\n'
}

hash_looks_like_bcrypt() {
    [[ "$1" =~ ^\$2[aby]\$ ]]
}

generate_bcrypt_hash() {
    local plain="$1"
    local hash=""
    local temp_dir=""
    local go_dir=""

    if command_exists htpasswd; then
        hash="$(htpasswd -bnBC 10 '' "${plain}" | tr -d ':\n' || true)"
        if hash_looks_like_bcrypt "${hash}"; then
            printf '%s\n' "${hash}"
            return
        fi
    fi

    if command_exists mkpasswd; then
        hash="$(mkpasswd -m bcrypt "${plain}" 2>/dev/null || true)"
        if hash_looks_like_bcrypt "${hash}"; then
            printf '%s\n' "${hash}"
            return
        fi
    fi

    if command_exists openssl && openssl passwd -help 2>&1 | grep -q -- '-bcrypt'; then
        hash="$(openssl passwd -bcrypt "${plain}" 2>/dev/null || true)"
        if hash_looks_like_bcrypt "${hash}"; then
            printf '%s\n' "${hash}"
            return
        fi
    fi

    if command_exists php; then
        hash="$(php -r 'echo password_hash($argv[1], PASSWORD_BCRYPT), PHP_EOL;' "${plain}" 2>/dev/null || true)"
        if hash_looks_like_bcrypt "${hash}"; then
            printf '%s\n' "${hash}"
            return
        fi
    fi

    if command_exists python3; then
        hash="$(PASSWORD="${plain}" python3 <<'PY' || true
import os
import sys

try:
    import crypt
except ImportError:
    sys.exit(1)

method = getattr(crypt, "METHOD_BLOWFISH", None)
if method is None:
    sys.exit(1)

print(crypt.crypt(os.environ["PASSWORD"], crypt.mksalt(method)))
PY
)"
        if hash_looks_like_bcrypt "${hash}"; then
            printf '%s\n' "${hash}"
            return
        fi
    fi

    if command_exists go && [[ -f "${SCRIPT_DIR}/go.mod" && -w "${SCRIPT_DIR}" ]]; then
        go_dir="$(mktemp -d "${SCRIPT_DIR}/.pruvon-install-go.XXXXXX")"
        cat >"${go_dir}/main.go" <<'EOF'
package main

import (
    "fmt"
    "os"

    "golang.org/x/crypto/bcrypt"
)

func main() {
    password := os.Getenv("PRUVON_ADMIN_PASSWORD")
    if password == "" {
        fmt.Fprintln(os.Stderr, "missing PRUVON_ADMIN_PASSWORD")
        os.Exit(1)
    }

    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }

    fmt.Println(string(hash))
}
EOF
        hash="$(cd "${SCRIPT_DIR}" && PRUVON_ADMIN_PASSWORD="${plain}" go run "${go_dir}/main.go" 2>/dev/null || true)"
        rm -rf "${go_dir}"
        if hash_looks_like_bcrypt "${hash}"; then
            printf '%s\n' "${hash}"
            return
        fi
    fi

    die "unable to generate a bcrypt password hash. Install apache2-utils, whois, php, or provide Go in the source checkout"
}

ensure_group() {
    local group_name="$1"

    if ! getent group "${group_name}" >/dev/null; then
        groupadd --system "${group_name}"
    fi
}

ensure_group_member() {
    local user_name="$1"
    local group_name="$2"

    if id -nG "${user_name}" | tr ' ' '\n' | grep -Fxq "${group_name}"; then
        return
    fi

    usermod -a -G "${group_name}" "${user_name}"
}

ensure_user() {
    local nologin_shell

    ensure_group "${APP_GROUP}"
    nologin_shell="$(detect_nologin_shell)"

    if ! id -u "${APP_USER}" >/dev/null 2>&1; then
        useradd \
            --system \
            --gid "${APP_GROUP}" \
            --home-dir /nonexistent \
            --no-create-home \
            --shell "${nologin_shell}" \
            "${APP_USER}"
    fi

    ensure_group "adm"
    getent group dokku >/dev/null || die "dokku group does not exist. Install Dokku before running this script"

    ensure_group_member "${APP_USER}" adm
    ensure_group_member "${APP_USER}" dokku

    if getent group docker >/dev/null; then
        ensure_group_member "${APP_USER}" docker
    fi
}

install_binary() {
    local binary_source

    binary_source="$(detect_binary_source)"

    install -d -m 0755 "${APP_INSTALL_DIR}"
    install -m 0755 "${binary_source}" "${APP_BINARY_DEST}"
    ln -sfn "${APP_BINARY_DEST}" "${APP_SYMLINK}"
}

prepare_directories() {
    install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 "${APP_RUNTIME_DIR}"
    install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0750 "${BACKUP_DIR}"
    install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0750 "${LOG_DIR}"
    install -o "${APP_USER}" -g "${APP_GROUP}" -m 0640 /dev/null "${LOG_DIR}/activity.log"
    install -o "${APP_USER}" -g "${APP_GROUP}" -m 0640 /dev/null "${LOG_DIR}/backup.log"
}

create_config_if_missing() {
    local template_path
    local admin_password
    local admin_hash
    local temp_file
    local replaced_password=0

    if [[ -f "${CONFIG_PATH}" ]]; then
        log "keeping existing config at ${CONFIG_PATH}"
        chown root:"${APP_GROUP}" "${CONFIG_PATH}"
        chmod 0640 "${CONFIG_PATH}"
        return
    fi

    template_path="$(detect_config_template)"
    admin_password="$(generate_admin_password)"
    admin_hash="$(generate_bcrypt_hash "${admin_password}")"
    temp_file="$(mktemp)"

    while IFS= read -r line || [[ -n "${line}" ]]; do
        if [[ ${replaced_password} -eq 0 && "${line}" =~ ^[[:space:]]*password:[[:space:]] ]]; then
            printf '  password: "%s"\n' "${admin_hash}" >>"${temp_file}"
            replaced_password=1
            continue
        fi

        printf '%s\n' "${line}" >>"${temp_file}"
    done <"${template_path}"

    if [[ ${replaced_password} -ne 1 ]]; then
        rm -f "${temp_file}"
        die "could not locate admin password field in ${template_path}"
    fi

    install -o root -g "${APP_GROUP}" -m 0640 "${temp_file}" "${CONFIG_PATH}"
    rm -f "${temp_file}"
    GENERATED_ADMIN_PASSWORD="${admin_password}"
}

install_systemd_unit() {
    local supplementary_groups="adm dokku"
    local temp_file

    if getent group docker >/dev/null; then
        supplementary_groups="adm dokku docker"
    fi

    temp_file="$(mktemp)"
    cat >"${temp_file}" <<EOF
[Unit]
Description=Pruvon server
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
SupplementaryGroups=${supplementary_groups}
WorkingDirectory=${APP_RUNTIME_DIR}
ExecStart=${APP_BINARY_DEST} -server -config ${CONFIG_PATH}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    install -o root -g root -m 0644 "${temp_file}" "${SYSTEMD_UNIT_PATH}"
    rm -f "${temp_file}"
}

install_sudoers() {
    local dokku_bin
    local chown_bin
    local chmod_bin
    local nginx_bin
    local systemctl_bin
    local temp_file

    dokku_bin="$(command -v dokku || true)"
    chown_bin="$(command -v chown || true)"
    chmod_bin="$(command -v chmod || true)"
    nginx_bin="$(command -v nginx || true)"
    systemctl_bin="$(command -v systemctl || true)"

    [[ -n "${dokku_bin}" ]] || die "dokku command not found"
    [[ -n "${chown_bin}" ]] || die "chown command not found"
    [[ -n "${chmod_bin}" ]] || die "chmod command not found"
    [[ -n "${nginx_bin}" ]] || die "nginx command not found"
    [[ -n "${systemctl_bin}" ]] || die "systemctl command not found"
    command_exists sudo || die "sudo is required"
    command_exists visudo || die "visudo is required to validate the sudoers file"

    temp_file="$(mktemp)"
    cat >"${temp_file}" <<EOF
Defaults:${APP_USER} !requiretty
Cmnd_Alias PRUVON_DOKKU = ${dokku_bin}, ${dokku_bin} *
Cmnd_Alias PRUVON_NGINX = ${nginx_bin}, ${nginx_bin} *, ${systemctl_bin} reload nginx, ${systemctl_bin} restart nginx, ${systemctl_bin} status nginx
Cmnd_Alias PRUVON_STORAGE = ${chown_bin} -R dokku:dokku /var/lib/dokku/data/storage/*, ${chmod_bin} -R * /var/lib/dokku/data/storage/*
${APP_USER} ALL=(root) NOPASSWD: PRUVON_DOKKU, PRUVON_NGINX, PRUVON_STORAGE
EOF

    visudo -cf "${temp_file}" >/dev/null
    install -o root -g root -m 0440 "${temp_file}" "${SUDOERS_PATH}"
    rm -f "${temp_file}"
}

install_cron_script() {
    local cron_source

    cron_source="${SCRIPT_DIR}/scripts/cron/pruvon-backup"
    [[ -f "${cron_source}" ]] || die "missing cron script at ${cron_source}"
    install -o root -g root -m 0755 "${cron_source}" "${CRON_PATH}"
}

install_logrotate_config() {
    local temp_file

    temp_file="$(mktemp)"
    cat >"${temp_file}" <<EOF
${LOG_DIR}/*.log {
    monthly
    rotate 12
    missingok
    notifempty
    compress
    delaycompress
    create 0640 ${APP_USER} ${APP_GROUP}
}
EOF
    install -o root -g root -m 0644 "${temp_file}" "${LOGROTATE_PATH}"
    rm -f "${temp_file}"
}

reload_and_start_service() {
    systemctl daemon-reload
    systemctl enable pruvon >/dev/null

    if systemctl is-active --quiet pruvon; then
        systemctl restart pruvon
    else
        systemctl start pruvon
    fi
}

print_summary() {
    printf '\n'
    printf 'Pruvon installation completed.\n'
    printf 'Binary: %s\n' "${APP_BINARY_DEST}"
    printf 'Config: %s\n' "${CONFIG_PATH}"
    printf 'Systemd unit: %s\n' "${SYSTEMD_UNIT_PATH}"
    printf 'Backup dir: %s\n' "${BACKUP_DIR}"
    printf 'Log dir: %s\n' "${LOG_DIR}"

    if [[ -n "${GENERATED_ADMIN_PASSWORD}" ]]; then
        printf 'Admin username: admin\n'
        printf 'Admin password: %s\n' "${GENERATED_ADMIN_PASSWORD}"
        printf 'Store this password now. It is not written anywhere in plaintext.\n'
    fi
}

main() {
    require_root

    command_exists install || die "install command is required"
    command_exists systemctl || die "systemctl is required"
    command_exists base64 || die "base64 is required"
    command_exists dd || die "dd is required"

    log "creating service user and group memberships"
    ensure_user

    log "installing binary"
    install_binary

    log "preparing runtime, backup, and log directories"
    prepare_directories

    log "creating config if needed"
    create_config_if_missing

    log "installing systemd unit"
    install_systemd_unit

    log "installing sudoers policy"
    install_sudoers

    log "installing backup cron job"
    install_cron_script

    log "installing logrotate config"
    install_logrotate_config

    log "reloading systemd and starting service"
    reload_and_start_service

    print_summary
}

main "$@"
