package journal

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// OpenReadOnly permits the game to atomically rename/replace an open source.
// No write access is requested; long absolute and UNC paths remain supported.
func OpenReadOnly(path string) (*os.File, error) {
	full, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(full, `\\?\`) {
		if strings.HasPrefix(full, `\\`) {
			full = `\\?\UNC\` + strings.TrimPrefix(full, `\\`)
		} else {
			full = `\\?\` + full
		}
	}
	name, err := windows.UTF16PtrFromString(full)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
