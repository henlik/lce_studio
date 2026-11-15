package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const StudioBackupFormat = "lce-studio-backup/1"
const MaxStudioBackupBytes = 128 << 20

var ErrStudioChanged = errors.New("studio changed; create a fresh backup or reload before continuing")

// Only these application tables are in reset scope. Backup history and control
// metadata remain. Names/columns never come from an uploaded file.
type backupTable struct {
	name    string
	columns []string
	order   string
}

var studioTables = []backupTable{
	{"configuration_packages", []string{"id", "version", "digest", "body", "installed_at"}, "id,version"},
	{"projects", []string{"id", "body", "updated_at"}, "id"},
	{"runs", []string{"id", "project_id", "created_at", "body", "summary"}, "id"},
	{"component_packages", []string{"id", "version", "digest", "body", "installed_at"}, "id,version"},
	{"component_evaluations", []string{"id", "created_at", "body", "summary"}, "id"},
	{"data_contracts", []string{"id", "version", "digest", "body", "installed_at"}, "id,version"},
	{"data_validations", []string{"id", "created_at", "body", "summary"}, "id"},
	{"state_machines", []string{"id", "version", "digest", "body", "installed_at"}, "id,version"},
	{"state_runs", []string{"id", "created_at", "body", "summary"}, "id"},
	{"delivery_models", []string{"id", "version", "digest", "body", "installed_at"}, "id,version"},
	{"delivery_runs", []string{"id", "created_at", "body", "summary"}, "id"},
	{"workflow_packages", []string{"id", "version", "digest", "body", "installed_at"}, "id,version"},
	{"workflow_runs", []string{"id", "created_at", "body", "summary"}, "id"},
	{"system_designs", []string{"id", "body", "updated_at"}, "id"},
	{"component_definitions", []string{"id", "version", "digest", "body"}, "id,version"},
	{"system_executions", []string{"id", "system_id", "created_at", "body"}, "id"},
	{"system_execution_replays", []string{"id", "execution_id", "created_at", "body"}, "id"},
	{"system_evidence", []string{"id", "system_id", "created_at", "body"}, "id"},
}

type BrowserDraft struct {
	Key      string `json:"key"`
	Revision int64  `json:"revision"`
	Updated  string `json:"updated"`
	Data     string `json:"data"`
}
type BrowserBackup struct {
	Drafts      []BrowserDraft    `json:"drafts"`
	Preferences map[string]string `json:"preferences"`
}
type StudioBackup struct {
	Format    string `json:"format"`
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Epoch     string `json:"epoch"`
	// A string preserves JSON numbers (e.g. int64 seeds) through browser downloads.
	Database     string        `json:"database"`
	DatabaseHash string        `json:"databaseHash"`
	Browser      BrowserBackup `json:"browser"`
	Digest       string        `json:"digest"`
}
type StudioStatus struct {
	Epoch     string `json:"epoch"`
	Action    string `json:"action"`
	BackupID  string `json:"backupId"`
	RestoreID string `json:"restoreId"`
}
type BackupSummary struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Rows      int    `json:"rows"`
	Drafts    int    `json:"drafts"`
	Digest    string `json:"digest"`
}
type databaseSnapshot map[string][]json.RawMessage

func randomStudioID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func bytesHash(b []byte) string          { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func backupDigest(b StudioBackup) string { b.Digest = ""; v, _ := json.Marshal(b); return bytesHash(v) }
func initializeMaintenance(ctx context.Context, tx *sql.Tx) error {
	_, e := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS studio_control (id INTEGER PRIMARY KEY CHECK(id=1),epoch TEXT NOT NULL,action TEXT NOT NULL DEFAULT '',backup_id TEXT NOT NULL DEFAULT '',restore_id TEXT NOT NULL DEFAULT ''); CREATE TABLE IF NOT EXISTS studio_backups (id TEXT PRIMARY KEY,created_at TIMESTAMPTZ NOT NULL,body JSON NOT NULL);`)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO studio_control(id,epoch) VALUES(1,$1) ON CONFLICT(id) DO NOTHING`, randomStudioID())
	return e
}

