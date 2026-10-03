package installsource

import (
	"os"
	"path/filepath"
	"runtime"

	"reasonix/internal/ext/pluginpkg"
)

func installPluginCopy(pkg pluginpkg.Package, sourceRoot, target string, replace bool) (string, func(), error) {
	if runtime.GOOS != "windows" || !replace {
		return target, nil, installCopiedPlugin(pkg, sourceRoot, target, replace)
	}
	// Windows directory watches deny renaming their ancestors. Publishing a
	// complete sibling through the state root pointer leaves those trees intact.
	root, err := stagePluginCopy(pkg, sourceRoot, target)
	if err != nil {
		return "", nil, err
	}
	return root, func() { _ = os.RemoveAll(root) }, nil
}

func stagePluginCopy(pkg pluginpkg.Package, sourceRoot, target string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(filepath.Dir(target), "."+filepath.Base(target)+".staging-")
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := copyDir(sourceRoot, staging, tarballTotalLimit); err != nil {
		return "", err
	}
	// Copying a symlink must not silently remove approved capabilities.
	if err := verifyCopiedCapabilities(pkg, staging); err != nil {
		return "", err
	}
	if err := os.Chmod(staging, 0o755); err != nil {
		return "", err
	}
	complete = true
	return staging, nil
}
