// Package termux provides runtime detection and environment adaptation for
// Termux on Android (arm64 and armv7).
package termux

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultPrefix is the standard Termux $PREFIX installation root on Android.
	DefaultPrefix = "/data/data/com.termux/files/usr"
	// DefaultHome is the standard Termux $HOME directory on Android.
	DefaultHome = "/data/data/com.termux/files/home"
)

var resolverOnce sync.Once

// IsAndroidOrTermux reports whether the current process is running on Android
// or inside a Termux environment (including GOOS=linux binaries run in Termux).
func IsAndroidOrTermux() bool {
	if runtime.GOOS == "android" {
		return true
	}
	return IsTermux()
}

// IsTermux reports whether the current process is running inside a Termux
// environment via Termux environment variables or the standard Termux prefix.
func IsTermux() bool {
	if IsTermuxEnv(os.Getenv) {
		return true
	}
	return dirExists(DefaultPrefix)
}

// IsTermuxEnv inspects environment variables to detect Termux or Android shells.
func IsTermuxEnv(getenv func(string) string) bool {
	if getenv == nil {
		return false
	}
	for _, key := range []string{"TERMUX_VERSION", "TERMUX_APP_PID", "TERMUX__PREFIX"} {
		if strings.TrimSpace(getenv(key)) != "" {
			return true
		}
	}
	if strings.Contains(strings.ToLower(getenv("PREFIX")), "com.termux") {
		return true
	}
	return strings.TrimSpace(getenv("ANDROID_ROOT")) != "" &&
		strings.TrimSpace(getenv("ANDROID_DATA")) != ""
}

// Prefix returns the active Termux usr prefix directory.
func Prefix() string {
	return PrefixFromEnv(os.Getenv)
}

// PrefixFromEnv resolves the Termux usr prefix from environment variables,
// falling back to DefaultPrefix.
func PrefixFromEnv(getenv func(string) string) string {
	if getenv != nil {
		for _, key := range []string{"TERMUX__PREFIX", "PREFIX"} {
			if p := strings.TrimSpace(getenv(key)); p != "" && filepath.IsAbs(p) {
				return filepath.Clean(p)
			}
		}
	}
	return DefaultPrefix
}

// TmpDir returns the Termux temporary directory ($PREFIX/tmp).
func TmpDir() string {
	return filepath.Join(Prefix(), "tmp")
}

// StableLockRootDir returns "/tmp" when present on the host, or the fixed
// Termux tmp directory when "/tmp" does not exist on Android.
func StableLockRootDir() string {
	if dirExists("/tmp") {
		return "/tmp"
	}
	if dirExists(DefaultPrefix) {
		return filepath.Join(DefaultPrefix, "tmp")
	}
	return TmpDir()
}

