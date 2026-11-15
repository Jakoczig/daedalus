package history

import (
	"context"
	"database/sql"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"os"
	"path/filepath"
	"testing"
)

func TestCorruptDatabaseRecoveryPreservesOriginalAndDisablesPublication(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.writer.Exec("INSERT INTO commander VALUES('SYNTHETIC','Fixture',?)", utc()); err != nil {
		t.Fatal(err)
	}
	if err = s.SetPublicationPolicy("SYNTHETIC", "After sale"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetLiveException("SYNTHETIC", "eddn", "travel", true, true); err != nil {
		t.Fatal(err)
	}
	backupDir := t.TempDir()
	privatefs.Prepare(backupDir)
	backup := filepath.Join(backupDir, "fixture.sqlite")
	if _, err = s.Backup(context.Background(), backup); err != nil {
		t.Fatal(err)
	}
	s.Close()
	broken := []byte("synthetic damaged sqlite retained for diagnosis")
	if err = os.WriteFile(filepath.Join(dir, "daedalus.sqlite"), broken, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(dir); err == nil {
		t.Fatal("corrupt database silently reset")
	}
	if err = StageRecovery(context.Background(), dir, backup); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	policy, err := recovered.PublicationPolicy("SYNTHETIC")
	if err != nil || policy.Mode != "Never" || len(policy.Exceptions) != 0 {
		t.Fatalf("restored permissions: %+v %v", policy, err)
	}
	found := false
	filepath.WalkDir(dir, func(path string, entry os.DirEntry, e error) error {
		if e == nil && !entry.IsDir() && filepath.Base(path) == "daedalus.sqlite" {
			raw, _ := os.ReadFile(path)
			if string(raw) == string(broken) {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Fatal("damaged original lost during explicit restore")
	}
}
func TestMigrationBackupFailureLeavesPriorSchemaUsable(t *testing.T) {
	dir := t.TempDir()
	privatefs.Prepare(dir)
	path := filepath.Join(dir, "daedalus.sqlite")
	f, err := privatefs.PrivateFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	db, err := sql.Open("sqlite3", dsn(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO schema_migrations VALUES(1,?,?)", digest([]byte(schema)), utc())
	db.Close()
	if err = os.WriteFile(filepath.Join(dir, "backups"), []byte("blocked destination"), 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir); err == nil {
		s.Close()
		t.Fatal("migration proceeded without verified backup")
	}
	db, err = sql.Open("sqlite3", dsn(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err = db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 1 {
		t.Fatal("prior schema changed", version, err)
	}
}
