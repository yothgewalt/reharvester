//go:build !windows

package main

import (
	"path/filepath"
	"syscall"
)

// freeGB reports free space for the filesystem holding path. The data
// directory usually does not exist on a first run, so it walks up to the
// nearest existing ancestor rather than reporting nothing.
func freeGB(path string) (float64, bool) {
	p, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}
	for {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err == nil {
			return float64(st.Bavail) * float64(st.Bsize) / (1 << 30), true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return 0, false
		}
		p = parent
	}
}
