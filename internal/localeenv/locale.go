// Package localeenv supplies a UTF-8 default for child processes without
// replacing explicitly configured locale semantics.
package localeenv

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"reasonix/internal/proc"
	"reasonix/internal/termux"
)

var hostDefault = sync.OnceValue(discoverUTF8)

// DefaultUTF8 returns a copy only when a default is needed. Empty locale
// variables have the same meaning as absent variables. Explicit C/POSIX and
// category overrides remain user-owned; this never sets LC_ALL.
func DefaultUTF8(env []string) []string {
	if runtime.GOOS == "windows" || hasLocale(env) {
		return env
	}
	return withDefault(env, hostDefault())
}

func hasLocale(env []string) bool {
	for _, item := range env {
		key, value, _ := strings.Cut(item, "=")
		if (key == "LANG" || key == "LC_ALL" || key == "LC_CTYPE") && value != "" {
			return true
		}
	}
	return false
}

func withDefault(env []string, locale string) []string {
	if locale == "" || hasLocale(env) {
		return env
	}
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, "LANG=") {
			out = append(out, item)
		}
	}
	return append(out, "LANG="+locale)
}

func discoverUTF8() string {
	// Probe once, without inheriting credentials or relying on the user's
	// PATH. Do not invent a locale that a minimal Linux image lacks.
	for _, bin := range append([]string{"/usr/bin/locale", "/bin/locale"}, termux.BinPaths("locale")...) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cmd := proc.CommandContext(ctx, bin, "-a")
		cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/data/data/com.termux/files/usr/bin"}
		output, err := cmd.Output()
		cancel()
		if err == nil {
			if selected := selectUTF8(string(output)); selected != "" {
				return selected
			}
		}
	}
	if termux.IsAndroidOrTermux() {
		return "C.UTF-8"
	}
	return ""
}

func selectUTF8(locales string) string {
	available := strings.Fields(locales)
	for _, preferred := range []string{"c.utf8", "en_us.utf8"} {
		for _, locale := range available {
			if strings.ReplaceAll(strings.ToLower(locale), "-", "") == preferred {
				return locale
			}
		}
	}
	for _, locale := range available {
		if strings.HasSuffix(strings.ReplaceAll(strings.ToLower(locale), "-", ""), ".utf8") {
			return locale
		}
	}
	return ""
}
