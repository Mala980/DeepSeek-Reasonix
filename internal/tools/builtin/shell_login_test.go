package builtin

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var errNoShell = errors.New("executable file not found in $PATH")

// A Termux session has no /bin at all: its shells sit on PATH under $PREFIX/bin,
// so a discovery that only stats absolute paths finds no interpreter and the
// PATH probe that feeds every Bash call answers nothing.
func TestLoginShellFindsTheShellOnPath(t *testing.T) {
	inPath := map[string]string{"bash": "/data/data/com.termux/files/usr/bin/bash"}
	lookPath := func(name string) (string, error) {
		if p, ok := inPath[name]; ok {
			return p, nil
		}
		return "", errNoShell
	}

	got := loginShellFrom("", lookPath, func(string) bool { return false })
	if got != inPath["bash"] {
		t.Fatalf("loginShell = %q, want bash from PATH", got)
	}
}

func TestLoginShellHonoursARunnableShellSetting(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "fish" {
			return "/usr/local/bin/fish", nil
		}
		return "", errNoShell
	}
	for _, tc := range []struct {
		name   string
		env    string
		exists func(string) bool
		want   string
	}{
		{
			name:   "an absolute setting that runs",
			env:    "/bin/zsh",
			exists: func(p string) bool { return p == "/bin/zsh" },
			want:   "/bin/zsh",
		},
		{
			name:   "a bare setting resolved on PATH",
			env:    "fish",
			exists: func(string) bool { return false },
			want:   "/usr/local/bin/fish",
		},
		{
			name:   "an absolute setting that does not run falls through",
			env:    "/gone/zsh",
			exists: func(p string) bool { return p == "/bin/sh" },
			want:   "/bin/sh",
		},
		{
			name:   "a setting that is only whitespace",
			env:    "   ",
			exists: func(p string) bool { return p == "/bin/sh" },
			want:   "/bin/sh",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := loginShellFrom(tc.env, lookPath, tc.exists); got != tc.want {
				t.Fatalf("loginShell = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoginShellFallsBackToTheAbsolutePaths(t *testing.T) {
	lookPath := func(string) (string, error) { return "", errNoShell }
	if got := loginShellFrom("", lookPath, func(p string) bool { return p == "/bin/bash" }); got != "/bin/bash" {
		t.Fatalf("loginShell = %q, want the absolute bash", got)
	}
	if got := loginShellFrom("", lookPath, func(string) bool { return false }); got != "" {
		t.Fatalf("loginShell = %q, want nothing to guess at", got)
	}
}

func TestIsExecutableFileAnswersForARealFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports no execute bit for a file")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isExecutableFile(script) {
		t.Fatal("an executable file was not executable")
	}
	if isExecutableFile(dir) {
		t.Fatal("a directory was executable")
	}
	if isExecutableFile(filepath.Join(dir, "absent")) {
		t.Fatal("an absent path was executable")
	}
}
