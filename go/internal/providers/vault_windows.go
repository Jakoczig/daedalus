//go:build windows

package providers

import (
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

func cryptSecret(data []byte, seal bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty secret")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var err error
	if seal {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, errors.New("Windows credential vault unavailable")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
func sealSecret(_ string, b []byte) ([]byte, error) { return cryptSecret(b, true) }
func openSecret(_ string, b []byte) ([]byte, error) { return cryptSecret(b, false) }
