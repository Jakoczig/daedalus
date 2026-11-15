//go:build windows

package gamestate

import (
	"syscall"
	"unsafe"
)

func peakRSS() uint64 {
	type counters struct {
		Size, Faults                                                                                        uint32
		Peak, Working, QuotaPeakPaged, QuotaPaged, QuotaPeakNonPaged, QuotaNonPaged, Pagefile, PeakPagefile uintptr
	}
	value := counters{}
	value.Size = uint32(unsafe.Sizeof(value))
	dll := syscall.NewLazyDLL("psapi.dll")
	function := dll.NewProc("GetProcessMemoryInfo")
	ok, _, _ := function.Call(^uintptr(0), uintptr(unsafe.Pointer(&value)), uintptr(value.Size))
	if ok == 0 {
		return 0
	}
	return uint64(value.Peak)
}
