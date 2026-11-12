package journal

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type DiscoveryOptions struct {
	Override   string
	Roots      []string
	SavedGames string
}
type DiscoveryResult struct {
	Directory  string
	Found      bool
	Source     string
	Candidates []string
}

func ValidateDir(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("journal directory must be absolute")
	}
	f, err := os.Open(path)
	if err != nil {
		return errors.New("journal directory unavailable or unreadable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.IsDir() {
		return errors.New("journal source is not a directory")
	}
	if _, err = f.ReadDir(1); err != nil && !errors.Is(err, io.EOF) {
		return errors.New("journal directory unreadable")
	}
	return nil
}

func Discover(ctx context.Context, opts DiscoveryOptions) (DiscoveryResult, error) {
	var result DiscoveryResult
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if opts.Override != "" {
		result = DiscoveryResult{Directory: opts.Override, Source: "override", Candidates: []string{opts.Override}}
		err := ValidateDir(opts.Override)
		result.Found = err == nil
		return result, err
	}
	if opts.SavedGames == "" && opts.Roots == nil {
		opts.SavedGames, opts.Roots = platformLocations()
	}
	try := func(path, source string) bool {
		result.Candidates = append(result.Candidates, path)
		if ValidateDir(path) == nil {
			result.Directory = path
			result.Source = source
			result.Found = true
			return true
		}
		return false
	}
	if opts.SavedGames != "" && try(filepath.Join(opts.SavedGames, "Frontier Developments", "Elite Dangerous"), "saved_games") {
		return result, nil
	}
	if len(opts.Roots) > 32 {
		return result, errors.New("too many Steam roots")
	}
	seen := map[string]bool{}
	roots := append([]string(nil), opts.Roots...)
	for i := 0; i < len(roots); i++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		root := filepath.Clean(roots[i])
		if !filepath.IsAbs(root) || seen[root] {
			continue
		}
		seen[root] = true
		for _, file := range []string{filepath.Join(root, "steamapps", "libraryfolders.vdf"), filepath.Join(root, "config", "libraryfolders.vdf")} {
			libraries, err := libraryPaths(file)
			if err != nil {
				return result, err
			}
			for _, p := range libraries {
				if !seen[p] {
					if len(roots) >= 32 {
						return result, errors.New("Steam library discovery limit exceeded")
					}
					roots = append(roots, p)
				}
			}
		}
		users := filepath.Join(root, "steamapps", "compatdata", "359320", "pfx", "drive_c", "users")
		f, err := os.Open(users)
		if err != nil {
			continue
		}
		entries, readErr := f.ReadDir(33)
		f.Close()
		if len(entries) > 32 {
			return result, errors.New("Proton user discovery limit exceeded")
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			continue
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if entry.IsDir() && try(filepath.Join(users, entry.Name(), "Saved Games", "Frontier Developments", "Elite Dangerous"), "proton") {
				return result, nil
			}
		}
	}
	return result, nil
}

// Parse only quoted VDF path fields (including the old numbered-string form).
// No recursive drive scan or platform shell commands are involved.
func libraryPaths(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, errors.New("cannot read Steam library configuration")
	}
	if len(raw) > 1<<20 {
		return nil, errors.New("Steam library configuration too large")
	}
	var tokens []string
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		start := i
		i++
		for i < len(raw) {
			if raw[i] == '\\' {
				i += 2
				continue
			}
			if raw[i] == '"' {
				break
			}
			i++
		}
		if i >= len(raw) {
			break
		}
		v, err := strconv.Unquote(string(raw[start : i+1]))
		if err == nil {
			tokens = append(tokens, v)
		}
	}
	var paths []string
	for i := 0; i+1 < len(tokens); i++ {
		_, numeric := strconv.Atoi(tokens[i])
		if strings.EqualFold(tokens[i], "path") || numeric == nil {
			p := filepath.Clean(tokens[i+1])
			if filepath.IsAbs(p) {
				paths = append(paths, p)
			}
		}
	}
	return paths, nil
}
