//go:build !windows

package main

import (
	"runtime"
	"syscall"
)

// peakRSSMB reports peak resident set size in megabytes, and whether the
// platform could measure it at all. Table 2b's memory column and its scaling
// exponent come from this number, so "not measured" has to be distinguishable
// from "measured zero" — a bare 0 would print as a real reading.
func peakRSSMB() (float64, bool) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, false
	}
	// Darwin reports ru_maxrss in bytes; Linux in kilobytes.
	if runtime.GOOS == "darwin" {
		return float64(ru.Maxrss) / (1 << 20), true
	}
	return float64(ru.Maxrss) / 1024, true
}