// BinDirs returns candidate executable directories for Termux and Android.
func BinDirs() []string {
	candidates := []string{
		filepath.Join(Prefix(), "bin"),
		filepath.Join(DefaultPrefix, "bin"),
		"/system/bin",
	}
	seen := make(map[string]bool, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, dir := range candidates {
		clean := filepath.Clean(dir)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

// BinPaths expands one or more executable names across Termux and Android
// binary directories.
func BinPaths(names ...string) []string {
	dirs := BinDirs()
	out := make([]string, 0, len(dirs)*len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		for _, dir := range dirs {
			out = append(out, filepath.Join(dir, name))
		}
	}
	return out
}

// InitRuntimeEnv normalizes environment variables and pure-Go DNS resolution
// when running inside Termux on Android. It is a no-op on non-Android hosts.
func InitRuntimeEnv() {
	if !IsAndroidOrTermux() {
		return
	}
	prefix := Prefix()
	initHomeAndUser()
	initTmpDir(prefix)
	initPathEnv(prefix)
	initTLSCerts(prefix)
	resolverOnce.Do(func() {
		initDNSResolver(prefix)
	})
}

func initHomeAndUser() {
	if strings.TrimSpace(os.Getenv("HOME")) == "" && dirExists(DefaultHome) {
		_ = os.Setenv("HOME", DefaultHome)
	}
	if strings.TrimSpace(os.Getenv("USER")) == "" {
		uid := os.Getuid()
		if uid >= 0 {
			_ = os.Setenv("USER", "u0_a"+strconv.Itoa(uid%100000))
		} else {
			_ = os.Setenv("USER", "termux")
		}
	}
}

func initTmpDir(prefix string) {
	cur := strings.TrimSpace(os.Getenv("TMPDIR"))
	if cur != "" && cur != "/tmp" && cur != "/data/local/tmp" && dirWritable(cur) {
		return
	}
	if cur != "" && dirWritable(cur) {
		return
	}
	candidate := filepath.Join(prefix, "tmp")
	if err := os.MkdirAll(candidate, 0o700); err == nil && dirWritable(candidate) {
		_ = os.Setenv("TMPDIR", candidate)
		return
	}
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		fallback := filepath.Join(home, ".reasonix", "tmp")
		if err := os.MkdirAll(fallback, 0o700); err == nil {
			_ = os.Setenv("TMPDIR", fallback)
		}
	}
}

func initPathEnv(prefix string) {
	currentPath := os.Getenv("PATH")
	parts := filepath.SplitList(currentPath)
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		seen[filepath.Clean(p)] = true
	}
	prefixBin := filepath.Join(prefix, "bin")
	if !seen[prefixBin] && dirExists(prefixBin) {
		parts = append([]string{prefixBin}, parts...)
		seen[prefixBin] = true
	}
	if !seen["/system/bin"] && dirExists("/system/bin") {
		parts = append(parts, "/system/bin")
	}
	if updated := strings.Join(parts, string(os.PathListSeparator)); updated != currentPath && updated != "" {
		_ = os.Setenv("PATH", updated)
	}
}

func initTLSCerts(prefix string) {
	if strings.TrimSpace(os.Getenv("SSL_CERT_FILE")) == "" {
		certFile := filepath.Join(prefix, "etc", "tls", "cert.pem")
		if fileExists(certFile) {
			_ = os.Setenv("SSL_CERT_FILE", certFile)
		}
	}
	if strings.TrimSpace(os.Getenv("SSL_CERT_DIR")) == "" {
		var dirs []string
		for _, d := range []string{
			filepath.Join(prefix, "etc", "tls", "certs"),
			"/system/etc/security/cacerts",
			"/apex/com.android.conscrypt/cacerts",
		} {
			if dirExists(d) {
				dirs = append(dirs, d)
			}
		}
		if len(dirs) > 0 {
			_ = os.Setenv("SSL_CERT_DIR", strings.Join(dirs, string(os.PathListSeparator)))
		}
	}
}

func initDNSResolver(prefix string) {
	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil && len(ParseResolvConf(string(data))) > 0 {
		return
	}
	if net.DefaultResolver == nil {
		return
	}
	net.DefaultResolver.PreferGo = true
	net.DefaultResolver.Dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		servers := DiscoverDNSServers(prefix)
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		var lastErr error
		for _, srv := range servers {
			conn, err := dialer.DialContext(ctx, network, srv)
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return dialer.DialContext(ctx, network, "1.1.1.1:53")
	}
}

// DiscoverDNSServers returns ordered DNS server host:port endpoints for Termux.
func DiscoverDNSServers(prefix string) []string {
	var servers []string
	if raw := strings.TrimSpace(os.Getenv("REASONIX_DNS_SERVERS")); raw != "" {
		for _, field := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
			if norm := normalizeDNSServer(field); norm != "" {
				servers = append(servers, norm)
			}
		}
	}
	if prefix == "" {
		prefix = Prefix()
	}
	for _, path := range []string{"/etc/resolv.conf", filepath.Join(prefix, "etc", "resolv.conf")} {
		if data, err := os.ReadFile(path); err == nil {
			servers = append(servers, ParseResolvConf(string(data))...)
		}
	}
	if len(servers) == 0 {
		servers = append(servers, androidPropDNSServers()...)
	}
	servers = append(servers, "1.1.1.1:53", "8.8.8.8:53")
	return uniqueStrings(servers)
}

// ParseResolvConf extracts nameserver host:port endpoints from resolv.conf text.
func ParseResolvConf(content string) []string {
	var out []string
	for line := range strings.SplitSeq(content, "\n") {
		if idx := strings.IndexAny(line, "#;"); idx >= 0 {
			line = line[:idx]
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], "nameserver") {
			if norm := normalizeDNSServer(fields[1]); norm != "" {
				out = append(out, norm)
			}
		}
	}
	return uniqueStrings(out)
}

func androidPropDNSServers() []string {
	getprop, err := exec.LookPath("getprop")
	if err != nil {
		if fileExists("/system/bin/getprop") {
			getprop = "/system/bin/getprop"
		} else {
			return nil
		}
	}
	var out []string
	for _, prop := range []string{"net.dns1", "net.dns2", "net.dns3", "net.dns4"} {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		raw, err := exec.CommandContext(ctx, getprop, prop).Output()
		cancel()
		if err != nil {
			continue
		}
		if norm := normalizeDNSServer(strings.TrimSpace(string(raw))); norm != "" {
			out = append(out, norm)
		}
	}
	return out
}

func normalizeDNSServer(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if host, port, err := net.SplitHostPort(raw); err == nil {
		if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && port != "" {
			return net.JoinHostPort(ip.String(), port)
		}
		return ""
	}
	if ip := net.ParseIP(strings.Trim(raw, "[]")); ip != nil {
		return net.JoinHostPort(ip.String(), "53")
	}
	return ""
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirWritable(path string) bool {
	if !dirExists(path) {
		return false
	}
	f, err := os.CreateTemp(path, ".reasonix-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
