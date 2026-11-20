//go:build windows

package atlas

import (
	"golang.org/x/sys/windows"
	"os"
)

// Truncate alone zero-fills ordinary Windows files. Mark test-only cold cache
// files sparse first so logical 128 MiB pressure does not become bulk disk I/O
// competing with the existing journal/SQLite regression packages.
func sparseColdFile(f *os.File) error {
	var returned uint32
	return windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &returned, nil)
}
