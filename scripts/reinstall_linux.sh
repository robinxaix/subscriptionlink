#!/usr/bin/env bash
set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly DEPLOY_SCRIPT="${SCRIPT_DIR}/deploy_linux.sh"
readonly UNINSTALL_SCRIPT="${SCRIPT_DIR}/uninstall_linux.sh"

DOMAIN=""
YES=0
FULL_CLEAN=0
PURGE_DATA=0
REMOVE_CADDY_CONFIG=0
REMOVE_XRAY_CONFIG=0

fail() {
  printf '[reinstall] error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  sudo ./scripts/reinstall_linux.sh [options]

Options:
  -d, --domain DOMAIN      Set the public domain for redeploy
  -y, --yes                Run non-interactively and skip confirmation
      --purge-data         Remove /usr/local/etc/subscriptionlink before redeploy
      --remove-caddy-config
                           Remove /etc/caddy/Caddyfile before redeploy
      --remove-xray-config Remove /usr/local/etc/xray/config.json before redeploy
      --full-clean         Equivalent to --purge-data --remove-caddy-config --remove-xray-config
  -h, --help               Show this help message

Default behavior preserves existing data and only refreshes the deployment wiring.
EOF
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "please run this script with sudo"
  fi
}

parse_args() {
  local positional=()

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -d|--domain)
        [[ $# -ge 2 ]] || fail "missing value for $1"
        DOMAIN="$2"
        shift 2
        ;;
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
        FULL_CLEAN=1
        PURGE_DATA=1
        REMOVE_CADDY_CONFIG=1
        REMOVE_XRAY_CONFIG=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      --)
        shift
        while [[ $# -gt 0 ]]; do
          positional+=("$1")
          shift
        done
        ;;
      -*)
        fail "unknown option: $1"
        ;;
      *)
        positional+=("$1")
        shift
        ;;
    esac
  done

  if [[ "${#positional[@]}" -gt 1 ]]; then
    fail "too many positional arguments"
  fi
  if [[ -z "${DOMAIN}" && "${#positional[@]}" -eq 1 ]]; then
    DOMAIN="${positional[0]}"
  fi
}

main() {
  local uninstall_args=()
  local deploy_args=()

  parse_args "$@"
  require_root

  [[ -x "${UNINSTALL_SCRIPT}" ]] || fail "missing uninstall script: ${UNINSTALL_SCRIPT}"
  [[ -x "${DEPLOY_SCRIPT}" ]] || fail "missing deploy script: ${DEPLOY_SCRIPT}"

  if [[ "${YES}" -eq 1 ]]; then
    uninstall_args+=(--yes)
    deploy_args+=(--yes)
  fi
  if [[ -n "${DOMAIN}" ]]; then
    deploy_args+=(--domain "${DOMAIN}")
  fi
  if [[ "${FULL_CLEAN}" -eq 1 || "${PURGE_DATA}" -eq 1 ]]; then
    uninstall_args+=(--purge-data)
  fi
  if [[ "${FULL_CLEAN}" -eq 1 || "${REMOVE_CADDY_CONFIG}" -eq 1 ]]; then
    uninstall_args+=(--remove-caddy-config)
  fi
  if [[ "${FULL_CLEAN}" -eq 1 || "${REMOVE_XRAY_CONFIG}" -eq 1 ]]; then
    uninstall_args+=(--remove-xray-config)
  fi

  "${UNINSTALL_SCRIPT}" "${uninstall_args[@]}"
  "${DEPLOY_SCRIPT}" "${deploy_args[@]}"
}

main "$@"
