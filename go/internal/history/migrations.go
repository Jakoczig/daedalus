package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"path/filepath"
	"time"
)

var migrations = []string{schema, `CREATE INDEX events_commander_id ON journal_events(commander_fid,event_id);
CREATE INDEX events_commander_type_id ON journal_events(commander_fid,type,event_id);
CREATE TABLE derived_statistics(loadout_hash TEXT NOT NULL,formula_version TEXT NOT NULL,result_json TEXT NOT NULL,computed_at TEXT NOT NULL,PRIMARY KEY(loadout_hash,formula_version));
CREATE TABLE reconciliation_state(commander_fid TEXT NOT NULL,provider TEXT NOT NULL,last_attempt TEXT,last_success TEXT,outcome TEXT NOT NULL,PRIMARY KEY(commander_fid,provider));
CREATE TABLE station_reference(provider TEXT NOT NULL,market_id TEXT NOT NULL,system_address TEXT,system_name TEXT NOT NULL,name TEXT NOT NULL,station_type TEXT,arrival_ls REAL,x REAL,y REAL,z REAL,services_json TEXT NOT NULL,observed_at TEXT,fetched_at TEXT NOT NULL,expires_at TEXT NOT NULL,PRIMARY KEY(provider,market_id));
CREATE INDEX station_reference_system ON station_reference(system_address,expires_at);`, `CREATE TABLE sidecar_snapshots(id INTEGER PRIMARY KEY,source_event_id INTEGER NOT NULL REFERENCES journal_events(event_id),commander_fid TEXT NOT NULL REFERENCES commander(fid),kind TEXT NOT NULL,content_hash TEXT NOT NULL,observed_at TEXT NOT NULL,raw_json TEXT NOT NULL,UNIQUE(source_event_id,content_hash));
CREATE INDEX sidecar_commander_kind_time ON sidecar_snapshots(commander_fid,kind,observed_at);`, `CREATE INDEX provenance_current ON field_provenance(commander_fid,entity,field,source,id DESC);
CREATE INDEX requests_provider_time ON provider_requests(provider,id DESC);
CREATE INDEX station_coordinates ON station_reference(x,y,z);
CREATE INDEX external_type_entity ON external_snapshots(provider,entity_type,entity_id,id DESC);`}

func init() { migrations = append(migrations, fieldSchema, operationsSchema) }

func migrate(db *sql.DB, dir string) error {
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&count); err != nil {
		return err
	}
	version := 0
	if count != 0 {
		rows, err := db.Query("SELECT version,checksum FROM schema_migrations ORDER BY version")
		if err != nil {
			return err
		}
		for rows.Next() {
			var v int
			var checksum string
			if err = rows.Scan(&v, &checksum); err != nil {
				rows.Close()
				return err
			}
			if v != version+1 || v > len(migrations) || checksum != digest([]byte(migrations[v-1])) {
				rows.Close()
				return errors.New("Unsupported migration checksum/version; original database preserved")
			}
			version = v
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	} else {
		var tables int
		if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil || tables > 0 {
			return errors.New("Unrecognized database schema; original preserved")
		}
	}
	for version < len(migrations) {
		if count != 0 {
			backupDir := filepath.Join(dir, "backups")
			if err := privatefs.Prepare(backupDir); err != nil {
				return err
			}
			destination := filepath.Join(backupDir, fmt.Sprintf("pre-migration-v%d-%s.sqlite", version, time.Now().UTC().Format("20060102T150405.000000000Z")))
			holder := &Store{writer: db, dir: dir, path: filepath.Join(dir, "daedalus.sqlite")}
			if _, err := holder.Backup(context.Background(), destination); err != nil {
				return fmt.Errorf("migration backup failed: %w", err)
			}
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(migrations[version]); err == nil {
			if version == 4 {
				err = backfillFieldTally(tx)
			}
			if version == 5 {
				err = backfillOperations(tx)
			}
		}
		if err == nil {
			_, err = tx.Exec("INSERT INTO schema_migrations VALUES(?,?,?)", version+1, digest([]byte(migrations[version])), utc())
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		version++
	}
	return nil
}
