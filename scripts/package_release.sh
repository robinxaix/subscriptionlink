#!/usr/bin/env bash
set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

TARGET=""
VERSION=""
OUTPUT_DIR="${PROJECT_ROOT}/release"

log() {
  printf '[package] %s\n' "$*" >&2
}

fail() {
  printf '[package] error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  ./scripts/package_release.sh --target OS/ARCH --version VERSION [--output-dir DIR]

Examples:
  ./scripts/package_release.sh --target linux/amd64 --version v1.0.0
  ./scripts/package_release.sh --target windows/amd64 --version v1.0.0 --output-dir ./release
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --target)
        [[ $# -ge 2 ]] || fail "missing value for $1"
        TARGET="$2"
        shift 2
        ;;
      --version)
        [[ $# -ge 2 ]] || fail "missing value for $1"
        VERSION="$2"
        shift 2
        ;;
      --output-dir)
        [[ $# -ge 2 ]] || fail "missing value for $1"
        OUTPUT_DIR="$2"
        shift 2
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

  [[ -n "${TARGET}" ]] || fail "--target is required"
  [[ -n "${VERSION}" ]] || fail "--version is required"
  [[ "${TARGET}" == */* ]] || fail "invalid target: ${TARGET}"
}

require_files() {
  local path
  for path in \
    "${PROJECT_ROOT}/README.md" \
    "${PROJECT_ROOT}/LICENSE" \
    "${PROJECT_ROOT}/data/Caddyfile" \
    "${PROJECT_ROOT}/data/clash.yaml" \
    "${PROJECT_ROOT}/data/xray.config" \
    "${PROJECT_ROOT}/scripts/deploy_linux.sh" \
    "${PROJECT_ROOT}/scripts/uninstall_linux.sh" \
    "${PROJECT_ROOT}/scripts/reinstall_linux.sh"; do
    [[ -f "${path}" ]] || fail "missing required file: ${path}"
  done
}

binary_ext() {
  local os="$1"
  if [[ "${os}" == "windows" ]]; then
    printf '.exe'
    return
  fi
  printf ''
}

binary_name() {
  local os="$1"
  printf 'subscriptionlink%s' "$(binary_ext "${os}")"
}

dist_artifact_path() {
  local os="$1" arch="$2"
  printf '%s/dist/subscriptionlink-%s-%s%s' "${PROJECT_ROOT}" "${os}" "${arch}" "$(binary_ext "${os}")"
}

ensure_dist_artifact() {
  local os="$1" arch="$2" artifact
  artifact="$(dist_artifact_path "${os}" "${arch}")"
  if [[ -f "${artifact}" ]]; then
    printf '%s' "${artifact}"
    return
  fi

  command -v make >/dev/null 2>&1 || fail "make is required to build ${TARGET}"
  command -v go >/dev/null 2>&1 || fail "go is required to build ${TARGET}"

  log "building ${TARGET}"
  (cd "${PROJECT_ROOT}" && make build PLATFORM="${os}/${arch}")
  [[ -f "${artifact}" ]] || fail "expected artifact not found: ${artifact}"
  printf '%s' "${artifact}"
}

checksum_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" > "$2"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" > "$2"
    return
  fi
  fail "sha256sum or shasum is required"
}

write_install_guide() {
  local os="$1" arch="$2" file="$3" archive_name="$4"
  cat > "${file}" <<EOF
# ${archive_name}

Contents:

- \`bin/$(binary_name "${os}")\`: prebuilt ${os}/${arch} binary
- \`data/\`: deployment templates
- \`scripts/\`: deployment helpers

Linux server deployment:

1. Extract the archive on the target server.
2. Change into the extracted directory.
3. Run:

\`\`\`bash
sudo ./scripts/deploy_linux.sh --domain your.domain.example
\`\`\`

Removal:

\`\`\`bash
sudo ./scripts/uninstall_linux.sh
\`\`\`

Reinstall:

\`\`\`bash
sudo ./scripts/reinstall_linux.sh --domain your.domain.example
\`\`\`

Notes:

- The deployment scripts only support Linux with systemd.
- The application binary itself is included for ${os}/${arch}.
- For Windows, unpack the archive and run \`bin\\subscriptionlink.exe\`.
EOF
}

create_archive() {
  local stage_dir="$1" archive_base="$2" os="$3"
  local archive_path

  mkdir -p "${OUTPUT_DIR}"
  if [[ "${os}" == "windows" ]]; then
    archive_path="${OUTPUT_DIR}/${archive_base}.zip"
    (cd "${OUTPUT_DIR}" && zip -qr "${archive_base}.zip" "${archive_base}")
  else
    archive_path="${OUTPUT_DIR}/${archive_base}.tar.gz"
    tar -C "${OUTPUT_DIR}" -czf "${archive_path}" "${archive_base}"
  fi

  checksum_file "${archive_path}" "${archive_path}.sha256"
  printf '%s' "${archive_path}"
}

main() {
  local os arch version_clean archive_base stage_dir artifact archive_path

  parse_args "$@"
  require_files

  os="${TARGET%/*}"
  arch="${TARGET#*/}"
  version_clean="${VERSION#v}"
  archive_base="subscriptionlink_${version_clean}_${os}_${arch}"
  stage_dir="${OUTPUT_DIR}/${archive_base}"
  artifact="$(ensure_dist_artifact "${os}" "${arch}")"

  rm -rf "${stage_dir}"
  mkdir -p "${stage_dir}/bin" "${stage_dir}/data" "${stage_dir}/scripts"

  cp "${PROJECT_ROOT}/README.md" "${stage_dir}/README.md"
  cp "${PROJECT_ROOT}/LICENSE" "${stage_dir}/LICENSE"
  cp "${PROJECT_ROOT}/data/Caddyfile" "${stage_dir}/data/Caddyfile"
  cp "${PROJECT_ROOT}/data/clash.yaml" "${stage_dir}/data/clash.yaml"
  cp "${PROJECT_ROOT}/data/xray.config" "${stage_dir}/data/xray.config"
  install -m 0755 "${PROJECT_ROOT}/scripts/deploy_linux.sh" "${stage_dir}/scripts/deploy_linux.sh"
  install -m 0755 "${PROJECT_ROOT}/scripts/uninstall_linux.sh" "${stage_dir}/scripts/uninstall_linux.sh"
  install -m 0755 "${PROJECT_ROOT}/scripts/reinstall_linux.sh" "${stage_dir}/scripts/reinstall_linux.sh"
  install -m 0755 "${artifact}" "${stage_dir}/bin/$(binary_name "${os}")"
  write_install_guide "${os}" "${arch}" "${stage_dir}/INSTALL.md" "${archive_base}"

  archive_path="$(create_archive "${stage_dir}" "${archive_base}" "${os}")"
  rm -rf "${stage_dir}"
  log "created ${archive_path}"
  log "created ${archive_path}.sha256"
}

main "$@"
