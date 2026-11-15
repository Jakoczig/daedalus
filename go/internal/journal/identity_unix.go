//go:build !windows

package journal

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

func durableIdentity(f *os.File) (string, error) {
	var i unix.Stat_t
	if e := unix.Fstat(int(f.Fd()), &i); e != nil {
		return "", e
	}
	return fmt.Sprintf("unix:%x:%x", i.Dev, i.Ino), nil
}
