#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(CDPATH='' cd -- "${SCRIPT_DIR}/../.." && pwd)"
CHANGELOG_PATH="${CHANGELOG_PATH:-${REPO_ROOT}/CHANGELOG.md}"
VERSION="${1:-}"
PREVIOUS_TAG="${2:-}"
TODAY="$(date +%F)"

usage() {
    cat <<'EOF'
Usage: scripts/changelog/generate.sh <version> [previous-tag]

Examples:
  scripts/changelog/generate.sh 0.1.1
  scripts/changelog/generate.sh 0.1.1 v0.1.0
EOF
}

require_git_repo() {
    git -C "${REPO_ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
        printf 'Error: not inside a git repository\n' >&2
        exit 1
    }
}

normalize_version() {
    if [[ -z "${VERSION}" ]]; then
        usage
        exit 1
    fi

    if [[ "${VERSION}" == v* ]]; then
        VERSION="${VERSION#v}"
    fi
}

detect_previous_tag() {
    if [[ -n "${PREVIOUS_TAG}" ]]; then
        return
    fi

    PREVIOUS_TAG="$(git -C "${REPO_ROOT}" tag --list 'v*' --sort=-v:refname | head -n 1 || true)"
}

commit_range() {
    if [[ -n "${PREVIOUS_TAG}" ]]; then
        printf '%s\n' "${PREVIOUS_TAG}..HEAD"
        return
    fi

    printf '%s\n' HEAD
}

collect_commits() {
    local range
    range="$(commit_range)"
    git -C "${REPO_ROOT}" log --format='%s' ${range}
}

trim_subject() {
    local subject="$1"
    subject="${subject#feat: }"
    subject="${subject#feature: }"
    subject="${subject#fix: }"
    subject="${subject#docs: }"
    subject="${subject#doc: }"
    subject="${subject#refactor: }"
    subject="${subject#perf: }"
    subject="${subject#test: }"
    subject="${subject#build: }"
    subject="${subject#ci: }"
    subject="${subject#chore: }"
    printf '%s\n' "${subject}"
}

classify_subject() {
    local subject_lower="$1"

    case "${subject_lower}" in
        feat:*|feature:*) printf '%s\n' Added ;;
        fix:*) printf '%s\n' Fixed ;;
        perf:*) printf '%s\n' Changed ;;
        refactor:*) printf '%s\n' Changed ;;
        docs:*|doc:*) printf '%s\n' Documentation ;;
        test:*) printf '%s\n' Tests ;;
        build:*|ci:*|chore:*) printf '%s\n' Maintenance ;;
        revert:*) printf '%s\n' Changed ;;
        *) printf '%s\n' Changed ;;
    esac
}

append_section() {
    local heading="$1"
    local file="$2"

    if [[ ! -s "${file}" ]]; then
        return
    fi

    printf '### %s\n' "${heading}"
    sed 's/^/- /' "${file}"
    printf '\n'
}

generate_release_notes() {
    local temp_dir
    local added_file
    local fixed_file
    local changed_file
    local docs_file
    local tests_file
    local maintenance_file
    local subject
    local lower
    local category
    local trimmed

    temp_dir="$(mktemp -d)"
    added_file="${temp_dir}/added"
    fixed_file="${temp_dir}/fixed"
    changed_file="${temp_dir}/changed"
    docs_file="${temp_dir}/docs"
    tests_file="${temp_dir}/tests"
    maintenance_file="${temp_dir}/maintenance"

    while IFS= read -r subject; do
        [[ -n "${subject}" ]] || continue
        lower="$(printf '%s' "${subject}" | tr '[:upper:]' '[:lower:]')"
        category="$(classify_subject "${lower}")"
        trimmed="$(trim_subject "${subject}")"
        case "${category}" in
            Added) printf '%s\n' "${trimmed}" >>"${added_file}" ;;
            Fixed) printf '%s\n' "${trimmed}" >>"${fixed_file}" ;;
            Changed) printf '%s\n' "${trimmed}" >>"${changed_file}" ;;
            Documentation) printf '%s\n' "${trimmed}" >>"${docs_file}" ;;
            Tests) printf '%s\n' "${trimmed}" >>"${tests_file}" ;;
            Maintenance) printf '%s\n' "${trimmed}" >>"${maintenance_file}" ;;
        esac
    done < <(collect_commits)

    {
        printf '## [%s] - %s\n\n' "${VERSION}" "${TODAY}"
        append_section Added "${added_file}"
        append_section Fixed "${fixed_file}"
        append_section Changed "${changed_file}"
        append_section Documentation "${docs_file}"
        append_section Tests "${tests_file}"
        append_section Maintenance "${maintenance_file}"
    } >"${temp_dir}/release-notes.md"

    cat "${temp_dir}/release-notes.md"
    rm -rf "${temp_dir}"
}

write_changelog() {
    local temp_file
    local header_file
    local release_file
    local existing_file

    temp_file="$(mktemp)"
    header_file="$(mktemp)"
    release_file="$(mktemp)"
    existing_file="$(mktemp)"

    cat >"${header_file}" <<'EOF'
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

EOF

    generate_release_notes >"${release_file}"

    cat "${header_file}" >"${temp_file}"
    cat "${release_file}" >>"${temp_file}"
    printf '\n' >>"${temp_file}"

    if [[ -f "${CHANGELOG_PATH}" ]]; then
        awk -v version="${VERSION}" '
            BEGIN {
                skip = 1
                drop = 0
            }
            /^## \[/ {
                if ($0 == "## [Unreleased]") {
                    next
                }
                skip = 0
                if ($0 == "## [" version "] - " substr($0, index($0, "- ") + 2)) {
                    drop = 1
                    next
                }
                drop = 0
            }
            !skip && !drop {
                print
            }
        ' "${CHANGELOG_PATH}" >"${existing_file}"
        cat "${existing_file}" >>"${temp_file}"
    fi

    mv "${temp_file}" "${CHANGELOG_PATH}"
    rm -f "${header_file}" "${release_file}" "${existing_file}"
}

main() {
    require_git_repo
    normalize_version
    detect_previous_tag
    write_changelog

    printf 'Updated %s for version %s\n' "${CHANGELOG_PATH}" "${VERSION}"
    if [[ -n "${PREVIOUS_TAG}" ]]; then
        printf 'Commit range: %s..HEAD\n' "${PREVIOUS_TAG}"
    else
        printf 'Commit range: initial history to HEAD\n'
    fi
}

main "$@"
