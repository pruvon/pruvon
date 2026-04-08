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

PRUVON_REPOSITORY="${PRUVON_REPOSITORY:-pruvon/pruvon}"
PRUVON_VERSION="${PRUVON_VERSION:-latest}"
PRUVON_LISTEN="${PRUVON_LISTEN:-}"
GITHUB_API_BASE="https://api.github.com/repos/${PRUVON_REPOSITORY}"
GITHUB_RELEASES_BASE="https://github.com/${PRUVON_REPOSITORY}/releases/download"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/${PRUVON_REPOSITORY}"

GENERATED_ADMIN_PASSWORD=""
WORK_DIR=""
RESOLVED_VERSION=""
DOWNLOADED_BINARY_SOURCE=""
DOWNLOADED_CONFIG_TEMPLATE=""
DOWNLOADED_CRON_SOURCE=""
DOWNLOADED_CHECKSUMS=""

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

cleanup() {
    if [[ -n "${WORK_DIR}" && -d "${WORK_DIR}" ]]; then
        rm -rf "${WORK_DIR}"
    fi
}

ensure_work_dir() {
    if [[ -n "${WORK_DIR}" ]]; then
        return
    fi

    WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pruvon-install.XXXXXX")"
}

download_file() {
    local url="$1"
    local destination="$2"

    if command_exists curl; then
        curl -fsSL "$url" -o "$destination"
        return
    fi

    if command_exists wget; then
        wget -qO "$destination" "$url"
        return
    fi

    die "curl or wget is required to download installation assets"
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

normalize_version() {
    local version="$1"

    if [[ -z "${version}" || "${version}" == "latest" ]]; then
        printf '%s\n' latest
        return
    fi

    if [[ "${version}" == v* ]]; then
        printf '%s\n' "${version}"
        return
    fi

    printf 'v%s\n' "${version}"
}

resolve_version() {
    local normalized_version
    local metadata_path

    if [[ -n "${RESOLVED_VERSION}" ]]; then
        printf '%s\n' "${RESOLVED_VERSION}"
        return
    fi

    normalized_version="$(normalize_version "${PRUVON_VERSION}")"
    if [[ "${normalized_version}" != latest ]]; then
        RESOLVED_VERSION="${normalized_version}"
        printf '%s\n' "${RESOLVED_VERSION}"
        return
    fi

    ensure_work_dir
    metadata_path="${WORK_DIR}/release-latest.json"
    download_file "${GITHUB_API_BASE}/releases/latest" "${metadata_path}"
    RESOLVED_VERSION="$(grep -m1 '"tag_name"' "${metadata_path}" | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"

    [[ -n "${RESOLVED_VERSION}" ]] || die "could not resolve the latest Pruvon release version"
    printf '%s\n' "${RESOLVED_VERSION}"
}

download_release_asset() {
    local asset_name="$1"
    local destination="$2"
    local version

    version="$(resolve_version)"
    download_file "${GITHUB_RELEASES_BASE}/${version}/${asset_name}" "${destination}"
}

download_versioned_source_file() {
    local relative_path="$1"
    local destination="$2"
    local version

    version="$(resolve_version)"
    download_file "${GITHUB_RAW_BASE}/${version}/${relative_path}" "${destination}"
}

ensure_checksums_file() {
    if [[ -n "${DOWNLOADED_CHECKSUMS}" && -f "${DOWNLOADED_CHECKSUMS}" ]]; then
        printf '%s\n' "${DOWNLOADED_CHECKSUMS}"
        return
    fi

    ensure_work_dir
    DOWNLOADED_CHECKSUMS="${WORK_DIR}/checksums.txt"
    download_release_asset "checksums.txt" "${DOWNLOADED_CHECKSUMS}"
    printf '%s\n' "${DOWNLOADED_CHECKSUMS}"
}

verify_release_asset() {
    local asset_name="$1"
    local asset_path="$2"
    local checksums_path
    local expected_checksum=""
    local actual_checksum=""
    local checksum=""
    local name=""

    command_exists sha256sum || die "sha256sum is required to verify downloaded release assets"

    checksums_path="$(ensure_checksums_file)"
    while read -r checksum name; do
        if [[ "${name}" == "${asset_name}" ]]; then
            expected_checksum="${checksum}"
            break
        fi
    done < "${checksums_path}"

    [[ -n "${expected_checksum}" ]] || die "could not find ${asset_name} in release checksums"

    read -r actual_checksum _ < <(sha256sum "${asset_path}")
    [[ "${actual_checksum}" == "${expected_checksum}" ]] || die "checksum verification failed for ${asset_name}"
}

download_release_binary() {
    local arch
    local archive_name
    local archive_path
    local extract_dir
    local extracted_binary

    if [[ -n "${DOWNLOADED_BINARY_SOURCE}" && -f "${DOWNLOADED_BINARY_SOURCE}" ]]; then
        printf '%s\n' "${DOWNLOADED_BINARY_SOURCE}"
        return
    fi

    arch="$(detect_arch)"
    [[ -n "${arch}" ]] || die "unsupported architecture: $(uname -m)"

    ensure_work_dir
    archive_name="pruvon-linux-${arch}.tar.gz"
    archive_path="${WORK_DIR}/${archive_name}"
    extract_dir="${WORK_DIR}/extract-${arch}"

    download_release_asset "${archive_name}" "${archive_path}"
    verify_release_asset "${archive_name}" "${archive_path}"

    mkdir -p "${extract_dir}"
    tar -xzf "${archive_path}" -C "${extract_dir}"

    extracted_binary="${extract_dir}/pruvon-linux-${arch}"
    [[ -f "${extracted_binary}" ]] || die "downloaded archive did not contain ${extracted_binary##*/}"

    DOWNLOADED_BINARY_SOURCE="${extracted_binary}"
    printf '%s\n' "${DOWNLOADED_BINARY_SOURCE}"
}

detect_binary_source() {
    if [[ -n "${PRUVON_BINARY:-}" ]]; then
        [[ -f "${PRUVON_BINARY}" ]] || die "PRUVON_BINARY points to a missing file: ${PRUVON_BINARY}"
        printf '%s\n' "${PRUVON_BINARY}"
        return
    fi

    download_release_binary
}

detect_config_template() {
    if [[ -n "${PRUVON_CONFIG_TEMPLATE:-}" ]]; then
        [[ -f "${PRUVON_CONFIG_TEMPLATE}" ]] || die "PRUVON_CONFIG_TEMPLATE points to a missing file: ${PRUVON_CONFIG_TEMPLATE}"
        printf '%s\n' "${PRUVON_CONFIG_TEMPLATE}"
        return
    fi

    if [[ -n "${DOWNLOADED_CONFIG_TEMPLATE}" && -f "${DOWNLOADED_CONFIG_TEMPLATE}" ]]; then
        printf '%s\n' "${DOWNLOADED_CONFIG_TEMPLATE}"
        return
    fi

    ensure_work_dir
    DOWNLOADED_CONFIG_TEMPLATE="${WORK_DIR}/pruvon.yml.example"
    download_versioned_source_file "pruvon.yml.example" "${DOWNLOADED_CONFIG_TEMPLATE}"
    printf '%s\n' "${DOWNLOADED_CONFIG_TEMPLATE}"
}

detect_cron_source() {
    if [[ -n "${PRUVON_CRON_SOURCE:-}" ]]; then
        [[ -f "${PRUVON_CRON_SOURCE}" ]] || die "PRUVON_CRON_SOURCE points to a missing file: ${PRUVON_CRON_SOURCE}"
        printf '%s\n' "${PRUVON_CRON_SOURCE}"
        return
    fi

    if [[ -n "${DOWNLOADED_CRON_SOURCE}" && -f "${DOWNLOADED_CRON_SOURCE}" ]]; then
        printf '%s\n' "${DOWNLOADED_CRON_SOURCE}"
        return
    fi

    ensure_work_dir
    DOWNLOADED_CRON_SOURCE="${WORK_DIR}/pruvon-backup"
    download_versioned_source_file "scripts/cron/pruvon-backup" "${DOWNLOADED_CRON_SOURCE}"
    printf '%s\n' "${DOWNLOADED_CRON_SOURCE}"
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

    die "unable to generate a bcrypt password hash. Install apache2-utils, whois, openssl with bcrypt support, php, or python3"
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
    local replaced_listen=0

    if [[ -f "${CONFIG_PATH}" ]]; then
        log "keeping existing config at ${CONFIG_PATH}"
        if [[ -n "${PRUVON_LISTEN}" ]]; then
            log "ignoring PRUVON_LISTEN because ${CONFIG_PATH} already exists"
        fi
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

        if [[ -n "${PRUVON_LISTEN}" && ${replaced_listen} -eq 0 && "${line}" =~ ^[[:space:]]*listen:[[:space:]] ]]; then
            printf '  listen: "%s"\n' "${PRUVON_LISTEN}" >>"${temp_file}"
            replaced_listen=1
            continue
        fi

        printf '%s\n' "${line}" >>"${temp_file}"
    done <"${template_path}"

    if [[ ${replaced_password} -ne 1 ]]; then
        rm -f "${temp_file}"
        die "could not locate admin password field in ${template_path}"
    fi

    if [[ -n "${PRUVON_LISTEN}" && ${replaced_listen} -ne 1 ]]; then
        rm -f "${temp_file}"
        die "could not locate pruvon.listen field in ${template_path}"
    fi

    install -o root -g "${APP_GROUP}" -m 0640 "${temp_file}" "${CONFIG_PATH}"
    rm -f "${temp_file}"
    GENERATED_ADMIN_PASSWORD="${admin_password}"
}

check_listen_address() {
    if systemctl is-active --quiet pruvon; then
        log "skipping listen availability check because pruvon service is already running"
        return
    fi

    log "checking configured listen address"
    sudo -u "${APP_USER}" "${APP_BINARY_DEST}" -config "${CONFIG_PATH}" -check-listen
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
Cmnd_Alias PRUVON_STORAGE = ${chown_bin} -R dokku\:dokku /var/lib/dokku/data/storage/*, ${chmod_bin} -R * /var/lib/dokku/data/storage/*
${APP_USER} ALL=(root) NOPASSWD: PRUVON_DOKKU, PRUVON_NGINX, PRUVON_STORAGE
EOF

    visudo -cf "${temp_file}" >/dev/null
    install -o root -g root -m 0440 "${temp_file}" "${SUDOERS_PATH}"
    rm -f "${temp_file}"
}

install_cron_script() {
    local cron_source

    cron_source="$(detect_cron_source)"
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
    if [[ -n "${RESOLVED_VERSION}" ]]; then
        printf 'Version: %s\n' "${RESOLVED_VERSION}"
    fi
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
    local needs_remote_assets=0

    require_root
    trap cleanup EXIT

    command_exists install || die "install command is required"
    command_exists systemctl || die "systemctl is required"
    command_exists base64 || die "base64 is required"
    command_exists dd || die "dd is required"
    command_exists mktemp || die "mktemp is required"
    command_exists tar || die "tar is required"

    if [[ -z "${PRUVON_BINARY:-}" || -z "${PRUVON_CONFIG_TEMPLATE:-}" || -z "${PRUVON_CRON_SOURCE:-}" ]]; then
        needs_remote_assets=1
    fi

    if [[ ${needs_remote_assets} -eq 1 ]]; then
        ensure_work_dir
        resolve_version >/dev/null
    fi

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

    check_listen_address

    log "reloading systemd and starting service"
    reload_and_start_service

    print_summary
}

main "$@"
