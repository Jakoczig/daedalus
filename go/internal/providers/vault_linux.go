//go:build linux

package providers

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func vaultKey(dir string, create bool) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	profile := hash(path)
	b, err := exec.CommandContext(ctx, "secret-tool", "lookup", "application", "daedalus", "profile", profile).Output()
	if err == nil {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
		if err == nil && len(key) == 32 {
			return key, nil
		}
	}
	if !create {
		return nil, errors.New("Secret Service vault locked or unavailable")
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "secret-tool", "store", "--label=DAEDALUS provider vault", "application", "daedalus", "profile", profile)
	cmd.Stdin = strings.NewReader(base64.StdEncoding.EncodeToString(key))
	if cmd.Run() != nil {
		return nil, errors.New("Unlock Secret Service and install secret-tool")
	}
	return key, nil
}
func secretCipher(dir string, create bool) (cipher.AEAD, error) {
	key, err := vaultKey(dir, create)
	if err != nil {
		return nil, err
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}
func sealSecret(dir string, b []byte) ([]byte, error) {
	a, err := secretCipher(dir, true)
	if err != nil {
		return nil, err
	}
	n := make([]byte, a.NonceSize())
	if _, err = rand.Read(n); err != nil {
		return nil, err
	}
	return a.Seal(n, n, b, []byte("DAEDALUS provider vault v1")), nil
}
func openSecret(dir string, b []byte) ([]byte, error) {
	a, err := secretCipher(dir, false)
	if err != nil {
		return nil, err
	}
	if len(b) < a.NonceSize() {
		return nil, errors.New("invalid vault")
	}
	return a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte("DAEDALUS provider vault v1"))
}
