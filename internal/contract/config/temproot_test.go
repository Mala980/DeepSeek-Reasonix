package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A host without /tmp — Termux is the case in production — still has to give
// every process the same registry root, so the answer comes from what the host
// can hold a file in, not from what a distribution calls its temp directory.
func TestSharedTempRootFallsBackToThePrefixTemp(t *testing.T) {
	prefix := t.TempDir()
	prefixTmp := filepath.Join(prefix, "tmp")
	if err := os.MkdirAll(prefixTmp, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PREFIX", prefix)
	t.Setenv("TMPDIR", prefixTmp)

	got := sharedTempRoot()
	if got != "/tmp" && got != prefixTmp {
		t.Fatalf("sharedTempRoot() = %q, want /tmp where it accepts files or $PREFIX/tmp", got)
	}
}

func TestSharedTempRootOrdersTheHostInvariantBeforeTheProcessTmpdir(t *testing.T) {
	prefix := t.TempDir()
	processTmp := t.TempDir()

	candidates := sharedTempRootCandidatesFor("/missing", prefix, processTmp)
	want := []string{"/missing", filepath.Join(prefix, "tmp"), processTmp}
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %v, want %v", candidates, want)
	}
	for i := range want {
		if candidates[i] != want[i] {
			t.Fatalf("candidates = %v, want %v", candidates, want)
		}
	}
	if got := firstAccepting(candidates); got != processTmp {
		t.Fatalf("firstAccepting = %q, want the one candidate that exists", got)
	}
}

func TestSharedTempRootDropsAnUnsetPrefix(t *testing.T) {
	candidates := sharedTempRootCandidatesFor("/tmp", "  ", "/tmp")
	if len(candidates) != 1 || candidates[0] != "/tmp" {
		t.Fatalf("candidates = %v, want the system temp root alone", candidates)
	}
}

func TestDirAcceptsFilesAnswersForADirectoryThatIsNotThere(t *testing.T) {
	if dirAcceptsFiles(filepath.Join(t.TempDir(), "absent")) {
		t.Fatal("a directory that does not exist accepted a file")
	}
	if dirAcceptsFiles("") {
		t.Fatal("an empty root accepted a file")
	}
	dir := t.TempDir()
	if !dirAcceptsFiles(dir) {
		t.Fatal("a real temporary directory refused a file")
	}
	if left, err := filepath.Glob(filepath.Join(dir, ".reasonix-root-probe-*")); err != nil || len(left) > 0 {
		t.Fatalf("the probe left files behind: %v %v", left, err)
	}
}
