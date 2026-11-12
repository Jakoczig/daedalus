package privatefs

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"strings"
)

func reparse(path string) bool {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	attributes, err := windows.GetFileAttributes(pointer)
	return err == nil && attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
func identity() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}
func protect(path string, dir bool) error {
	sid, err := identity()
	if err != nil {
		return err
	}
	existing, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := existing.Owner()
	if err != nil || owner.String() != sid {
		return errors.New("private storage has a different owner")
	}
	flags := ""
	if dir {
		flags = "OICI"
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;" + flags + ";FA;;;SY)(A;" + flags + ";FA;;;" + sid + ")")
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}
func checkPrivate(path string) error {
	sid, err := identity()
	if err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner.String() != sid {
		return errors.New("private file owner mismatch")
	}
	// Files created by this package have exactly these two full-access principals.
	sddl := descriptor.String()
	for _, entry := range strings.Split(sddl, "(")[1:] {
		ace := strings.TrimSuffix(entry, ")")
		fields := strings.Split(ace, ";")
		if len(fields) != 6 || fields[0] != "A" || (fields[5] != "SY" && fields[5] != sid) {
			return errors.New("private file ACL is too broad")
		}
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return errors.New("missing private file ACL")
	}
	return nil
}
func lockFile(f *os.File) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
}
