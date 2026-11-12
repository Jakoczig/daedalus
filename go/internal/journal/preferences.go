package journal

import (
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/Jakoczig/daedalus/go/internal/privatefs"
)

type sourcePreference struct {
	Version   int
	Directory string
}

// Call while holding the enclosing application's exclusive private-profile lock.
func LoadOverride(profile string) (string, error) {
	raw, err := privatefs.Read(profile, "journal.json")
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("journal preference is unreadable or unsafe")
	}
	var value sourcePreference
	if json.Unmarshal(raw, &value) != nil || value.Version != 1 {
		return "", errors.New("journal preference is damaged; select a directory again")
	}
	return value.Directory, nil
}
func SaveOverride(profile, dir string) error {
	if dir != "" {
		if err := ValidateDir(dir); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(sourcePreference{Version: 1, Directory: dir})
	return privatefs.Write(profile, "journal.json", raw)
}
