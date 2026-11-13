package journal

import "golang.org/x/sys/windows"

func platformLocations() (string, []string) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_SavedGames, 0)
	if err != nil {
		return "", nil
	}
	return path, nil
}
