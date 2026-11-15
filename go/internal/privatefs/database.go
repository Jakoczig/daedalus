package privatefs

import (
	"os"
	"path/filepath"
)

// PrivateFile validates ancestors and protects only this file, never an export directory.
func PrivateFile(path string, exclusive bool) (*os.File, error) {
	if err := safePath(path); err != nil {
		return nil, err
	}
	if err := checkPrivate(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, os.ErrPermission
		}
		if err = checkPrivate(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	flags := os.O_RDWR | os.O_CREATE
	if exclusive {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return nil, err
	}
	if err = protect(path, false); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func CheckPrivate(path string) error {
	if err := safePath(path); err != nil {
		return err
	}
	return checkPrivate(path)
}
