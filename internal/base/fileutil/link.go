package fileutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
)

// linkFile is a test seam: a volume that refuses hard links cannot be
// provoked portably on a real filesystem.
var linkFile = os.Link

// LinkNoReplace publishes src at dst without replacing whatever already sits
// there. It links where the filesystem allows links and copies exclusively
// where it denies them (Android/Termux, FAT, some network mounts), because the
// guarantee callers hold is "dst appeared complete or not at all", not "dst and
// src share an inode". A destination that exists is fs.ErrExist either way.
func LinkNoReplace(src, dst string) error {
	err := linkFile(src, dst)
	if err == nil || !linksDenied(err) {
		return err
	}
	return copyNoReplace(src, dst)
}

// linksDenied is the errno class that says this volume does not do hard links,
// as distinct from the one that says this link was refused for its own reason.
func linksDenied(err error) bool {
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, errors.ErrUnsupported)
}

// copyNoReplace writes src to dst under O_EXCL, so a destination created
// between the refused link and this open is still never replaced.
func copyNoReplace(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}
