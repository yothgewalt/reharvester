//go:build windows

package main

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32DLL             = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = kernel32DLL.NewProc("GetDiskFreeSpaceExW")
)

// freeGB reports free space for the volume holding path, in gigabytes.
//
// GetDiskFreeSpaceExW wants a directory that exists, and the data directory
// usually does not on a first run, so it walks up to the nearest existing
// ancestor — the same approach as the Unix implementation.
//
// The first out-parameter is free bytes available *to the calling user*, which
// is the honest figure under a disk quota: the volume's total free space can be
// far larger than what this process is allowed to write.
func freeGB(path string) (float64, bool) {
	if err := procGetDiskFreeSpaceExW.Find(); err != nil {
		return 0, false
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}
	for {
		if free, ok := freeBytesAt(p); ok {
			return float64(free) / (1 << 30), true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return 0, false
		}
		p = parent
	}
}

func freeBytesAt(dir string) (uint64, bool) {
	wide, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var freeToCaller, total, totalFree uint64
	ret, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(wide)),
		uintptr(unsafe.Pointer(&freeToCaller)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if ret == 0 {
		return 0, false
	}
	return freeToCaller, true
}
