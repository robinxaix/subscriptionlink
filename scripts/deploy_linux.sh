#!/usr/bin/env bash
set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

readonly CADDY_TEMPLATE="${PROJECT_ROOT}/data/Caddyfile"
readonly XRAY_TEMPLATE="${PROJECT_ROOT}/data/xray.config"
readonly CLASH_TEMPLATE="${PROJECT_ROOT}/data/clash.yaml"
readonly PACKAGED_BIN_DIR="${PROJECT_ROOT}/bin"

readonly CADDY_CONFIG_DIR="/etc/caddy"
readonly CADDY_CONFIG_PATH="${CADDY_CONFIG_DIR}/Caddyfile"
readonly XRAY_CONFIG_DIR="/usr/local/etc/xray"
readonly XRAY_CONFIG_PATH="${XRAY_CONFIG_DIR}/config.json"
readonly SUB_DATA_DIR="/usr/local/etc/subscriptionlink"
readonly SUB_BIN_PATH="/usr/local/bin/subscriptionlink"

readonly CADDY_UNIT_PATH="/etc/systemd/system/caddy.service"
readonly XRAY_UNIT_PATH="/etc/systemd/system/xray.service"
readonly SUB_UNIT_PATH="/etc/systemd/system/subscriptionlink.service"
readonly MANAGED_MARKER="# Managed by subscriptionlink deploy script"

DOMAIN="${DOMAIN:-}"
YES=0

log() {
  printf '[deploy] %s\n' "$*"
}

fail() {
  printf '[deploy] error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  sudo ./scripts/deploy_linux.sh [options] [domain]

Examples:
  sudo ./scripts/deploy_linux.sh example.com
  sudo ./scripts/deploy_linux.sh --domain example.com --yes
  sudo DOMAIN=example.com ./scripts/deploy_linux.sh

Options:
  -d, --domain DOMAIN   Set the public domain without prompting
  -y, --yes             Run non-interactively and skip confirmation
  -h, --help            Show this help message
EOF
}

cleanup() {
  if [[ -n "${TMP_DIR:-}" && -d "${TMP_DIR}" ]]; then
    rm -rf "${TMP_DIR}"
  fi
}

trap cleanup EXIT

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "please run this script with sudo"
  fi
}

require_linux() {
  [[ "$(uname -s)" == "Linux" ]] || fail "this script only supports Linux"
}

require_systemd() {
  command -v systemctl >/dev/null 2>&1 || fail "systemctl is required"
  [[ -d /run/systemd/system ]] || fail "systemd does not appear to be the init system"
}

require_template_files() {
  [[ -f "${CADDY_TEMPLATE}" ]] || fail "missing template: ${CADDY_TEMPLATE}"
  [[ -f "${XRAY_TEMPLATE}" ]] || fail "missing template: ${XRAY_TEMPLATE}"
  [[ -f "${CLASH_TEMPLATE}" ]] || fail "missing template: ${CLASH_TEMPLATE}"
}

detect_pkg_manager() {
  if command -v apt-get >/dev/null 2>&1; then
    printf 'apt'
    return
  fi
  if command -v dnf >/dev/null 2>&1; then
    printf 'dnf'
    return
  fi
  if command -v yum >/dev/null 2>&1; then
    printf 'yum'
    return
  fi
  if command -v zypper >/dev/null 2>&1; then
    printf 'zypper'
    return
  fi
  printf 'unknown'
}

install_packages() {
  local manager="$1"
  shift

  case "${manager}" in
    apt)
      export DEBIAN_FRONTEND=noninteractive
      apt-get update
      apt-get install -y "$@"
      ;;
    dnf)
      dnf install -y "$@"
      ;;
    yum)
      yum install -y "$@"
      ;;
    zypper)
      zypper --non-interactive install "$@"
      ;;
    *)
      fail "unsupported package manager for installing: $*"
      ;;
  esac
}

ensure_basic_tools() {
  local manager="$1"
  local missing=()
  local tool
  for tool in curl tar; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
      missing+=("${tool}")
    fi
  done

  if [[ "${#missing[@]}" -eq 0 ]]; then
    return
  fi

  [[ "${manager}" != "unknown" ]] || fail "missing required tools: ${missing[*]}"
  log "installing required tools: ${missing[*]}"
  install_packages "${manager}" "${missing[@]}"
}

