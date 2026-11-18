package history

import (
	"database/sql"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"path/filepath"
	"testing"
)

func TestG2JToG2KMigrationPreservesHistoryAndVerifiesBackup(t *testing.T) {
	dir := t.TempDir()
	if e := privatefs.Prepare(dir); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "daedalus.sqlite")
	f, e := privatefs.PrivateFile(path, true)
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
	db, e := sql.Open("sqlite3", dsn(path))
	if e != nil {
		t.Fatal(e)
	}
	for i, m := range migrations[:4] {
		if _, e = db.Exec(m); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec("INSERT INTO schema_migrations VALUES(?,?,?)", i+1, digest([]byte(m)), utc()); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.Exec("INSERT INTO commander VALUES('SYNTHETIC','Fixture',?)", utc()); e != nil {
		t.Fatal(e)
	}
	db.Close()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.Stats().Schema != schemaVersion {
		t.Fatal(s.Stats())
	}
	people, e := s.Commanders()
	if e != nil || len(people) != 1 {
		t.Fatal("history lost", people, e)
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "backups", "pre-migration-v4-*.sqlite"))
	if len(paths) != 1 {
		t.Fatal("missing migration backup", paths)
	}
	if _, e = verifyBackup(paths[0]); e != nil {
		t.Fatal(e)
	}
}
