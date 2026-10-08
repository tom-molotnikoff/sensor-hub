#!/usr/bin/env bash
#
# Build Sensor Hub packages locally using nfpm.
#
# Usage:
#   scripts/build-packages.sh <target> [options]
#
# Targets:
#   cli           Build CLI-only package (no UI, no server config)
#   server        Build full server package (includes React UI)
#   sensor        Build temperature-sensor deb (with OpenTelemetry)
#   sensor-lite   Build temperature-sensor-lite deb (no OpenTelemetry)
#   all           Build cli + server packages
#
# Options:
#   --arch <arch>       Target architecture: amd64, arm64 (default: host)
#   --format <fmt>      Package format: rpm, deb (default: host-appropriate)
#   --version <ver>     Package version (default: auto from git tag)
#   --output-dir <dir>  Output directory (default: dist/)
#   -h, --help          Show this help message
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Defaults ---
TARGET=""
ARCH=""
FORMAT=""
VERSION=""
OUTPUT_DIR="$REPO_ROOT/dist"

# --- Usage ---
usage() {
  sed -n '3,16p' "$0" | sed -E 's/^# ?//'
  exit 0
}

# --- Parse arguments ---
[[ $# -eq 0 ]] && usage

while [[ $# -gt 0 ]]; do
  case "$1" in
    cli|server|sensor|sensor-lite|all) TARGET="$1"; shift ;;
    --arch)         ARCH="$2"; shift 2 ;;
    --format)       FORMAT="$2"; shift 2 ;;
    --version)      VERSION="$2"; shift 2 ;;
    --output-dir)   OUTPUT_DIR="${2%/}"; shift 2 ;;
    -h|--help)      usage ;;
    *) echo "Unknown option: $1" >&2; usage ;;
  esac
done

