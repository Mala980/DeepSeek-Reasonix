package config

import (
	"os"
	"path/filepath"
	"strings"
)

// sharedTempRoot is the temporary root a cross-process registry converges on:
// one answer per host, whichever TMPDIR each process was started with.
func sharedTempRoot() string {
	return firstAccepting(sharedTempRootCandidates())
}

func sharedTempRootCandidates() []string {
	return sharedTempRootCandidatesFor("/tmp", os.Getenv("PREFIX"), os.TempDir())
}

// sharedTempRootCandidatesFor orders the roots a host offers: its /tmp, then
// the Termux prefix, where $PREFIX/tmp is the invariant a host without /tmp has
// instead, then the process TMPDIR — the only answer left when a host names
// neither, and the one a per-process override can split.
func sharedTempRootCandidatesFor(systemTmp, prefix, processTmp string) []string {
	candidates := []string{systemTmp}
	if p := strings.TrimSpace(prefix); p != "" {
		candidates = append(candidates, filepath.Join(p, "tmp"))
	}
	if t := strings.TrimSpace(processTmp); t != "" && t != systemTmp {
		candidates = append(candidates, t)
	}
	return candidates
}

// firstAccepting is the first candidate that holds a file, or "" when none does.
func firstAccepting(candidates []string) string {
	for _, dir := range candidates {
		if dirAcceptsFiles(dir) {
			return dir
		}
	}
	return ""
}

// dirAcceptsFiles answers the only question a registry root can be asked: it
// holds the file. A name is not a guarantee, and a root that refuses the write
// is not a root whatever it is called.
func dirAcceptsFiles(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	f, err := os.CreateTemp(dir, ".reasonix-root-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
