package journal

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func durableIdentity(f *os.File) (string, error) {
	var i windows.ByHandleFileInformation
	if e := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &i); e != nil {
		return "", e
	}
	return fmt.Sprintf("win:%x:%x:%x:%x:%x", i.VolumeSerialNumber, i.FileIndexHigh, i.FileIndexLow, i.CreationTime.HighDateTime, i.CreationTime.LowDateTime), nil
}
