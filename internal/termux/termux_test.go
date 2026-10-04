package termux

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIsTermuxEnv(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "empty environment",
			env:  map[string]string{},
			want: false,
		},
		{
			name: "TERMUX_VERSION set",
			env:  map[string]string{"TERMUX_VERSION": "0.118.1"},
			want: true,
		},
		{
			name: "PREFIX under com.termux",
			env:  map[string]string{"PREFIX": "/data/data/com.termux/files/usr"},
			want: true,
		},
		{
			name: "ANDROID_ROOT and ANDROID_DATA set",
			env:  map[string]string{"ANDROID_ROOT": "/system", "ANDROID_DATA": "/data"},
			want: true,
		},
		{
			name: "ANDROID_ROOT alone is insufficient",
			env:  map[string]string{"ANDROID_ROOT": "/system"},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsTermuxEnv(func(k string) string { return tc.env[k] })
			if got != tc.want {
				t.Fatalf("IsTermuxEnv() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPrefixFromEnv(t *testing.T) {
	if got := PrefixFromEnv(func(string) string { return "" }); got != DefaultPrefix {
		t.Fatalf("PrefixFromEnv(empty) = %q, want %q", got, DefaultPrefix)
	}
	custom := "/data/user/0/com.termux/files/usr"
	got := PrefixFromEnv(func(k string) string {
		if k == "PREFIX" {
			return custom
		}
		return ""
	})
	if got != filepath.Clean(custom) {
		t.Fatalf("PrefixFromEnv(custom) = %q, want %q", got, filepath.Clean(custom))
	}
}

func TestParseResolvConf(t *testing.T) {
	raw := `
# Termux resolv.conf
nameserver 8.8.4.4 ; comment
nameserver 2001:4860:4860::8888
search local
nameserver invalid-host
nameserver 8.8.4.4
`
	got := ParseResolvConf(raw)
	want := []string{"8.8.4.4:53", "[2001:4860:4860::8888]:53"}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseResolvConf() = %v, want %v", got, want)
	}
}

func TestDiscoverDNSServersIncludesEnvAndFallbacks(t *testing.T) {
	prefix := t.TempDir()
	etcDir := filepath.Join(prefix, "etc")
	if err := os.MkdirAll(etcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(etcDir, "resolv.conf"), []byte("nameserver 9.9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REASONIX_DNS_SERVERS", "1.0.0.1, [2606:4700:4700::1111]:5353")
	got := DiscoverDNSServers(prefix)
	for _, required := range []string{"1.0.0.1:53", "[2606:4700:4700::1111]:5353", "9.9.9.9:53", "1.1.1.1:53", "8.8.8.8:53"} {
		if !slices.Contains(got, required) {
			t.Fatalf("DiscoverDNSServers() = %v, missing %q", got, required)
		}
	}
}

func TestInitRuntimeEnvPopulatesTermuxDefaults(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "com.termux", "files", "usr")
	binDir := filepath.Join(prefix, "bin")
	certDir := filepath.Join(prefix, "etc", "tls")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		t.Fatal(err)
	}
	certFile := filepath.Join(certDir, "cert.pem")
	if err := os.WriteFile(certFile, []byte("-----BEGIN CERTIFICATE-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TERMUX_VERSION", "0.118.1")
	t.Setenv("PREFIX", prefix)
	t.Setenv("TMPDIR", "")
	t.Setenv("SSL_CERT_FILE", "")

	InitRuntimeEnv()

	wantTmp := filepath.Join(prefix, "tmp")
	if got := os.Getenv("TMPDIR"); got != wantTmp {
		t.Fatalf("TMPDIR = %q, want %q", got, wantTmp)
	}
	if got := os.Getenv("SSL_CERT_FILE"); got != certFile {
		t.Fatalf("SSL_CERT_FILE = %q, want %q", got, certFile)
	}
	pathEntries := filepath.SplitList(os.Getenv("PATH"))
	if !slices.Contains(pathEntries, binDir) {
		t.Fatalf("PATH = %v, missing %q", pathEntries, binDir)
	}
}