const studioLeaseKey = `hashtextextended(current_database() || '.' || current_schema() || '.studio-maintenance',0)`

// Separate connections prevent request leases from consuming the query pool.
// All application instances sharing a schema use the same PostgreSQL lock.
func (s *Store) StudioRequest(ctx context.Context, epoch string) (func(), string, error) {
	tx, e := s.leases.BeginTx(ctx, nil)
	if e != nil {
		return nil, "", e
	}
	release := func() { _ = tx.Rollback() }
	if _, e = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock_shared("+studioLeaseKey+")"); e != nil {
		release()
		return nil, "", e
	}
	var current string
	e = tx.QueryRowContext(ctx, "SELECT epoch FROM studio_control WHERE id=1").Scan(&current)
	if e == nil && epoch != "" && epoch != current {
		e = ErrStudioChanged
	}
	if e != nil {
		release()
		return nil, current, e
	}
	return release, current, nil
}
func (s *Store) StudioStatus(ctx context.Context) (StudioStatus, error) {
	var v StudioStatus
	e := s.db.QueryRowContext(ctx, "SELECT epoch,action,backup_id,restore_id FROM studio_control WHERE id=1").Scan(&v.Epoch, &v.Action, &v.BackupID, &v.RestoreID)
	return v, e
}
func (s *Store) maintenanceTx(ctx context.Context) (*sql.Tx, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock("+studioLeaseKey+")"); e == nil {
		_, e = tx.ExecContext(ctx, "SET LOCAL TIME ZONE 'UTC'")
	}
	if e == nil {
		names := []string{}
		for _, t := range studioTables {
			names = append(names, t.name)
		}
		_, e = tx.ExecContext(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN SHARE ROW EXCLUSIVE MODE")
	}
	if e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func snapshotDatabase(ctx context.Context, tx *sql.Tx) (string, int, error) {
	data := databaseSnapshot{}
	total, rowsCount := 0, 0
	for _, t := range studioTables {
		rows, e := tx.QueryContext(ctx, "SELECT row_to_json(t) FROM (SELECT "+strings.Join(t.columns, ",")+" FROM "+t.name+" ORDER BY "+t.order+") t")
		if e != nil {
			return "", 0, e
		}
		list := []json.RawMessage{}
		for rows.Next() {
			var b []byte
			if e = rows.Scan(&b); e != nil {
				rows.Close()
				return "", 0, e
			}
			total += len(b)
			rowsCount++
			if total > MaxStudioBackupBytes || rowsCount > 200000 {
				rows.Close()
				return "", 0, fmt.Errorf("studio exceeds backup limit (128 MiB / 200000 rows); no data was cleared")
			}
			list = append(list, json.RawMessage(b))
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", 0, e
		}
		data[t.name] = list
	}
	b, e := json.Marshal(data)
	return string(b), rowsCount, e
}
func validateBrowser(b BrowserBackup) error {
	if b.Drafts == nil || b.Preferences == nil {
		return fmt.Errorf("browser backup must include drafts and preferences")
	}
	seen := map[string]bool{}
	for _, d := range b.Drafts {
		if d.Key == "" || len(d.Key) > 512 || d.Revision < 0 || seen[d.Key] || !json.Valid([]byte(d.Data)) {
			return fmt.Errorf("invalid browser draft")
		}
		seen[d.Key] = true
	}
	for k := range b.Preferences {
		if !strings.HasPrefix(k, "studio.ribbon.") {
			return fmt.Errorf("unsupported browser preference")
		}
	}
	return nil
}
func decodeSnapshot(raw string) (databaseSnapshot, error) {
	var tables databaseSnapshot
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&tables); e != nil {
		return nil, e
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("invalid database snapshot")
	}
	if len(tables) != len(studioTables) {
		return nil, fmt.Errorf("backup table set is incomplete or unsupported")
	}
	count := 0
	for _, t := range studioTables {
		rows, ok := tables[t.name]
		if !ok || rows == nil {
			return nil, fmt.Errorf("missing table %s", t.name)
		}
		count += len(rows)
		if count > 200000 {
			return nil, fmt.Errorf("too many backup rows")
		}
		for _, row := range rows {
			var fields map[string]json.RawMessage
			if json.Unmarshal(row, &fields) != nil || len(fields) != len(t.columns) {
				return nil, fmt.Errorf("invalid columns for %s", t.name)
			}
			for _, name := range t.columns {
				v, ok := fields[name]
				if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
					return nil, fmt.Errorf("missing %s.%s", t.name, name)
				}
			}
		}
	}
	return tables, nil
}
func ValidateStudioBackup(b StudioBackup) error {
	raw, e := json.Marshal(b)
	if e != nil || len(raw) > MaxStudioBackupBytes {
		return fmt.Errorf("backup exceeds 128 MiB")
	}
	if b.Format != StudioBackupFormat || len(b.ID) != 32 || b.Digest != backupDigest(b) || b.DatabaseHash != bytesHash([]byte(b.Database)) {
		return fmt.Errorf("unsupported or damaged backup; checksum mismatch")
	}
	if _, e = time.Parse(time.RFC3339Nano, b.CreatedAt); e != nil {
		return fmt.Errorf("invalid backup date")
	}
	if e = validateBrowser(b.Browser); e != nil {
		return e
	}
	_, e = decodeSnapshot(b.Database)
	return e
}
func saveStudioBackup(ctx context.Context, tx *sql.Tx, b StudioBackup) error {
	raw, e := json.Marshal(b)
	if e != nil || len(raw) > MaxStudioBackupBytes {
		return fmt.Errorf("backup exceeds 128 MiB; no data was cleared")
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO studio_backups(id,created_at,body) VALUES($1,$2,$3) ON CONFLICT(id) DO NOTHING", b.ID, b.CreatedAt, string(raw))
	if e != nil {
		return e
	}
	var stored StudioBackup
	var existing []byte
	if e = tx.QueryRowContext(ctx, "SELECT body FROM studio_backups WHERE id=$1", b.ID).Scan(&existing); e != nil {
		return e
	}
	if json.Unmarshal(existing, &stored) != nil || stored.Digest != b.Digest {
		return fmt.Errorf("backup identity conflict")
	}
	return nil
}
func (s *Store) CreateStudioBackup(ctx context.Context, epoch string, browser BrowserBackup) (StudioBackup, error) {
	if e := validateBrowser(browser); e != nil {
		return StudioBackup{}, e
	}
	tx, e := s.maintenanceTx(ctx)
	if e != nil {
		return StudioBackup{}, e
	}
	defer tx.Rollback()
	var current string
	if e = tx.QueryRowContext(ctx, "SELECT epoch FROM studio_control WHERE id=1").Scan(&current); e != nil {
		return StudioBackup{}, e
	}
	if epoch != current {
		return StudioBackup{}, ErrStudioChanged
	}
	data, _, e := snapshotDatabase(ctx, tx)
	if e != nil {
		return StudioBackup{}, e
	}
	b := StudioBackup{Format: StudioBackupFormat, ID: randomStudioID(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Epoch: current, Database: data, DatabaseHash: bytesHash([]byte(data)), Browser: browser}
	b.Digest = backupDigest(b)
	if e = saveStudioBackup(ctx, tx, b); e != nil {
		return StudioBackup{}, e
	}
	return b, tx.Commit()
}
func (s *Store) StudioBackup(ctx context.Context, id string) (StudioBackup, error) {
	var b StudioBackup
	var raw []byte
	e := s.db.QueryRowContext(ctx, "SELECT body FROM studio_backups WHERE id=$1", id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	if e != nil {
		return b, e
	}
	e = json.Unmarshal(raw, &b)
	return b, e
}
func (s *Store) StudioBackups(ctx context.Context) ([]BackupSummary, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,body->>'createdAt',body->>'digest',json_array_length(body->'browser'->'drafts'),COALESCE((SELECT SUM(json_array_length(value)) FROM json_each((body->>'database')::json)),0) FROM studio_backups ORDER BY created_at DESC,id DESC LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BackupSummary{}
	for rows.Next() {
		var b BackupSummary
		if e = rows.Scan(&b.ID, &b.CreatedAt, &b.Digest, &b.Drafts, &b.Rows); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Reset or replacement is committed only against the exact archived current
// database. Foreign-key failures, upload errors and concurrent edits roll back.
func (s *Store) ReplaceStudio(ctx context.Context, epoch, backupID string, target *StudioBackup) (StudioStatus, error) {
	var tables databaseSnapshot
	var e error
	if target != nil {
		if e = ValidateStudioBackup(*target); e != nil {
			return StudioStatus{}, e
		}
		tables, e = decodeSnapshot(target.Database)
		if e != nil {
			return StudioStatus{}, e
		}
	}
	tx, e := s.maintenanceTx(ctx)
	if e != nil {
		return StudioStatus{}, e
	}
	defer tx.Rollback()
	var state StudioStatus
	if e = tx.QueryRowContext(ctx, "SELECT epoch,action,backup_id,restore_id FROM studio_control WHERE id=1").Scan(&state.Epoch, &state.Action, &state.BackupID, &state.RestoreID); e != nil {
		return state, e
	}
	// Retrying after a lost response must finish the same operation, never delete
	// data a second time. The archive ID is unique and immutable.
	if state.BackupID == backupID && ((target == nil && state.Action == "reset") || (target != nil && state.Action == "restore" && state.RestoreID == target.ID)) {
		return state, nil
	}
	if epoch != state.Epoch {
		return state, ErrStudioChanged
	}
	var raw []byte
	var safety StudioBackup
	if e = tx.QueryRowContext(ctx, "SELECT body FROM studio_backups WHERE id=$1", backupID).Scan(&raw); e != nil {
		return state, fmt.Errorf("a retained backup is required")
	}
	if json.Unmarshal(raw, &safety) != nil || ValidateStudioBackup(safety) != nil || safety.Epoch != epoch {
		return state, fmt.Errorf("invalid safety backup")
	}
	current, _, e := snapshotDatabase(ctx, tx)
	if e != nil {
		return state, e
	}
	if bytesHash([]byte(current)) != safety.DatabaseHash {
		return state, ErrStudioChanged
	}
	if target != nil {
		if e = saveStudioBackup(ctx, tx, *target); e != nil {
			return state, e
		}
	}
	// The only application FK is replays -> executions. Delete in reverse order.
	for i := len(studioTables) - 1; i >= 0; i-- {
		if _, e = tx.ExecContext(ctx, "DELETE FROM "+studioTables[i].name); e != nil {
			return state, e
		}
	}
	if target != nil {
		for _, t := range studioTables {
			b, _ := json.Marshal(tables[t.name])
			if _, e = tx.ExecContext(ctx, "INSERT INTO "+t.name+" ("+strings.Join(t.columns, ",")+") SELECT "+strings.Join(t.columns, ",")+" FROM json_populate_recordset(NULL::"+t.name+",$1::json)", string(b)); e != nil {
				return state, fmt.Errorf("restore %s: %w", t.name, e)
			}
		}
	}
	state = StudioStatus{Epoch: randomStudioID(), Action: "reset", BackupID: safety.ID}
	if target != nil {
		state.Action = "restore"
		state.RestoreID = target.ID
	}
	_, e = tx.ExecContext(ctx, "UPDATE studio_control SET epoch=$1,action=$2,backup_id=$3,restore_id=$4 WHERE id=1", state.Epoch, state.Action, state.BackupID, state.RestoreID)
	if e != nil {
		return state, e
	}
	return state, tx.Commit()
}

// A stable summary is useful for acceptance checks and backup review.
func BackupRowCounts(b StudioBackup) map[string]int {
	tables, _ := decodeSnapshot(b.Database)
	names := make([]string, 0, len(tables))
	for k := range tables {
		names = append(names, k)
	}
	sort.Strings(names)
	out := map[string]int{}
	for _, k := range names {
		out[k] = len(tables[k])
	}
	return out
}
