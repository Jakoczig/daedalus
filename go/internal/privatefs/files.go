// Package privatefs protects the new Go profile without touching legacy files.
package privatefs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func safePath(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for current := absolute; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err == nil && (info.Mode()&os.ModeSymlink != 0 || reparse(current)) {
			return errors.New("linked private storage is unsupported")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return nil
}

func Prepare(dir string) error {
	if err := safePath(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := safePath(dir); err != nil {
		return err
	}
	return protect(dir, true)
}

func file(dir, name string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") {
		return "", errors.New("invalid private filename")
	}
	target := filepath.Join(dir, name)
	if err := safePath(target); err != nil {
		return "", err
	}
	return target, nil
}

func Read(dir, name string) ([]byte, error) {
	target, err := file(dir, name)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("invalid private file")
	}
	if err := checkPrivate(target); err != nil {
		return nil, err
	}
	return os.ReadFile(target)
}

func Write(dir, name string, data []byte) error {
	target, err := file(dir, name)
	if err != nil {
		return err
	}
	if err := Prepare(dir); err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		if err := checkPrivate(target); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(dir, ".write-")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err = protect(temp.Name(), false); err != nil {
		return err
	}
	if _, err = temp.Write(data); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), target)
}

// Lock is released by the OS on process exit, including crashes.
func Lock(dir string) (func(), error) {
	if err := Prepare(dir); err != nil {
		return nil, err
	}
	target, err := file(dir, "profile.lock")
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = protect(target, false); err == nil {
		err = lockFile(f)
	}
	if err != nil {
		f.Close()
		return nil, errors.New("profile is in use or cannot be locked")
	}
	return func() { _ = f.Close() }, nil
}
