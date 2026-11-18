package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

func verifyBackup(path string) (string, error) {
	u := url.URL{Scheme: "file", Path: databaseURIPath(path)}
	db, err := sql.Open("sqlite3", u.String()+"?mode=ro&immutable=1&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return "", err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var v string
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&v); err != nil || v != "ok" {
		return "", errors.New("Backup integrity check failed")
	}
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return "", err
	}
	bad := rows.Next()
	rows.Close()
	if bad {
		return "", errors.New("Backup foreign-key check failed")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func (s *Store) Backup(ctx context.Context, destination string) (string, error) {
	if !filepath.IsAbs(destination) || filepath.Clean(destination) == s.path {
		return "", errors.New("Choose a distinct absolute private backup path")
	}
	f, err := privatefs.PrivateFile(destination, true)
	if err != nil {
		return "", err
	}
	f.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("Database closed")
	}
	if _, err = s.writer.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return "", err
	}
	hash, err := verifyBackup(destination)
	if err != nil {
		return "", err
	}
	if err = privatefs.Write(filepath.Dir(destination), filepath.Base(destination)+".sha256", []byte(hash+"  "+filepath.Base(destination)+"\n")); err != nil {
		return "", err
	}
	return hash, nil
}
