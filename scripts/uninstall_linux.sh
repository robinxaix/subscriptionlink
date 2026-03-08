#!/usr/bin/env bash
set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

readonly CADDY_CONFIG_PATH="/etc/caddy/Caddyfile"
readonly XRAY_CONFIG_PATH="/usr/local/etc/xray/config.json"
readonly SUB_DATA_DIR="/usr/local/etc/subscriptionlink"
readonly SUB_BIN_PATH="/usr/local/bin/subscriptionlink"

readonly CADDY_UNIT_PATH="/etc/systemd/system/caddy.service"
readonly XRAY_UNIT_PATH="/etc/systemd/system/xray.service"
readonly SUB_UNIT_PATH="/etc/systemd/system/subscriptionlink.service"
readonly MANAGED_MARKER="# Managed by subscriptionlink deploy script"

YES=0
PURGE_DATA=0
REMOVE_CADDY_CONFIG=0
REMOVE_XRAY_CONFIG=0

log() {
  printf '[uninstall] %s\n' "$*"
}

fail() {
  printf '[uninstall] error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  sudo ./scripts/uninstall_linux.sh [options]

Options:
  -y, --yes                 Run non-interactively and skip confirmation
      --purge-data          Remove /usr/local/etc/subscriptionlink entirely
      --remove-caddy-config Remove /etc/caddy/Caddyfile
      --remove-xray-config  Remove /usr/local/etc/xray/config.json
      --full-clean          Equivalent to --purge-data --remove-caddy-config --remove-xray-config
  -h, --help                Show this help message

Default behavior only removes subscriptionlink itself and any Caddy/Xray systemd units
that were created by this repository's deploy script. Installed Caddy/Xray packages are
left in place.
EOF
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "please run this script with sudo"
  fi
}

require_linux() {
  [[ "$(uname -s)" == "Linux" ]] || fail "this script only supports Linux"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -y|--yes)
        YES=1
        shift
        ;;
      --purge-data)
        PURGE_DATA=1
        shift
        ;;
      --remove-caddy-config)
        REMOVE_CADDY_CONFIG=1
        shift
        ;;
      --remove-xray-config)
        REMOVE_XRAY_CONFIG=1
        shift
        ;;
      --full-clean)
        PURGE_DATA=1
        REMOVE_CADDY_CONFIG=1
        REMOVE_XRAY_CONFIG=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "unknown option: $1"
        ;;
    esac
  done
}

confirm_or_abort() {
  local answer

  if [[ "${YES}" -eq 1 ]]; then
    return
  fi

  cat <<EOF
About to uninstall subscriptionlink integration with:
  Remove subscriptionlink binary: ${SUB_BIN_PATH}
  Remove subscriptionlink unit: ${SUB_UNIT_PATH}
  Purge data dir: ${PURGE_DATA}
  Remove Caddy config: ${REMOVE_CADDY_CONFIG}
  Remove Xray config: ${REMOVE_XRAY_CONFIG}
EOF
  read -r -p "Continue? [y/N] " answer
  case "${answer}" in
    y|Y|yes|YES)
      ;;
    *)
      fail "aborted"
      ;;
  esac
}

has_systemd() {
  command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]
}

unit_exists() {
  has_systemd && systemctl cat "$1" >/dev/null 2>&1
}

stop_and_disable_service() {
  local service="$1"

  if ! unit_exists "${service}"; then
    return
  fi

  if systemctl is-active --quiet "${service}"; then
    log "stopping ${service}"
    systemctl stop "${service}" || true
  fi

  if systemctl is-enabled --quiet "${service}" 2>/dev/null; then
    log "disabling ${service}"
    systemctl disable "${service}" >/dev/null || true
  fi
}

is_managed_unit() {
  local unit_path="$1"
  [[ -f "${unit_path}" ]] && grep -Fq "${MANAGED_MARKER}" "${unit_path}"
}

remove_file_if_exists() {
  local path="$1"
  if [[ -e "${path}" ]]; then
    log "removing ${path}"
    rm -f "${path}"
  fi
}

remove_dir_if_exists() {
  local path="$1"
  if [[ -e "${path}" ]]; then
    log "removing ${path}"
    rm -rf "${path}"
  fi
}

main() {
  parse_args "$@"
  require_root
  require_linux
  confirm_or_abort

  stop_and_disable_service subscriptionlink
  remove_file_if_exists "${SUB_UNIT_PATH}"
  remove_file_if_exists "${SUB_BIN_PATH}"

  if is_managed_unit "${CADDY_UNIT_PATH}"; then
    stop_and_disable_service caddy
    remove_file_if_exists "${CADDY_UNIT_PATH}"
  fi

  if is_managed_unit "${XRAY_UNIT_PATH}"; then
    stop_and_disable_service xray
    remove_file_if_exists "${XRAY_UNIT_PATH}"
  fi

  if [[ "${REMOVE_CADDY_CONFIG}" -eq 1 ]]; then
    remove_file_if_exists "${CADDY_CONFIG_PATH}"
  fi

  if [[ "${REMOVE_XRAY_CONFIG}" -eq 1 ]]; then
    remove_file_if_exists "${XRAY_CONFIG_PATH}"
  fi

  if [[ "${PURGE_DATA}" -eq 1 ]]; then
    remove_dir_if_exists "${SUB_DATA_DIR}"
  fi

  if has_systemd; then
    systemctl daemon-reload
  fi

  log "uninstall completed"
}

main "$@"
