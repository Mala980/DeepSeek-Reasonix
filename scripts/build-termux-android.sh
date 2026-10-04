#!/usr/bin/env bash
# Build Reasonix CLI binaries and release archives for Termux on Android
# (arm64 / aarch64 and armv7 / armv7a).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

OUT_DIR="${TERMUX_OUT_DIR:-bin}"
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"
GIT_COMMIT="${GIT_COMMIT:-$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME_UTC="${BUILD_TIME_UTC:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
PACKAGE_ARCHIVES=0
TARGETS=()

usage() {
  cat <<'EOF'
Usage: scripts/build-termux-android.sh [options] [arm64|armv7|all ...]

Options:
  --out-dir DIR       Output directory for binaries and archives (default: bin)
  --version VERSION   Version string embedded in main.version
  --package           Also create .tar.gz archives and SHA256SUMS in --out-dir
  -h, --help          Show this help message

Targets:
  arm64               Termux Android 64-bit ARM (aarch64 / arm64-v8a)
  armv7 (or arm)      Termux Android 32-bit ARMv7 (armeabi-v7a / armv7l)
  all                 Build both arm64 and armv7 (default)
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --out-dir)
      [ $# -ge 2 ] || { echo "error: --out-dir requires a value" >&2; exit 2; }
      OUT_DIR="$2"
      shift 2
      ;;
    --out-dir=*)
      OUT_DIR="${1#*=}"
      shift
      ;;
    --version)
      [ $# -ge 2 ] || { echo "error: --version requires a value" >&2; exit 2; }
      VERSION="$2"
      shift 2
      ;;
    --version=*)
      VERSION="${1#*=}"
      shift
      ;;
    --package)
      PACKAGE_ARCHIVES=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    arm64|aarch64)
      TARGETS+=("arm64")
      shift
      ;;
    armv7|arm|armeabi-v7a)
      TARGETS+=("armv7")
      shift
      ;;
    all)
      TARGETS+=("arm64" "armv7")
      shift
      ;;
    *)
      echo "error: unknown argument or target: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ ${#TARGETS[@]} -eq 0 ]; then
  TARGETS=("arm64" "armv7")
fi

GO_BIN="${GO_BIN:-go}"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildTimeUTC=${BUILD_TIME_UTC}"

find_ndk_clang() {
  local prefix="$1"
  local preferred_api="${TERMUX_ANDROID_API:-24}"
  local candidate

  if command -v "${prefix}${preferred_api}-clang" >/dev/null 2>&1; then
    command -v "${prefix}${preferred_api}-clang"
    return 0
  fi

  local ndk_roots=()
  for var in ANDROID_NDK_HOME ANDROID_NDK_ROOT ANDROID_NDK ANDROID_NDK_LATEST_HOME; do
    if [ -n "${!var:-}" ] && [ -d "${!var}" ]; then
      ndk_roots+=("${!var}")
    fi
  done
  for sdk_var in ANDROID_HOME ANDROID_SDK_ROOT; do
    if [ -n "${!sdk_var:-}" ] && [ -d "${!sdk_var}/ndk" ]; then
      while IFS= read -r d; do
        [ -n "$d" ] && ndk_roots+=("$d")
      done < <(find "${!sdk_var}/ndk" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort -V -r)
    fi
  done
  if [ -d "/usr/local/lib/android/sdk/ndk" ]; then
    while IFS= read -r d; do
      [ -n "$d" ] && ndk_roots+=("$d")
    done < <(find "/usr/local/lib/android/sdk/ndk" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort -V -r)
  fi

  for root in "${ndk_roots[@]}"; do
    for host_tag in linux-x86_64 linux-aarch64 darwin-x86_64 darwin-arm64; do
      local bin_dir="${root}/toolchains/llvm/prebuilt/${host_tag}/bin"
      [ -d "${bin_dir}" ] || continue
      if [ -x "${bin_dir}/${prefix}${preferred_api}-clang" ]; then
        echo "${bin_dir}/${prefix}${preferred_api}-clang"
        return 0
      fi
      for api in 24 23 22 21 26 27 28 29 30 31 32 33 34 35; do
        candidate="${bin_dir}/${prefix}${api}-clang"
        if [ -x "${candidate}" ]; then
          echo "${candidate}"
          return 0
        fi
      done
    done
  done

  for api in 24 23 22 21 26 27 28 29 30 31 32 33 34 35; do
    if command -v "${prefix}${api}-clang" >/dev/null 2>&1; then
      command -v "${prefix}${api}-clang"
      return 0
    fi
  done
  return 1
}

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    openssl dgst -sha256 "$file" | awk '{print $NF}'
  fi
}

package_binary() {
  local bin_path="$1"
  local arch="$2"
  local stage_dir
  stage_dir="$(mktemp -d)"
  cp "${bin_path}" "${stage_dir}/reasonix"
  chmod 0755 "${stage_dir}/reasonix"

  local android_archive="${OUT_DIR}/reasonix-android-${arch}.tar.gz"
  local termux_archive="${OUT_DIR}/reasonix-termux-android-${arch}.tar.gz"
  tar -C "${stage_dir}" -czf "${android_archive}" reasonix
  cp "${android_archive}" "${termux_archive}"
  rm -rf "${stage_dir}"
  echo "packaged ${android_archive}"
  echo "packaged ${termux_archive}"
}

