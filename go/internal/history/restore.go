package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	sqlite "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type restoreIntent struct {
	Hash string `json:"sha256"`
}

func (s *Store) StageRestore(ctx context.Context, source string) error {
	return StageRecovery(ctx, s.dir, source)
}

// StageRecovery is also usable when the active DB is corrupt. It never opens it
// for mutation: only a verified selected backup is staged, pending an app restart.
func StageRecovery(ctx context.Context, dir, source string) error {
	if !filepath.IsAbs(source) {
		return errors.New("Select an absolute private backup path")
	}
	if err := privatefs.CheckPrivate(source); err != nil {
		return err
	}
	if err := privatefs.Prepare(dir); err != nil {
		return err
	}
	if _, err := verifyBackup(source); err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: databaseURIPath(source)}
	db, err := sql.Open("sqlite3", u.String()+"?mode=ro&immutable=1&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var v int
	var sum string
	if err = db.QueryRow("SELECT version,checksum FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&v, &sum); err != nil || v < 1 || v > len(migrations) || sum != digest([]byte(migrations[v-1])) {
		return errors.New("Backup schema is not supported")
	}
	destination := filepath.Join(dir, "restore.pending.sqlite")
	f, err := privatefs.PrivateFile(destination, true)
	if err != nil {
		return err
	}
	f.Close()
	if _, err = db.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return err
	}
	pending, err := sql.Open("sqlite3", dsn(destination))
	if err != nil {
		return err
	}
	pending.SetMaxOpenConns(1)
	_, err = pending.Exec(`BEGIN IMMEDIATE; UPDATE publication_policy SET mode='Never'; DELETE FROM publication_exceptions; UPDATE provider_consent SET config_json=json_set(config_json,'$.upload',json('false'),'$.paused',json('true')); UPDATE publication_candidates SET status='held',reason='Restored backup requires renewed consent' WHERE status IN ('eligible','queued'); COMMIT;`)
	if err != nil {
		pending.Close()
		return err
	}
	if err = pending.Close(); err != nil {
		return err
	}
	hash, err := verifyBackup(destination)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(restoreIntent{Hash: hash})
	return privatefs.Write(dir, "restore.pending.json", raw)
}
func consumeRestore(dir string) error {
	raw, err := privatefs.Read(dir, "restore.pending.json")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var intent restoreIntent
	if json.Unmarshal(raw, &intent) != nil || len(intent.Hash) != 64 {
		return errors.New("Restore intent damaged; originals retained")
	}
	path := filepath.Join(dir, "daedalus.sqlite")
	staged := filepath.Join(dir, "restore.pending.sqlite")
	if _, err = os.Stat(staged); os.IsNotExist(err) {
		hash, e := verifyBackup(path)
		if e != nil || hash != intent.Hash {
			return errors.New("Interrupted restore requires owner recovery")
		}
		return os.Remove(filepath.Join(dir, "restore.pending.json"))
	}
	hash, err := verifyBackup(staged)
	if err != nil || hash != intent.Hash {
		return errors.New("Staged restore checksum/integrity failure; originals retained")
	}
	backupDir := filepath.Join(dir, "backups")
	if err = privatefs.Prepare(backupDir); err != nil {
		return err
	}
	if _, err = os.Stat(path); err == nil {
		db, e := sql.Open("sqlite3", dsn(path))
		if e != nil {
			return e
		}
		db.SetMaxOpenConns(1)
		e = db.Ping()
		var integrity string
		if e == nil {
			e = db.QueryRow("PRAGMA integrity_check").Scan(&integrity)
			if e == nil && integrity != "ok" {
				e = errors.New("existing integrity check failed")
			}
		}
		if e != nil {
			db.Close()
			var se sqlite.Error
			corrupt := errors.As(e, &se) && (se.Code == sqlite.ErrNotADB || se.Code == sqlite.ErrCorrupt)
			if !corrupt && integrity == "" {
				return errors.New("Existing database could not be opened safely; originals retained")
			}
			evidence := filepath.Join(backupDir, "corrupt-before-restore-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
			if e = privatefs.Prepare(evidence); e != nil {
				return e
			}
			for _, suffix := range []string{"-wal", "-shm", ""} {
				src := path + suffix
				if _, e = os.Stat(src); os.IsNotExist(e) {
					continue
				}
				if e = privatefs.CheckPrivate(src); e != nil {
					return e
				}
				if e = os.Rename(src, filepath.Join(evidence, "daedalus.sqlite"+suffix)); e != nil {
					return e
				}
			}
		} else {
			holder := &Store{writer: db, dir: dir, path: path}
			dest := filepath.Join(backupDir, "pre-restore-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".sqlite")
			_, e = holder.Backup(context.Background(), dest)
			if e == nil {
				var busy, log, done int
				e = db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &done)
				if busy != 0 {
					e = errors.New("Restore checkpoint busy")
				}
			}
			db.Close()
			if e != nil {
				return e
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.Rename(staged, path); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, "restore.pending.json"))
}
