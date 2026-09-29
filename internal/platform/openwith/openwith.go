// Package openwith names the command a host uses to hand something to its own
// handler: the browser for a URL, the file manager for a directory. One place
// answers it because the platforms disagree in shape rather than in spelling —
// Windows opens a folder with Explorer and a URL with rundll32, and an Android
// (Termux) session has neither, only termux-api.
package openwith

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// URL returns the command that opens target in this host's browser.
func URL(target string) *exec.Cmd {
	return command(false, target, runtime.GOOS, exec.LookPath)
}

// Folder returns the command that opens dir in this host's file manager.
func Folder(dir string) *exec.Cmd {
	return command(true, dir, runtime.GOOS, exec.LookPath)
}

func command(folder bool, target, goos string, lookPath func(string) (string, error)) *exec.Cmd {
	name, args := handler(folder, target, goos, lookPath)
	return exec.Command(name, args...)
}

func handler(folder bool, target, goos string, lookPath func(string) (string, error)) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{target}
	case "windows":
		if folder {
			return windowsExplorer(), []string{target}
		}
		return "rundll32", []string{"url.dll,FileProtocolHandler", target}
	case "android":
		// termux-api is the handler an Android session has. A proot or chroot
		// under Termux provides xdg-open instead, so look before naming.
		name := "termux-open-url"
		if folder {
			name = "termux-open"
		}
		if path, err := lookPath(name); err == nil {
			return path, []string{target}
		}
		return "xdg-open", []string{target}
	default:
		return "xdg-open", []string{target}
	}
}

// windowsExplorer names explorer.exe by its %SystemRoot% path: a launch
// environment can hand a process a PATH without that directory.
func windowsExplorer() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = os.Getenv("windir")
	}
	if root == "" {
		return "explorer.exe"
	}
	return filepath.Join(root, "explorer.exe")
}