map_goarch() {
  case "$(uname -m)" in
    x86_64|amd64)
      printf 'amd64'
      ;;
    aarch64|arm64)
      printf 'arm64'
      ;;
    *)
      fail "unsupported architecture: $(uname -m)"
      ;;
  esac
}

generate_uuid() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
    return
  fi
  if [[ -r /proc/sys/kernel/random/uuid ]]; then
    tr '[:upper:]' '[:lower:]' < /proc/sys/kernel/random/uuid
    return
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16 | sed -E 's/^(.{8})(.{4})(.{4})(.{4})(.{12})$/\1-\2-\3-\4-\5/'
    return
  fi
  fail "unable to generate UUID"
}

prompt_domain() {
  local input="${1:-${DOMAIN}}"

  if [[ -z "${input}" ]]; then
    if [[ "${YES}" -eq 1 ]]; then
      fail "domain is required when using --yes"
    fi
    read -r -p "Enter the public domain for this server: " input
  fi

  input="${input:-}"
  input="${input//[$'\r\n\t ']}"
  [[ -n "${input}" ]] || fail "domain is required"
  [[ "${input}" =~ ^[A-Za-z0-9.-]+$ ]] || fail "invalid domain: ${input}"
  [[ "${input}" == *.* ]] || fail "domain must contain at least one dot: ${input}"
  DOMAIN="${input,,}"
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

confirm_or_abort() {
  local answer

  if [[ "${YES}" -eq 1 ]]; then
    return
  fi

  cat <<EOF
About to deploy subscriptionlink with:
  Domain: ${DOMAIN}
  Caddy config: ${CADDY_CONFIG_PATH}
  Xray config: ${XRAY_CONFIG_PATH}
  Data dir: ${SUB_DATA_DIR}
  Binary: ${SUB_BIN_PATH}
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

github_latest_tag() {
  local repo="$1"
  curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n1
}

install_caddy_from_release() {
  local arch="$1"
  local tag asset archive
  tag="$(github_latest_tag caddyserver/caddy)"
  [[ -n "${tag}" ]] || fail "unable to determine latest Caddy release"

  asset="caddy_${tag#v}_linux_${arch}.tar.gz"
  archive="${TMP_DIR}/${asset}"

  log "installing Caddy ${tag} from release archive"
  curl -fsSL -o "${archive}" "https://github.com/caddyserver/caddy/releases/download/${tag}/${asset}"
  tar -xzf "${archive}" -C "${TMP_DIR}"
  install -m 0755 "${TMP_DIR}/caddy" /usr/local/bin/caddy
}

ensure_caddy_installed() {
  local manager="$1"
  local arch="$2"

  if command -v caddy >/dev/null 2>&1; then
    log "found Caddy: $(command -v caddy)"
    return
  fi

  case "${manager}" in
    apt|dnf|yum|zypper)
      log "installing Caddy via ${manager}"
      if install_packages "${manager}" caddy; then
        command -v caddy >/dev/null 2>&1 && return
      fi
      log "package manager install did not provide caddy, falling back to release archive"
      ;;
    *)
      log "package manager unavailable for Caddy, falling back to release archive"
      ;;
  esac

  install_caddy_from_release "${arch}"
  command -v caddy >/dev/null 2>&1 || fail "Caddy installation failed"
}

install_xray_from_official_script() {
  local installer="${TMP_DIR}/install-xray.sh"

  log "installing Xray from the official installer"
  curl -fsSL -o "${installer}" "https://github.com/XTLS/Xray-install/raw/main/install-release.sh"
  bash "${installer}" install
}

ensure_xray_installed() {
  if command -v xray >/dev/null 2>&1; then
    log "found Xray: $(command -v xray)"
    return
  fi

  install_xray_from_official_script
  command -v xray >/dev/null 2>&1 || fail "Xray installation failed"
}

