#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(CDPATH='' cd -- "${SCRIPT_DIR}/../.." && pwd)"

VERSION_INPUT="${1:-}"
PREVIOUS_TAG="${2:-}"
NOTES_FILE_INPUT="${3:-}"

TAG=""
VERSION=""
NOTES_FILE=""
NOTES_PATH=""

usage() {
    printf '%s\n' "Usage: scripts/release/local.sh <version> [previous-tag] [notes-file]"
    printf '%s\n' "Example: scripts/release/local.sh v0.1.1 v0.1.0"
}

require_clean_repo() {
    if [[ -n "$(git -C "${REPO_ROOT}" status --porcelain)" ]]; then
        printf '%s\n' 'Error: working tree must be clean before creating a release.' >&2
        exit 1
    fi
}

require_command() {
    local command_name="$1"
    command -v "${command_name}" >/dev/null 2>&1 || {
        printf 'Error: required command not found: %s\n' "${command_name}" >&2
        exit 1
    }
}

die() {
    printf 'Error: %s\n' "$1" >&2
    exit 1
}

normalize_version() {
    if [[ -z "${VERSION_INPUT}" ]]; then
        usage
        exit 1
    fi

    TAG="${VERSION_INPUT}"
    if [[ "${TAG}" != v* ]]; then
        TAG="v${TAG}"
    fi

    VERSION="${TAG#v}"

    if [[ ! "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        die 'version must be semver like v0.1.0'
    fi
}

ensure_tag_absent() {
    if git -C "${REPO_ROOT}" rev-parse --verify --quiet "refs/tags/${TAG}" >/dev/null; then
        die "tag already exists: ${TAG}"
    fi
}

update_version_files() {
    DIRTY_FILES=("${REPO_ROOT}/CHANGELOG.md" "${REPO_ROOT}/cmd/app/main.go" "${REPO_ROOT}/cmd/app/main_test.go")
    perl -0pi -e "s/var PruvonVersion = \"[^\"]+\"/var PruvonVersion = \"${VERSION}\"/" "${REPO_ROOT}/cmd/app/main.go"
    perl -0pi -e "s/expectedVersion := \"[^\"]+\"/expectedVersion := \"${VERSION}\"/" "${REPO_ROOT}/cmd/app/main_test.go"
}

extract_release_section() {
    awk -v version="${VERSION}" '
        BEGIN { in_section = 0 }
        $0 ~ ("^## \\[" version "\\] - ") { in_section = 1 }
        /^## \[/ && in_section && $0 !~ ("^## \\[" version "\\] - ") { exit }
        in_section { print }
    ' "${REPO_ROOT}/CHANGELOG.md"
}

trim_release_heading() {
    awk 'NR == 1 { next } { print }'
}

prepare_notes() {
    if [[ -n "${NOTES_FILE_INPUT}" ]]; then
        NOTES_PATH="${NOTES_FILE_INPUT}"
        if [[ ! -f "${NOTES_PATH}" ]]; then
            die "notes file not found: ${NOTES_PATH}"
        fi
        return
    fi

    NOTES_FILE="$(mktemp)"
    NOTES_PATH="${NOTES_FILE}"

    local release_section
    release_section="$(extract_release_section)"
    [[ -n "${release_section}" ]] || die "could not find CHANGELOG.md section for ${VERSION}"

    {
        printf 'Install/update on a Dokku host:\n\n'
        printf '```bash\n'
        printf 'curl -fsSL https://pruvon.dev/install.sh | sudo PRUVON_VERSION=%s bash\n' "${TAG}"
        printf '```\n\n'
        printf '%s\n' "${release_section}" | trim_release_heading
        printf '\nIncluded artifacts:\n'
        printf -- '- `pruvon-linux-amd64.tar.gz`\n'
        printf -- '- `pruvon-linux-arm64.tar.gz`\n'
        printf -- '- `checksums.txt`\n'
    } >"${NOTES_PATH}"
}

DIRTY_FILES=()

rollback_version_files() {
    if [[ ${#DIRTY_FILES[@]} -gt 0 ]]; then
        git -C "${REPO_ROOT}" checkout -- "${DIRTY_FILES[@]}" 2>/dev/null || true
    fi
}

cleanup() {
    if [[ -n "${NOTES_FILE}" && -f "${NOTES_FILE}" ]]; then
        rm -f "${NOTES_FILE}"
    fi
}

verify_gh_auth() {
    gh auth status >/dev/null
}

run_release_verification() {
    printf '%s\n' 'Running release verification (go vet, go test -race, golangci-lint)...'
    make -C "${REPO_ROOT}" vet
    (
        cd "${REPO_ROOT}"
        go test -v -race -coverprofile=coverage.out ./...
    )
    make -C "${REPO_ROOT}" lint
}

update_changelog() {
    bash "${REPO_ROOT}/scripts/changelog/generate.sh" "${VERSION}" "${PREVIOUS_TAG}"
}

commit_release() {
    git -C "${REPO_ROOT}" add CHANGELOG.md cmd/app/main.go cmd/app/main_test.go
    git -C "${REPO_ROOT}" commit -m "release: ${TAG}"
}

build_artifacts() {
    make -C "${REPO_ROOT}" dist VERSION="${VERSION}"
}

create_tag() {
    git -C "${REPO_ROOT}" tag -a "${TAG}" -m "${TAG}"
}

push_release_commit() {
    git -C "${REPO_ROOT}" push
}

wait_for_ci_success() {
    local commit_sha="$1"
    local run_id=""
    local attempts=0
    local max_attempts=60

    printf 'Waiting for CI workflow for %s\n' "${commit_sha}"
    while [[ -z "${run_id}" ]]; do
        run_id="$(gh run list --workflow ci.yml --commit "${commit_sha}" --json databaseId --jq '.[0].databaseId // empty')"
        if [[ -n "${run_id}" ]]; then
            break
        fi

        attempts=$((attempts + 1))
        if (( attempts >= max_attempts )); then
            die "timed out waiting for CI workflow for ${commit_sha}"
        fi

        sleep 5
    done

    gh run watch "${run_id}" --compact --exit-status
}

push_tag() {
    git -C "${REPO_ROOT}" push origin "${TAG}"
}

create_release() {
    (
        cd "${REPO_ROOT}"
        gh release create "${TAG}" \
            "${REPO_ROOT}/dist/pruvon-linux-amd64.tar.gz" \
            "${REPO_ROOT}/dist/pruvon-linux-arm64.tar.gz" \
            "${REPO_ROOT}/dist/checksums.txt" \
            --title "${TAG}" \
            --notes-file "${NOTES_PATH}"
    )
}

main() {
    trap cleanup EXIT
    trap rollback_version_files ERR

    require_command git
    require_command gh
    require_command make
    require_command perl
    require_command shasum

    normalize_version
    require_clean_repo
    ensure_tag_absent
    verify_gh_auth
    update_version_files
    update_changelog
    run_release_verification
    build_artifacts
    prepare_notes
    commit_release
    local release_commit_sha
    release_commit_sha="$(git -C "${REPO_ROOT}" rev-parse HEAD)"
    push_release_commit
    wait_for_ci_success "${release_commit_sha}"
    create_tag
    push_tag
    create_release

    printf 'Release published: %s\n' "${TAG}"
}

main "$@"
