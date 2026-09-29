#!/usr/bin/env bash
# Build Reasonix for Termux on Android.
#
# Termux is a Linux userspace, but the binary it runs has to be a GOOS=android
# one: android is what makes Go emit a PIE executable, link the armv7 binary
# against libc at all, and resolve DNS through libc instead of an
# /etc/resolv.conf Android does not have. A GOOS=linux binary compiles and then
# fails at runtime, so the target list is fixed here rather than left to the
# caller's GOOS.
#
# A cross build needs the NDK's clang as CC. A build inside Termux needs none:
# its own clang is already the right one for the one architecture that matters.
#
# Usage:
#   scripts/build-termux.sh [-out DIR] [-api LEVEL] [-targets 'android/arm64 ...']
#
#   e.g. scripts/build-termux.sh -out dist
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist"
API="${TERMUX_API_LEVEL:-24}"
TARGETS="${TERMUX_TARGETS:-android/arm64 android/armv7}"
LDFLAGS="${REASONIX_LDFLAGS:-}"

while [ $# -gt 0 ]; do
	case "$1" in
	-out) OUT="$2"; shift 2 ;;
	-api) API="$2"; shift 2 ;;
	-targets) TARGETS="$2"; shift 2 ;;
	*) echo "usage: $0 [-out DIR] [-api LEVEL] [-targets 'android/arm64 ...']" >&2; exit 2 ;;
	esac
done

# Termux runs on Android 7+, whose kernels predate futex_time64 on some 32-bit
# devices; the runtime falls back per-syscall, so 24 is the floor rather than a
# compromise. Anything lower would need Termux's own patched Go.
if [ "$(uname -o 2>/dev/null || echo)" = "Android" ]; then
	case "$(uname -m)" in
	aarch64) TARGETS="android/arm64" ;;
	armv7l | armv8l | arm) TARGETS="android/arm" ;;
	*) echo "unsupported Termux architecture: $(uname -m)" >&2; exit 1 ;;
	esac
fi

TOOLCHAIN_BIN=""
if [ -n "${ANDROID_NDK_HOME:-}" ]; then
	TOOLCHAIN_BIN=$(echo "$ANDROID_NDK_HOME"/toolchains/llvm/prebuilt/*/bin)
else
	SDK="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
	for ndk in "$SDK"/ndk/*; do
		[ -d "$ndk" ] || continue
		TOOLCHAIN_BIN=$(echo "$ndk"/toolchains/llvm/prebuilt/*/bin)
	done
fi
if [ -n "$TOOLCHAIN_BIN" ] && [ ! -x "$TOOLCHAIN_BIN/clang" ]; then
	echo "no NDK clang at $TOOLCHAIN_BIN" >&2
	exit 1
fi

clang_for() {
	local goarch="$1" name=""
	case "$goarch" in
	arm64) name="aarch64-linux-android${API}-clang" ;;
	arm) name="armv7a-linux-androideabi${API}-clang" ;;
	esac
	if [ -z "$name" ]; then
		echo ""
		return
	fi
	if [ -n "$TOOLCHAIN_BIN" ] && [ -x "$TOOLCHAIN_BIN/$name" ]; then
		echo "$TOOLCHAIN_BIN/$name"
		return
	fi
	# On the device the NDK-named wrappers do not exist; Termux's own clang is
	# already targeting this kernel and libc.
	if command -v clang >/dev/null 2>&1; then
		echo "clang"
		return
	fi
	echo ""
}

if [ -z "$LDFLAGS" ]; then
	VERSION="$(git -C "$ROOT" describe --tags --always 2>/dev/null | sed 's/^studio-//' || echo dev)"
	COMMIT="$(git -C "$ROOT" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
	LDFLAGS="-s -w -X main.version=$VERSION -X main.gitCommit=$COMMIT -X main.buildTimeUTC=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

mkdir -p "$OUT"
for target in $TARGETS; do
	goarch="${target#android/}"
	cc="$(clang_for "$goarch")"
	if [ -z "$cc" ]; then
		echo "no clang for $target; install the Android NDK or run inside Termux" >&2
		exit 1
	fi
	suffix="$goarch"
	[ "$goarch" = "arm" ] && suffix="armv7"
	out="$OUT/reasonix-android-$suffix"
	echo "==> $target (CC=$cc)"
	(
		cd "$ROOT"
		export CGO_ENABLED=1 GOOS=android GOARCH="$goarch" CC="$cc"
		[ "$goarch" = "arm" ] && export GOARM=7
		go build -trimpath -ldflags "$LDFLAGS" -o "$out" ./cmd/reasonix
	)
	echo "    $out"
done