# Resolve OUTPUT_DIR to absolute path (nfpm -t and cd can break relative paths)
[[ "$OUTPUT_DIR" != /* ]] && OUTPUT_DIR="$(pwd)/$OUTPUT_DIR"

[[ -z "$TARGET" ]] && { echo "Error: target required (cli, server, sensor, sensor-lite, or all)" >&2; exit 1; }

# --- Detect host architecture ---
detect_arch() {
  local machine
  machine="$(uname -m)"
  case "$machine" in
    x86_64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "Error: unsupported architecture: $machine" >&2; exit 1 ;;
  esac
}

# --- Detect host package format ---
detect_format() {
  if command -v rpm &>/dev/null; then
    echo "rpm"
  elif command -v dpkg &>/dev/null; then
    echo "deb"
  else
    echo "Error: cannot detect package format — install rpm or dpkg, or pass --format" >&2
    exit 1
  fi
}

# --- Auto-detect version from git tag ---
auto_version() {
  local tag
  tag="$(git -C "$REPO_ROOT" describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")"
  tag="${tag#v}"

  local major minor patch
  IFS='.' read -r major minor patch <<< "$tag"
  patch=$(( ${patch%%[^0-9]*} + 1 ))

  echo "${major}.${minor}.${patch}~dev1"
}

# --- Apply defaults ---
[[ -z "$ARCH" ]]    && ARCH="$(detect_arch)"
[[ -z "$FORMAT" ]]  && FORMAT="$(detect_format)"
[[ -z "$VERSION" ]] && VERSION="$(auto_version)"

# --- Map Go arch name ---
go_arch() {
  case "$1" in
    amd64) echo "amd64" ;;
    arm64) echo "arm64" ;;
    *) echo "Error: unsupported arch: $1" >&2; exit 1 ;;
  esac
}

# --- Check prerequisites ---
needs_go() {
  [[ "$1" == "cli" || "$1" == "server" || "$1" == "all" ]]
}

check_prereqs() {
  local target="$1"
  local missing=()

  if needs_go "$target"; then
    command -v go &>/dev/null || missing+=("go")
  fi

  command -v nfpm &>/dev/null || {
    local gopath_bin
    gopath_bin="$(go env GOPATH 2>/dev/null)/bin"
    if [[ -x "$gopath_bin/nfpm" ]]; then
      export PATH="$gopath_bin:$PATH"
    else
      missing+=("nfpm (install: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest)")
    fi
  }

  if [[ "$target" == "server" || "$target" == "all" ]]; then
    command -v npm &>/dev/null || missing+=("npm (required for server builds)")
  fi

  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "Error: missing required tools:" >&2
    printf '  - %s\n' "${missing[@]}" >&2
    exit 1
  fi
}

check_prereqs "$TARGET"

echo "==> Configuration:"
echo "    Target:  $TARGET"
if needs_go "$TARGET"; then
echo "    Arch:    $ARCH"
echo "    Format:  $FORMAT"
fi
echo "    Version: $VERSION"
echo "    Output:  $OUTPUT_DIR"
echo ""

BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

# --- Build React UI (server only) ---
build_ui() {
  echo "==> Building React UI..."
  cd "$REPO_ROOT/sensor_hub/ui/sensor_hub_ui"
  npm ci --silent
  npm run build

  echo "==> Copying UI dist..."
  rm -rf "$REPO_ROOT/sensor_hub/web/dist"/*
  mkdir -p "$REPO_ROOT/sensor_hub/web/dist"
  cp -r dist/* "$REPO_ROOT/sensor_hub/web/dist/"
  cd "$REPO_ROOT"
}

# --- Build Docusaurus docs (server only) ---
build_docs() {
  echo "==> Building Docusaurus docs..."
  cd "$REPO_ROOT/docs"
  npm ci --silent
  npm run build

  echo "==> Copying docs build..."
  mkdir -p "$REPO_ROOT/sensor_hub/web/docs"
  rm -rf "$REPO_ROOT/sensor_hub/web/docs/"*
  cp -r build/* "$REPO_ROOT/sensor_hub/web/docs/"
  cd "$REPO_ROOT"
}

# --- Compile Go binary ---
build_binary() {
  echo "==> Compiling sensor-hub (linux/$GOARCH, version $VERSION)..."
  cd "$REPO_ROOT/sensor_hub"
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
    go build -ldflags "-s -w -X main.version=$VERSION" -o "$BINARY_PATH" .
  cd "$REPO_ROOT"
}

# --- Generate shell completions ---
generate_completions() {
  echo "==> Generating shell completions..."

  local completion_bin="$BINARY_PATH"

  # If cross-compiling, build a host-native binary for completion generation
  if [[ "$GOARCH" != "$(detect_arch)" ]]; then
    completion_bin="$BUILD_DIR/sensor-hub-host"
    cd "$REPO_ROOT/sensor_hub"
    CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$VERSION" -o "$completion_bin" .
    cd "$REPO_ROOT"
  fi

  "$completion_bin" completion bash > "$REPO_ROOT/packaging/completions/sensor-hub.bash"
  "$completion_bin" completion zsh  > "$REPO_ROOT/packaging/completions/_sensor-hub"
  "$completion_bin" completion fish > "$REPO_ROOT/packaging/completions/sensor-hub.fish"
}

# --- Package with nfpm ---
build_package() {
  local config_name="$1"
  local nfpm_config="$REPO_ROOT/packaging/nfpm-${config_name}.yaml"

  echo "==> Packaging ${config_name} (${FORMAT})..."
  mkdir -p "$OUTPUT_DIR"

  cd "$REPO_ROOT"
  export ARCH VERSION BINARY_PATH

  local resolved_config="$BUILD_DIR/nfpm-${config_name}.yaml"
  envsubst < "$nfpm_config" > "$resolved_config"
  nfpm pkg -f "$resolved_config" -p "$FORMAT" -t "$OUTPUT_DIR"
}

# --- Package sensor with nfpm ---
build_sensor_package() {
  local config_name="$1"
  local nfpm_config="$REPO_ROOT/temperature_sensor/${config_name}.yaml"

  echo "==> Packaging ${config_name} (deb)..."
  mkdir -p "$OUTPUT_DIR"

  cd "$REPO_ROOT/temperature_sensor"
  export VERSION

  local resolved_config="$BUILD_DIR/${config_name}.yaml"
  envsubst < "$nfpm_config" > "$resolved_config"
  nfpm pkg -f "$resolved_config" -p deb -t "$OUTPUT_DIR"
  cd "$REPO_ROOT"
}

# --- Main ---
if needs_go "$TARGET"; then
  GOARCH="$(go_arch "$ARCH")"
  BINARY_PATH="$BUILD_DIR/sensor-hub"

  if [[ "$TARGET" == "server" || "$TARGET" == "all" ]]; then
    build_ui
    build_docs
  fi

  build_binary
  generate_completions

  if [[ "$TARGET" == "cli" || "$TARGET" == "all" ]]; then
    build_package "cli"
  fi

  if [[ "$TARGET" == "server" || "$TARGET" == "all" ]]; then
    build_package "server"
  fi
fi

if [[ "$TARGET" == "sensor" ]]; then
  build_sensor_package "nfpm"
fi

if [[ "$TARGET" == "sensor-lite" ]]; then
  build_sensor_package "nfpm-lite"
fi

echo ""
echo "==> Packages built successfully:"
found=0
for ext in deb rpm apk; do
  for f in "$OUTPUT_DIR"/*."$ext"; do
    [[ -f "$f" ]] && { ls -lh "$f"; found=1; }
  done
done
if [[ "$found" -eq 0 ]]; then
  echo "(no packages found)"
fi
