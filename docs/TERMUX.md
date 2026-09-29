---
owner: @SivanCola
backup: @esengine
status: active
reviewed: 2026-09-29
---

# Termux on Android

<a href="../README.md">README</a>
&nbsp;·&nbsp;
<a href="./GUIDE.md">Guide</a>
&nbsp;·&nbsp;
<a href="./CLI.md">CLI reference</a>

## Purpose

The CLI runs inside Termux on Android arm64 and armv7. This page covers
installing a prebuilt binary, what behaves differently from a desktop Linux host,
and how to build the binaries. Termux is a Linux userspace on an Android kernel,
so the binaries are `GOOS=android` ones rather than `GOOS=linux` ones.

## Install

1. Install Termux from F-Droid or from its GitHub releases.

2. Install what the CLI looks for at runtime:

   ```bash
   pkg install bash git termux-api
   ```

3. Read the architecture the device reports:

   ```bash
   uname -m    # aarch64 -> arm64; armv7l or armv8l -> armv7
   ```

4. Take the matching artifact from a Termux workflow run (Actions -> Termux):
   `reasonix-android-arm64.tar.gz` or `reasonix-android-armv7.tar.gz`.

5. Check the digest, unpack, and install where the shell finds it:

   ```bash
   sha256sum -c reasonix-android-arm64.tar.gz.sha256
   tar -xzf reasonix-android-arm64.tar.gz
   install -m 0755 reasonix-android-arm64 "$PREFIX/bin/reasonix"
   reasonix --version
   ```

6. Turn the bash sandbox off. Termux has no bubblewrap, and an unset mode means
   `enforce`, so the bash tool refuses to run unconfined until you say so:

   ```toml
   # ~/.config/reasonix/config.toml
   [sandbox]
   bash = "off"
   ```

## Reference

| Area | Behavior on Termux |
| --- | --- |
| Shell | `$SHELL`, then `zsh`, `bash`, `sh` found on `$PATH`, then the absolute `/bin/*` names |
| Bash tool | the interpreter shell discovery finds on `$PATH`; Termux has no bubblewrap, so `[sandbox] bash` has to be `off` for it to run at all |
| Browser automation | needs a Chromium-family binary, which Termux does not ship: discovery reports the missing engine unless `[browser] executable` names one |
| Notifications | `termux-notification`, from the `termux-api` package |
| Open a URL or a folder | `termux-open-url` / `termux-open`, falling back to `xdg-open` when `termux-api` is absent |
| Temp directories | `/tmp`, then `$PREFIX/tmp`, then Go's `os.TempDir()`; the first that accepts a file wins |
| Publishing a file in place | a hard link where the filesystem allows one, an exclusive copy where it refuses them |
| Network interfaces | Android 11+ denies the netlink bind an ordinary app needs, so listing interfaces can fail; callers read that as unknown, not fatal |
| Config and state | `~/.config/reasonix` and the rest of [CONFIG_PATHS](./CONFIG_PATHS.md); `$HOME` is Termux's own home |
| Remote CLI download | refused: published CLI assets cover linux, darwin and windows on amd64 and arm64 |
| Studio desktop | not built for Android; the CLI and `reasonix serve` are what Termux runs |

## Build

| Where | Command | Needs |
| --- | --- | --- |
| GitHub Actions | the Termux workflow: a push to `main-v2` or `studio`, a `v*` or `studio-v*` tag, a pull request, or a manual run | nothing; the artifacts are the output |
| Termux itself | `make termux` | `pkg install golang git make` |
| A Linux or macOS host | `ANDROID_NDK_HOME=/path/to/ndk make termux` | an Android NDK carrying level-24 clang wrappers |

`make termux` runs `scripts/build-termux.sh`, which reads the variables the CI
job sets:

| Variable | Default | Meaning |
| --- | --- | --- |
| `TERMUX_TARGETS` | `android/arm64 android/armv7` | the targets to build; on a device this is forced to the host architecture |
| `TERMUX_API_LEVEL` | `24` | the Android API level the clang wrappers are chosen for |
| `REASONIX_LDFLAGS` | version, commit and build time from git | the `-ldflags` handed to `go build` |
| `-out DIR` | `dist` | where the binaries are written |

Both halves of the environment are fixed on purpose. `GOOS=android` is what
makes Go emit a PIE binary and resolve names through libc instead of an
`/etc/resolv.conf` Termux does not have, and android/arm has no internal linker,
so a cgo-free armv7 binary cannot be produced at all.

The workflow checks each binary as ELF rather than running it: a bionic-linked
binary cannot exec on a runner, whose sysroot holds no `/system/bin/linker`. It
asserts the architecture, `ET_DYN` (PIE), the interpreter, and a `libc.so`
dependency, which is what proves the build really was cgo.
