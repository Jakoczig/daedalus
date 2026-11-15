package history

import (
	"errors"
	"golang.org/x/sys/windows"
	"path/filepath"
	"strings"
)

func localFilesystem(path string) error {
	if strings.HasPrefix(path, `\\`) {
		return errors.New("SQLite WAL requires a local profile")
	}
	root, _ := windows.UTF16PtrFromString(filepath.VolumeName(path) + `\`)
	if windows.GetDriveType(root) == windows.DRIVE_REMOTE {
		return errors.New("SQLite WAL requires a local profile")
	}
	return nil
}
