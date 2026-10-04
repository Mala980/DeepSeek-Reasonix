#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

FAKE_BIN="${TMP_DIR}/bin"
NDK_DIR="${TMP_DIR}/ndk/27.0.0/toolchains/llvm/prebuilt/linux-x86_64/bin"
LOG_FILE="${TMP_DIR}/go-invocations.log"
mkdir -p "${FAKE_BIN}" "${NDK_DIR}"

cat > "${FAKE_BIN}/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = "env" ]; then
  case "${2:-}" in
    GOHOSTOS) echo "${FAKE_GOHOSTOS:-linux}" ;;
    GOHOSTARCH) echo "${FAKE_GOHOSTARCH:-amd64}" ;;
    *) echo "" ;;
  esac
  exit 0
fi
if [ "${1:-}" = "build" ]; then
  out=""
  prev=""
  for arg in "$@"; do
    if [ "${prev}" = "-o" ]; then
      out="${arg}"
    fi
    prev="${arg}"
  done
  printf 'GOOS=%s GOARCH=%s GOARM=%s CGO_ENABLED=%s CC=%s OUT=%s\n' \
    "${GOOS:-}" "${GOARCH:-}" "${GOARM:-}" "${CGO_ENABLED:-}" "${CC:-}" "${out}" >> "${GO_LOG_FILE}"
  mkdir -p "$(dirname "${out}")"
  printf '#!/bin/sh\necho reasonix-mock-%s-%s\n' "${GOOS:-}" "${GOARCH:-}" > "${out}"
  chmod 0755 "${out}"
  exit 0
fi
echo "unexpected go subcommand: $*" >&2
exit 1
EOF
chmod 0755 "${FAKE_BIN}/go"

cat > "${NDK_DIR}/armv7a-linux-androideabi24-clang" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "${NDK_DIR}/aarch64-linux-android24-clang" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod 0755 "${NDK_DIR}/armv7a-linux-androideabi24-clang" "${NDK_DIR}/aarch64-linux-android24-clang"

# Case 1: With Android NDK present, arm64 uses GOOS=android GOARCH=arm64 CGO_ENABLED=0
# and armv7 uses GOOS=android GOARCH=arm GOARM=7 CGO_ENABLED=1 with NDK clang.
: > "${LOG_FILE}"
OUT1="${TMP_DIR}/out-ndk"
GO_BIN="${FAKE_BIN}/go" GO_LOG_FILE="${LOG_FILE}" ANDROID_NDK_HOME="${TMP_DIR}/ndk/27.0.0" \
  bash "${ROOT_DIR}/scripts/build-termux-android.sh" --out-dir "${OUT1}" --version v1.2.3 --package all

grep -q 'GOOS=android GOARCH=arm64 GOARM= CGO_ENABLED=0 CC= OUT=.*/reasonix-android-arm64' "${LOG_FILE}"
grep -q "GOOS=android GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=${NDK_DIR}/armv7a-linux-androideabi24-clang OUT=.*/reasonix-android-armv7" "${LOG_FILE}"

for f in \
  reasonix-android-arm64 \
  reasonix-android-armv7 \
  reasonix-android-arm64.tar.gz \
  reasonix-android-armv7.tar.gz \
  reasonix-termux-android-arm64.tar.gz \
  reasonix-termux-android-armv7.tar.gz \
  SHA256SUMS; do
  [ -f "${OUT1}/${f}" ] || { echo "missing expected output: ${OUT1}/${f}" >&2; exit 1; }
done

grep -q '  reasonix-android-arm64.tar.gz$' "${OUT1}/SHA256SUMS"
grep -q '  reasonix-android-armv7.tar.gz$' "${OUT1}/SHA256SUMS"
grep -q '  reasonix-termux-android-arm64.tar.gz$' "${OUT1}/SHA256SUMS"
grep -q '  reasonix-termux-android-armv7.tar.gz$' "${OUT1}/SHA256SUMS"

# Case 2: Without Android NDK (or with TERMUX_ANDROID_PURE_GO=1), armv7 falls back
# to GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 so it links without external cgo.
: > "${LOG_FILE}"
OUT2="${TMP_DIR}/out-pure"
GO_BIN="${FAKE_BIN}/go" GO_LOG_FILE="${LOG_FILE}" TERMUX_ANDROID_PURE_GO=1 \
  bash "${ROOT_DIR}/scripts/build-termux-android.sh" --out-dir "${OUT2}" armv7

grep -q 'GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 CC= OUT=.*/reasonix-android-armv7' "${LOG_FILE}"
[ -x "${OUT2}/reasonix-android-armv7" ]

echo "build-termux-android.test.sh: PASS"
