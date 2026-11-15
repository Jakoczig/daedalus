package providers

import (
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"os"
)

type nativeVault struct{ dir string }

func newVault(dir string) Vault { return &nativeVault{dir} }
func (v *nativeVault) Load() (map[string]Credential, error) {
	b, err := privatefs.Read(v.dir, "provider-vault.bin")
	if os.IsNotExist(err) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := openSecret(v.dir, b)
	if err != nil {
		return nil, err
	}
	var out map[string]Credential
	err = json.Unmarshal(raw, &out)
	return out, err
}
func (v *nativeVault) Save(c map[string]Credential) error {
	if old, err := privatefs.Read(v.dir, "provider-vault.bin"); err == nil {
		if _, err := openSecret(v.dir, old); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	sealed, err := sealSecret(v.dir, raw)
	if err != nil {
		return err
	}
	return privatefs.Write(v.dir, "provider-vault.bin", sealed)
}
