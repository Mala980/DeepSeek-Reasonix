//go:build linux

package fileutil

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func RenameNoReplace(oldPath, newPath string) error {
	err := unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) {
		if _, statErr := os.Lstat(newPath); statErr == nil {
			return unix.EEXIST
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		return unix.Renameat(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath)
	}
	return err
}
