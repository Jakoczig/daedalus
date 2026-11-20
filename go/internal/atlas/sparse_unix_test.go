//go:build !windows

package atlas

import "os"

// ftruncate extends a regular Unix file with holes without writing the payload.
func sparseColdFile(f *os.File) error { return nil }
