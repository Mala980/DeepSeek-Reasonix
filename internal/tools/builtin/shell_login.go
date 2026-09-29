package builtin

// shell_login.go resolves the interpreter the login-shell PATH probe runs.

import (
	"os"
	"os/exec"
	"strings"
)

// fallbackShellNames are asked of PATH, which is the only place a host without
// the Unix /bin layout answers: Termux keeps its shells in $PREFIX/bin.
var fallbackShellNames = []string{"zsh", "bash", "sh"}

// fallbackShellPaths are the distribution's own, for a PATH too narrow to name
// an interpreter that is certainly installed.
var fallbackShellPaths = []string{"/bin/zsh", "/bin/bash", "/bin/sh"}

func loginShell() string {
	return loginShellFrom(os.Getenv("SHELL"), exec.LookPath, isExecutableFile)
}

// loginShellFrom is $SHELL where it can run, else the first shell the host has:
// a name on PATH, then an absolute path that executes. An unrunnable $SHELL is
// a stale setting, not an answer, so it falls through to both.
func loginShellFrom(shellEnv string, lookPath func(string) (string, error), exists func(string) bool) string {
	if shell := strings.TrimSpace(shellEnv); shell != "" {
		if hasPathSeparator(shell) {
			if exists(shell) {
				return shell
			}
		} else if p, err := lookPath(shell); err == nil {
			return p
		}
	}
	for _, name := range fallbackShellNames {
		if p, err := lookPath(name); err == nil {
			return p
		}
	}
	for _, path := range fallbackShellPaths {
		if exists(path) {
			return path
		}
	}
	return ""
}

func hasPathSeparator(s string) bool {
	return strings.ContainsAny(s, `/\`)
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
