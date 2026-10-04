package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	"reasonix/internal/proc"
	"reasonix/internal/secrets"
	"reasonix/internal/termux"
)

// wslClipboardWriteScript reads stdin as UTF-8 and lets Windows PowerShell
// perform the UTF-16 clipboard write. Passing text through clip.exe instead
// makes the Windows program decode the raw UTF-8 bytes using the active OEM
// code page (CP936 on Chinese systems), producing mojibake such as 中文 -> 涓枃.
const wslClipboardWriteScript = `$stdin = [Console]::OpenStandardInput()
$reader = [IO.StreamReader]::new($stdin, [Text.UTF8Encoding]::new($false))
Set-Clipboard -Value $reader.ReadToEnd()`

// writeClipboardText uses the normal platform clipboard outside WSL and Termux.
func writeClipboardText(text string) error {
	if isWSL() {
		return writeWSLClipboardText(text)
	}
	err := clipboard.WriteAll(text)
	if err == nil {
		return nil
	}
	if termux.IsAndroidOrTermux() {
		if tErr := writeTermuxClipboardText(text); tErr == nil {
			return nil
		}
	}
	return err
}

func readPlatformClipboardText() (string, error) {
	text, err := clipboard.ReadAll()
	if err == nil {
		return text, nil
	}
	if termux.IsAndroidOrTermux() {
		if out, tErr := readTermuxClipboardText(); tErr == nil {
			return out, nil
		}
	}
	return "", err
}

func writeTermuxClipboardText(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := proc.CommandContext(ctx, "termux-clipboard-set")
	cmd.Env = secrets.ProcessEnv()
	proc.SetCancelKillsTree(cmd)
	cmd.WaitDelay = time.Second
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func readTermuxClipboardText() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := proc.CommandContext(ctx, "termux-clipboard-get")
	cmd.Env = secrets.ProcessEnv()
	proc.SetCancelKillsTree(cmd)
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func isWSL() bool {
	return isWSLFor(runtime.GOOS, os.Getenv)
}

func isWSLFor(goos string, getenv func(string) string) bool {
	if goos != "linux" {
		return false
	}
	return getenv("WSL_DISTRO_NAME") != "" || getenv("WSL_INTEROP") != ""
}

func writeWSLClipboardText(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := newWSLClipboardCommand(ctx, text)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("write WSL clipboard: %w", ctx.Err())
		}
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return fmt.Errorf("write WSL clipboard: %w: %s", err, detail)
		}
		return fmt.Errorf("write WSL clipboard: %w", err)
	}
	return nil
}

func newWSLClipboardCommand(ctx context.Context, text string) *exec.Cmd {
	args := []string{
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		wslClipboardWriteScript,
	}
	// Keep native Linux clipboard support when Windows interop is unavailable.
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "" && clipboardCommandAvailable("wl-copy"):
		args = []string{"wl-copy"}
	case clipboardCommandAvailable("xclip"):
		args = []string{"xclip", "-in", "-selection", "clipboard"}
	case clipboardCommandAvailable("xsel"):
		args = []string{"xsel", "--input", "--clipboard"}
	}
	cmd := proc.CommandContext(ctx, args[0], args[1:]...)
	proc.SetCancelKillsTree(cmd)
	// Bound pipe draining too, if an interop child inherits an output handle.
	cmd.WaitDelay = time.Second
	cmd.Stdin = strings.NewReader(text)
	return cmd
}

func clipboardCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func cliPlatformAssetCandidates(goos, goarch string) []string {
	out := []string{cliPlatformAssetName(goos, goarch)}
	if goarch == "arm" {
		out = append(out, cliPlatformAssetName(goos, "armv7"))
	} else if goarch == "armv7" {
		out = append(out, cliPlatformAssetName(goos, "arm"))
	}
	if goos == "android" {
		out = append(out, cliPlatformAssetName("linux", goarch), cliPlatformAssetName("linux", "armv7"))
	} else if goos == "linux" && termux.IsTermux() {
		out = append(out, cliPlatformAssetName("android", goarch), cliPlatformAssetName("android", "armv7"))
	}
	return out
}
