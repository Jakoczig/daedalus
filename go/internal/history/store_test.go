package history

import (
	"context"
	"database/sql"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicEventsCheckpointCacheAndBackup(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	src := journal.Source{ID: "fixture-source", Path: filepath.Join(dir, "Journal.fixture.log"), Identity: "fixture-identity", Size: 100}
	cp, err := s.Resume(src)
	if err != nil {
		t.Fatal(err)
	}
	cp.Offset = 100
	cp.Sequence = 1
	cp.Head = []byte("head")
	cp.Anchor = []byte("anchor")
	e, err := journal.ParseLine([]byte(`{"timestamp":"2026-11-15T00:00:00Z","event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture"}`), journal.SourcePosition{FileID: src.ID, Sequence: 1, Offset: 0, Length: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Commit(context.Background(), cp, []journal.Record{{Event: e, Phase: journal.Replay}}, []byte("private-cache")); err != nil {
		t.Fatal(err)
	}
	if err = s.Commit(context.Background(), cp, []journal.Record{{Event: e, Phase: journal.Replay}}, []byte("private-cache")); err != nil {
		t.Fatal(err)
	}
	if n := s.Stats().Events; n != 1 {
		t.Fatalf("duplicates: %d", n)
	}
	backupDir := t.TempDir()
	if err := privatefs.Prepare(backupDir); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(backupDir, "backup.sqlite")
	if _, err = s.Backup(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(dest); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cp, err = s.Resume(src)
	if err != nil || cp.Offset != 100 {
		t.Fatalf("checkpoint: %+v %v", cp, err)
	}
	cached, err := s.Cache()
	if err != nil || string(cached) != "private-cache" {
		t.Fatalf("cache lost: %q %v", cached, err)
	}
}
func TestCorruptDatabasePreserved(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "daedalus.sqlite")
	os.WriteFile(p, []byte("broken private evidence"), 0600)
	if s, e := Open(dir); e == nil {
		s.Close()
		t.Fatal("corrupt accepted")
	}
	raw, _ := os.ReadFile(p)
	if string(raw) != "broken private evidence" {
		t.Fatal("damaged DB overwritten")
	}
}

func TestRestoreIsExplicitVerifiedAndPreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	backupDir := t.TempDir()
	privatefs.Prepare(backupDir)
	backup := filepath.Join(backupDir, "selected.sqlite")
	if _, e = s.Backup(context.Background(), backup); e != nil {
		t.Fatal(e)
	}
	if e = s.StageRestore(context.Background(), backup); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(dir, "restore.pending.sqlite")); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	paths, _ := filepath.Glob(filepath.Join(dir, "backups", "pre-restore-*.sqlite"))
	if len(paths) != 1 {
		t.Fatalf("original not backed up: %v", paths)
	}
}

func TestMigrationBacksUpBeforeChangingSchema(t *testing.T) {
	dir := t.TempDir()
	privatefs.Prepare(dir)
	p := filepath.Join(dir, "daedalus.sqlite")
	f, e := privatefs.PrivateFile(p, true)
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
	db, e := sql.Open("sqlite3", dsn(p))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(schema); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO schema_migrations VALUES(1,?,?)", digest([]byte(schema)), utc()); e != nil {
		t.Fatal(e)
	}
	db.Close()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.Stats().Schema != schemaVersion {
		t.Fatal("migration not applied")
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "backups", "pre-migration-v1-*.sqlite"))
	if len(paths) != 1 {
		t.Fatalf("missing verified migration backup: %v", paths)
	}
	if _, e = verifyBackup(paths[0]); e != nil {
		t.Fatal(e)
	}
}
func TestCheckpointRefusalRollsBackAllRows(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	src := journal.Source{ID: "quarantined", Identity: "fixture"}
	cp, e := s.Resume(src)
	if e != nil {
		t.Fatal(e)
	}
	s.Quarantine(src.ID)
	event, e := journal.ParseLine([]byte(`{"event":"LoadGame","FID":"SYNTHETIC-ROLLBACK"}`), journal.SourcePosition{FileID: src.ID, Sequence: 1})
	if e != nil {
		t.Fatal(e)
	}
	cp.Offset = 50
	if e = s.Commit(context.Background(), cp, []journal.Record{{Event: event}}, []byte("must-not-commit")); e == nil {
		t.Fatal("refused checkpoint committed")
	}
	if s.Stats().Events != 0 {
		t.Fatal("partial event commit")
	}
	cache, _ := s.Cache()
	if len(cache) > 0 {
		t.Fatal("partial cache commit")
	}
	var n int
	s.reader.QueryRow("SELECT count(*) FROM commander").Scan(&n)
	if n != 0 {
		t.Fatal("normalized rows escaped rollback")
	}
}
