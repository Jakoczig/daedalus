//go:build !windows

package webapp

import (
	"os"
	"testing"
)

func makeUnsafeRemoteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
}
