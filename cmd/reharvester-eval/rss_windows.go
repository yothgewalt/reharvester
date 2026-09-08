//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	psapiDLL                 = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = psapiDLL.NewProc("GetProcessMemoryInfo")
)

// processMemoryCounters mirrors PROCESS_MEMORY_COUNTERS from psapi.h.
//
// cb and PageFaultCount are DWORD; every other field is SIZE_T, which is
// pointer-sized. Declaring those as uintptr keeps the Go layout identical to
// the C one on both 386 and amd64 — the struct is passed by address with its
// own size, so a mismatch would be read as garbage rather than rejected.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// peakRSSMB reports peak resident set size in megabytes, and whether it could
// be measured. PeakWorkingSetSize is the Windows equivalent of ru_maxrss: the
// high-water mark of resident pages, reported in bytes.
//
// Table 2b's memory column and its scaling exponent come from this number, so
// the second return exists to keep "not measured" distinguishable from a real
// reading of zero.
func peakRSSMB() (float64, bool) {
	// LazyProc.Call panics when the symbol cannot be resolved. Resolving first
	// turns a missing psapi.dll into a declared gap instead of a crash in the
	// middle of a long evaluation run.
	if err := procGetProcessMemoryInfo.Find(); err != nil {
		return 0, false
	}
	proc, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, false
	}
	var counters processMemoryCounters
	counters.cb = uint32(unsafe.Sizeof(counters))
	ret, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(proc),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.cb),
	)
	if ret == 0 {
		return 0, false
	}
	return float64(counters.peakWorkingSetSize) / (1 << 20), true
}
