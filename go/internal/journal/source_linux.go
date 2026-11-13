package journal

import "os"

func OpenReadOnly(path string) (*os.File, error) { return os.Open(path) }