configure_caddy() {
  local tmp_caddy="${TMP_DIR}/Caddyfile"

  mkdir -p "${CADDY_CONFIG_DIR}" /var/log/caddy
  sed "s/xxx\\.yyy\\.zzz/${DOMAIN}/g" "${CADDY_TEMPLATE}" > "${tmp_caddy}"
  install -m 0644 "${tmp_caddy}" "${CADDY_CONFIG_PATH}"

  if command -v caddy >/dev/null 2>&1; then
    caddy validate --config "${CADDY_CONFIG_PATH}" >/dev/null
  fi
}

configure_xray() {
  local tmp_xray="${TMP_DIR}/config.json"
  local placeholder_uuid

  mkdir -p "${XRAY_CONFIG_DIR}" /var/log/xray
  cp "${XRAY_TEMPLATE}" "${tmp_xray}"

  if grep -q '"id":[[:space:]]*"xxx"' "${tmp_xray}"; then
    placeholder_uuid="$(generate_uuid)"
    sed -i.bak "0,/\"id\":[[:space:]]*\"xxx\"/s//\"id\": \"${placeholder_uuid}\"/" "${tmp_xray}"
    rm -f "${tmp_xray}.bak"
    log "replaced placeholder Xray client UUID with ${placeholder_uuid}"
  fi

  install -m 0644 "${tmp_xray}" "${XRAY_CONFIG_PATH}"
  chmod 0644 "${XRAY_CONFIG_PATH}"
}

ensure_subscription_data() {
  mkdir -p "${SUB_DATA_DIR}"
  install -m 0644 "${CLASH_TEMPLATE}" "${SUB_DATA_DIR}/clash.yaml"

  if [[ ! -f "${SUB_DATA_DIR}/users.json" ]]; then
    printf '[]\n' > "${SUB_DATA_DIR}/users.json"
    chmod 0644 "${SUB_DATA_DIR}/users.json"
  fi
  if [[ ! -f "${SUB_DATA_DIR}/nodes.json" ]]; then
    printf '[]\n' > "${SUB_DATA_DIR}/nodes.json"
    chmod 0644 "${SUB_DATA_DIR}/nodes.json"
  fi
}

build_subscriptionlink_binary() {
  local goarch artifact packaged_artifact
  goarch="$(map_goarch)"
  artifact="${PROJECT_ROOT}/dist/subscriptionlink-linux-${goarch}"
  packaged_artifact="${PACKAGED_BIN_DIR}/subscriptionlink"

  if [[ -f "${packaged_artifact}" ]]; then
    log "using packaged binary ${packaged_artifact}"
    install -m 0755 "${packaged_artifact}" "${SUB_BIN_PATH}"
    return
  fi

  if [[ -f "${artifact}" ]]; then
    log "using existing build artifact ${artifact}"
    install -m 0755 "${artifact}" "${SUB_BIN_PATH}"
    return
  fi

  command -v go >/dev/null 2>&1 || fail "go is required to build the project when dist/subscriptionlink-linux-${goarch} is missing"

  if command -v make >/dev/null 2>&1; then
    log "building subscriptionlink via make for linux/${goarch}"
    (cd "${PROJECT_ROOT}" && make build PLATFORM="linux/${goarch}")
    [[ -x "${artifact}" ]] || fail "expected build artifact not found: ${artifact}"
    install -m 0755 "${artifact}" "${SUB_BIN_PATH}"
    return
  fi

  log "building subscriptionlink via go build"
  mkdir -p "${PROJECT_ROOT}/cmd/server/embedded_assets/web"
  install -m 0644 "${PROJECT_ROOT}/web/admin.html" "${PROJECT_ROOT}/cmd/server/embedded_assets/web/admin.html"
  (cd "${PROJECT_ROOT}" && CGO_ENABLED=0 go build -o "${SUB_BIN_PATH}" ./cmd/server)
}

unit_exists() {
  systemctl cat "$1" >/dev/null 2>&1
}

