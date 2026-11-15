//go:build !windows

package history

import (
	"errors"
	"golang.org/x/sys/unix"
)

func localFilesystem(path string) error {
	var s unix.Statfs_t
	if err := unix.Statfs(path, &s); err != nil {
		return err
	}
	switch uint64(s.Type) {
	case 0x6969, 0xff534d42, 0x517b, 0x1021997, 0x65735546:
		return errors.New("SQLite WAL requires a supported local filesystem")
	}
	return nil
}
