package fileutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// denyLinks makes every link in the test report the refusal class a volume
// without hard links reports, which is the one LinkNoReplace copies through.
func denyLinks(t *testing.T, err error) {
	t.Helper()
	orig := linkFile
	linkFile = func(_, _ string) error { return err }
	t.Cleanup(func() { linkFile = orig })
}

func writeSource(t *testing.T, body string, perm os.FileMode) (dir, src string) {
	t.Helper()
	dir = t.TempDir()
	src = filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte(body), perm); err != nil {
		t.Fatal(err)
	}
	return dir, src
}

func filePerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestLinkNoReplaceRefusesAnExistingDestination(t *testing.T) {
	dir, src := writeSource(t, "new\n", 0o600)
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(dst, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := LinkNoReplace(src, dst)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("err = %v, want fs.ErrExist", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "old\n" {
		t.Fatalf("destination rewritten: %q", got)
	}
}

func TestLinkNoReplaceCopiesWhereLinksAreDenied(t *testing.T) {
	dir, src := writeSource(t, "payload\n", 0o640)
	denyLinks(t, &os.LinkError{Op: "link", Old: src, New: "dst", Err: syscall.EPERM})

	dst := filepath.Join(dir, "dst")
	if err := LinkNoReplace(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "payload\n" {
		t.Fatalf("dst = %q, %v", got, err)
	}
	// The source's own mode, not a literal: umask trims it on Unix and Windows
	// reports no mode bits at all beyond the read-only flag.
	wantMode := filePerm(t, src)
	if got := filePerm(t, dst); got != wantMode {
		t.Fatalf("dst mode = %o, want the source's %o", got, wantMode)
	}
}

func TestLinkNoReplaceCopyStillRefusesAnExistingDestination(t *testing.T) {
	dir, src := writeSource(t, "new\n", 0o600)
	denyLinks(t, &os.LinkError{Op: "link", Old: src, New: "dst", Err: syscall.EACCES})
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(dst, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LinkNoReplace(src, dst); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("err = %v, want fs.ErrExist", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "old\n" {
		t.Fatalf("destination rewritten: %q", got)
	}
}

func TestLinkNoReplaceReportsARefusalItDoesNotRecognise(t *testing.T) {
	dir, src := writeSource(t, "payload\n", 0o600)
	denyLinks(t, &os.LinkError{Op: "link", Old: src, New: "dst", Err: syscall.EXDEV})

	if err := LinkNoReplace(src, filepath.Join(dir, "dst")); !errors.Is(err, syscall.EXDEV) {
		t.Fatalf("err = %v, want the link's own EXDEV", err)
	}
}

func TestLinksDeniedIsTheUnsupportedVolumeClass(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{syscall.EPERM, true},
		{syscall.EACCES, true},
		{syscall.ENOSYS, true},
		{syscall.EXDEV, false},
		{syscall.ENOENT, false},
		{fs.ErrExist, false},
	} {
		if got := linksDenied(tc.err); got != tc.want {
			t.Errorf("linksDenied(%v) = %t, want %t", tc.err, got, tc.want)
		}
	}
}

func TestAtomicCreateFilePublishesWhereLinksAreDenied(t *testing.T) {
	dir := t.TempDir()
	denyLinks(t, syscall.EPERM)
	path := filepath.Join(dir, "created")
	if err := AtomicCreateFile(path, []byte("body\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "body\n" {
		t.Fatalf("path = %q, %v", got, err)
	}
	if err := AtomicCreateFile(path, []byte("second\n"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("second create = %v, want fs.ErrExist", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "body\n" {
		t.Fatalf("published file replaced: %q", got)
	}
}

func TestMarkUntrackedPublishesWhereLinksAreDenied(t *testing.T) {
	dir := t.TempDir()
	denyLinks(t, syscall.EPERM)
	if err := MarkUntracked(dir); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, ".gitignore")); err != nil || string(got) != "*\n" {
		t.Fatalf("marker = %q, %v", got, err)
	}
}
