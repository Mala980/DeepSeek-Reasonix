//go:build linux

package builtin

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplace(src, dst string) error {
	err := unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) {
		if _, statErr := os.Lstat(dst); statErr == nil {
			return &os.LinkError{Op: "rename", Old: src, New: dst, Err: unix.EEXIST}
		} else if !os.IsNotExist(statErr) {
			return &os.LinkError{Op: "rename", Old: src, New: dst, Err: statErr}
		}
		if rerr := unix.Renameat(unix.AT_FDCWD, src, unix.AT_FDCWD, dst); rerr != nil {
			return &os.LinkError{Op: "rename", Old: src, New: dst, Err: rerr}
		}
		return nil
	}
	return &os.LinkError{Op: "rename", Old: src, New: dst, Err: err}
}