ensure_caddy_unit() {
  if unit_exists caddy; then
    log "using existing caddy systemd unit"
    return
  fi

  local caddy_bin
  caddy_bin="$(command -v caddy)"

  log "creating caddy systemd unit"
  cat > "${CADDY_UNIT_PATH}" <<EOF
# Managed by subscriptionlink deploy script
[Unit]
Description=Caddy
Documentation=https://caddyserver.com/docs/
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
ExecStart=${caddy_bin} run --environ --config ${CADDY_CONFIG_PATH}
ExecReload=${caddy_bin} reload --config ${CADDY_CONFIG_PATH} --force
ExecStop=${caddy_bin} stop
Restart=on-failure
RestartSec=5s
TimeoutStopSec=5s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
}

ensure_xray_unit() {
  if unit_exists xray; then
    log "using existing xray systemd unit"
    return
  fi

  local xray_bin
  xray_bin="$(command -v xray)"

  log "creating xray systemd unit"
  cat > "${XRAY_UNIT_PATH}" <<EOF
# Managed by subscriptionlink deploy script
[Unit]
Description=Xray
Documentation=https://github.com/XTLS/Xray-core
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${xray_bin} run -config ${XRAY_CONFIG_PATH}
Restart=on-failure
RestartSec=5s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
}

ensure_subscriptionlink_unit() {
  log "writing subscriptionlink systemd unit"
  cat > "${SUB_UNIT_PATH}" <<EOF
# Managed by subscriptionlink deploy script
[Unit]
Description=subscriptionlink
After=network-online.target xray.service
Wants=network-online.target xray.service

[Service]
Type=simple
Environment=ADMIN_COOKIE_SECURE=true
Environment=LISTEN_ADDR=127.0.0.1:8081
Environment=XRAY_CONFIG_PATH=${XRAY_CONFIG_PATH}
Environment="XRAY_RELOAD_CMD=systemctl restart xray"
ExecStart=${SUB_BIN_PATH} -data_dir ${SUB_DATA_DIR}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
}

prepare_service_directory_permissions() {
  local service="$1"
  local dir="$2"
  local user group

  mkdir -p "${dir}"
  chmod 0755 "${dir}"

  user="$(systemctl show "${service}" --property=User --value 2>/dev/null || true)"
  group="$(systemctl show "${service}" --property=Group --value 2>/dev/null || true)"

  if [[ -n "${user}" && "${user}" != "root" ]]; then
    if [[ -z "${group}" ]]; then
      group="${user}"
    fi
    chown -R "${user}:${group}" "${dir}"
  fi
}

restart_and_verify_service() {
  local service="$1"

  log "starting ${service}"
  systemctl enable "${service}" >/dev/null
  systemctl restart "${service}"

  if ! systemctl is-active --quiet "${service}"; then
    systemctl --no-pager --full status "${service}" || true
    journalctl -u "${service}" -n 50 --no-pager || true
    fail "${service} failed to start"
  fi
}

show_admin_key() {
  local admin_key_path="${SUB_DATA_DIR}/admin.key"
  local admin_key

  [[ -f "${admin_key_path}" ]] || fail "admin key not found at ${admin_key_path}"
  admin_key="$(tr -d '\r\n' < "${admin_key_path}")"
  [[ -n "${admin_key}" ]] || fail "admin key is empty"

  printf '\nAdmin key:\n%s\n\n' "${admin_key}"
  printf 'Admin URL: https://%s/admin\n' "${DOMAIN}"
  printf 'Use the admin.key above to log in to the management page.\n'
}

main() {
  local manager arch

  parse_args "$@"

  require_root
  require_linux
  require_systemd
  require_template_files

  prompt_domain
  confirm_or_abort

  TMP_DIR="$(mktemp -d)"
  manager="$(detect_pkg_manager)"
  arch="$(map_goarch)"

  ensure_basic_tools "${manager}"
  ensure_caddy_installed "${manager}" "${arch}"
  ensure_xray_installed

  configure_caddy
  configure_xray
  ensure_subscription_data
  build_subscriptionlink_binary

  ensure_caddy_unit
  ensure_xray_unit
  ensure_subscriptionlink_unit
  systemctl daemon-reload

  prepare_service_directory_permissions xray /var/log/xray
  prepare_service_directory_permissions caddy /var/log/caddy

  restart_and_verify_service xray
  restart_and_verify_service subscriptionlink
  restart_and_verify_service caddy

  show_admin_key
}

main "$@"
