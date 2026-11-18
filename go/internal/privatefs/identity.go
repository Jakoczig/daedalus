package privatefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// IdentityProfile adopts the existing native profile in place. A versioned atomic
// identity manifest keeps the original lock and Linux Secret Service path stable.
// No database, certificate, permission, token or legacy application file is copied.
func IdentityProfile(config string) (string, error) {
	current := filepath.Join(config, "eridion-go")
	old := filepath.Join(config, "daedalus-go")
	type manifest struct {
		Version int    `json:"version"`
		Storage string `json:"storage"`
	}
	b, e := Read(current, "identity-v1.json")
	if e == nil {
		var m manifest
		if json.Unmarshal(b, &m) != nil || m.Version != 1 || (m.Storage != "eridion-go" && m.Storage != "daedalus-go") {
			return "", errors.New("Invalid ERIDION identity manifest; preserve both native profiles for recovery")
		}
		target := filepath.Join(config, m.Storage)
		if _, e = os.Stat(target); e != nil {
			return "", errors.New("Adopted native profile missing; restore its original directory before starting ERIDION")
		}
		if e = safePath(target); e != nil {
			return "", e
		}
		return target, nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	storage := "eridion-go"
	if info, e := os.Lstat(old); e == nil {
		if !info.IsDir() {
			return "", errors.New("Existing native profile is not a directory")
		}
		if e = safePath(old); e != nil {
			return "", e
		}
		// Never overwrite an interrupted or unrelated new-profile installation.
		if entries, e := os.ReadDir(current); e == nil {
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), ".write-") || !entry.Type().IsRegular() {
					return "", errors.New("Both native profiles exist without an identity manifest; preserve and resolve them manually")
				}
			}
		}
		storage = "daedalus-go"
	} else if !os.IsNotExist(e) {
		return "", e
	}
	raw, _ := json.Marshal(manifest{1, storage})
	if e = Write(current, "identity-v1.json", raw); e != nil {
		return "", e
	}
	return filepath.Join(config, storage), nil
}
