package privatefs

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsRejectsBroadPrivateFileACL(t *testing.T) {
	dir := t.TempDir()
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "secret.json")
	if err := Write(dir, "secret.json", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, _ := descriptor.DACL()
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, "secret.json"); err == nil {
		t.Fatal("world-readable secret accepted")
	}
	// Restore a restrictive ACL so test cleanup can remove its own fixture.
	if err := protect(path, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
}