build_arm64() {
  local out="${OUT_DIR}/reasonix-android-arm64"
  local host_os host_arch
  host_os="$("${GO_BIN}" env GOHOSTOS 2>/dev/null || echo "")"
  host_arch="$("${GO_BIN}" env GOHOSTARCH 2>/dev/null || echo "")"

  local cc="${TERMUX_ANDROID_ARM64_CC:-}"
  if [ -z "${cc}" ] && [ "${TERMUX_ANDROID_CGO:-0}" = "1" ]; then
    cc="$(find_ndk_clang "aarch64-linux-android" || true)"
  fi

  if [ -n "${cc}" ]; then
    echo "building Termux Android arm64 (GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC=${cc}) -> ${out}"
    CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC="${cc}" \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${out}" ./cmd/reasonix
  elif [ "${host_os}" = "android" ] && [ "${host_arch}" = "arm64" ] && [ "${TERMUX_ANDROID_CGO:-0}" = "1" ]; then
    echo "building Termux Android arm64 native (GOOS=android GOARCH=arm64 CGO_ENABLED=1) -> ${out}"
    CGO_ENABLED=1 GOOS=android GOARCH=arm64 \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${out}" ./cmd/reasonix
  else
    echo "building Termux Android arm64 (GOOS=android GOARCH=arm64 CGO_ENABLED=0) -> ${out}"
    CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${out}" ./cmd/reasonix
  fi
  chmod 0755 "${out}"

  if [ "${PACKAGE_ARCHIVES}" -eq 1 ]; then
    package_binary "${out}" "arm64"
  fi
}

build_armv7() {
  local out="${OUT_DIR}/reasonix-android-armv7"
  local host_os host_arch
  host_os="$("${GO_BIN}" env GOHOSTOS 2>/dev/null || echo "")"
  host_arch="$("${GO_BIN}" env GOHOSTARCH 2>/dev/null || echo "")"

  local cc="${TERMUX_ANDROID_ARMV7_CC:-}"
  if [ -z "${cc}" ] && [ "${TERMUX_ANDROID_PURE_GO:-0}" != "1" ]; then
    cc="$(find_ndk_clang "armv7a-linux-androideabi" || true)"
  fi

  if [ "${TERMUX_ANDROID_PURE_GO:-0}" != "1" ] && [ -n "${cc}" ]; then
    echo "building Termux Android armv7 (GOOS=android GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=${cc}) -> ${out}"
    CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 CC="${cc}" \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${out}" ./cmd/reasonix
  elif [ "${TERMUX_ANDROID_PURE_GO:-0}" != "1" ] && [ "${host_os}" = "android" ] && [ "${host_arch}" = "arm" ] && command -v clang >/dev/null 2>&1; then
    echo "building Termux Android armv7 native (GOOS=android GOARCH=arm GOARM=7 CGO_ENABLED=1) -> ${out}"
    CGO_ENABLED=1 GOOS=android GOARCH=arm GOARM=7 \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${out}" ./cmd/reasonix
  else
    if [ "${TERMUX_ANDROID_REQUIRE_NDK:-0}" = "1" ]; then
      echo "error: Android NDK clang (armv7a-linux-androideabi*-clang) is required when TERMUX_ANDROID_REQUIRE_NDK=1" >&2
      exit 1
    fi
    # Go's internal linker disallows CGO_ENABLED=0 for GOOS=android GOARCH=arm
    # ("android/arm requires external (cgo) linking"), so when no Android NDK
    # C toolchain is installed, build a static ARMv7 binary with GOOS=linux
    # GOARCH=arm GOARM=7 (internal/termux still detects Termux at runtime).
    echo "building Termux Android armv7 fallback (GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0) -> ${out}"
    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
      "${GO_BIN}" build -trimpath -ldflags "${LDFLAGS}" -o "${out}" ./cmd/reasonix
  fi
  chmod 0755 "${out}"

  if [ "${PACKAGE_ARCHIVES}" -eq 1 ]; then
    package_binary "${out}" "armv7"
  fi
}

mkdir -p "${OUT_DIR}"

seen_targets=" "
for target in "${TARGETS[@]}"; do
  case "${seen_targets}" in
    *" ${target} "*) continue ;;
  esac
  seen_targets="${seen_targets}${target} "
  case "${target}" in
    arm64)
      build_arm64
      ;;
    armv7)
      build_armv7
      ;;
  esac
done

if [ "${PACKAGE_ARCHIVES}" -eq 1 ]; then
  sums_file="${OUT_DIR}/SHA256SUMS"
  : > "${sums_file}"
  for archive in "${OUT_DIR}"/reasonix-*.tar.gz; do
    [ -f "${archive}" ] || continue
    digest="$(sha256_file "${archive}")"
    printf '%s  %s\n' "${digest}" "$(basename "${archive}")" >> "${sums_file}"
  done
  echo "wrote ${sums_file}"
fi
