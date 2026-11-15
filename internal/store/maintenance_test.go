package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"software-studio/internal/testutil"
)

func maintenanceStore(t *testing.T) (*Store, string) {
	t.Helper()
	url := testutil.DatabaseURL(t)
	s, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s, url
}
func emptyBrowser() BrowserBackup {
	return BrowserBackup{Drafts: []BrowserDraft{}, Preferences: map[string]string{}}
}
func backupNow(t *testing.T, s *Store) StudioBackup {
	t.Helper()
	state, e := s.StudioStatus(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.CreateStudioBackup(context.Background(), state.Epoch, emptyBrowser())
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func seedMaintenanceTables(t *testing.T, s *Store) {
	t.Helper()
	for _, table := range studioTables {
		values := make([]any, len(table.columns))
		slots := make([]string, len(values))
		for i, c := range table.columns {
			slots[i] = fmt.Sprintf("$%d", i+1)
			switch c {
			case "body", "summary":
				values[i] = `{"seed":9223372036854775807,"text":"café <☃> \\n","nested":{"x":true}}`
			case "created_at", "updated_at", "installed_at":
				values[i] = "2026-11-15T11:25:37.123456Z"
			case "version":
				values[i] = "1.0.0"
			case "id":
				values[i] = table.name + "-row"
			case "execution_id":
				values[i] = "system_executions-row"
			default:
				values[i] = "example"
			}
		}
		if _, e := s.db.Exec("INSERT INTO "+table.name+" ("+strings.Join(table.columns, ",")+") VALUES ("+strings.Join(slots, ",")+")", values...); e != nil {
			t.Fatalf("seed %s: %v", table.name, e)
		}
	}
}
func TestStudioBackupResetRestoreAllTables(t *testing.T) {
	s, _ := maintenanceStore(t)
	ctx := context.Background()
	seedMaintenanceTables(t, s)
	if _, e := s.db.Exec("CREATE TABLE unrelated_data(id TEXT); INSERT INTO unrelated_data VALUES ('keep')"); e != nil {
		t.Fatal(e)
	}
	state, _ := s.StudioStatus(ctx)
	browser := BrowserBackup{Drafts: []BrowserDraft{{Key: "configuration-project-v1", Revision: 4, Updated: "2026-11-15T00:00:00Z", Data: `{"source":"component Counter"}`}}, Preferences: map[string]string{"studio.ribbon.components": "collapsed"}}
	b, e := s.CreateStudioBackup(ctx, state.Epoch, browser)
	if e != nil {
		t.Fatal(e)
	}
	if e = ValidateStudioBackup(b); e != nil {
		t.Fatal(e)
	}
	counts := BackupRowCounts(b)
	if len(counts) != 18 {
		t.Fatal(counts)
	}
	for name, n := range counts {
		if n != 1 {
			t.Fatal(name, n)
		}
	}
	if !strings.Contains(b.Database, "9223372036854775807") {
		t.Fatal("int64 lost")
	}
	reset, e := s.ReplaceStudio(ctx, state.Epoch, b.ID, nil)
	if e != nil {
		t.Fatal(e)
	}
	if reset.Epoch == state.Epoch || reset.Action != "reset" {
		t.Fatal(reset)
	}
	empty := backupNow(t, s)
	for name, n := range BackupRowCounts(empty) {
		if n != 0 {
			t.Fatal(name, n)
		}
	}
	var sentinel string
	if e = s.db.QueryRow("SELECT id FROM unrelated_data").Scan(&sentinel); e != nil || sentinel != "keep" {
		t.Fatal(sentinel, e)
	}
	retained, e := s.StudioBackups(ctx)
	if e != nil || len(retained) != 2 || retained[1].Rows != 18 || retained[1].Drafts != 1 {
		t.Fatal(retained, e)
	}
	// Lost-response retries cannot delete new data after the original reset.
	if _, e = s.db.Exec(`INSERT INTO projects VALUES('after-reset','{}',now())`); e != nil {
		t.Fatal(e)
	}
	retry, e := s.ReplaceStudio(ctx, state.Epoch, b.ID, nil)
	if e != nil || retry != reset {
		t.Fatal(retry, e)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM projects").Scan(&count)
	if count != 1 {
		t.Fatal("retry deleted new data")
	}
	fresh := backupNow(t, s)
	restored, e := s.ReplaceStudio(ctx, reset.Epoch, fresh.ID, &b)
	if e != nil {
		t.Fatal(e)
	}
	if restored.Action != "restore" || restored.RestoreID != b.ID {
		t.Fatal(restored)
	}
	after := backupNow(t, s)
	if after.DatabaseHash != b.DatabaseHash {
		t.Fatalf("restore changed snapshot:\n%s\n%s", b.Database, after.Database)
	}
	saved, e := s.StudioBackup(ctx, b.ID)
	if e != nil || saved.Digest != b.Digest || saved.Browser.Drafts[0].Data != browser.Drafts[0].Data {
		t.Fatal(saved, e)
	}
}
func TestStudioResetRejectsStaleAndFailedRestoreRollsBack(t *testing.T) {
	s, _ := maintenanceStore(t)
	ctx := context.Background()
	seedMaintenanceTables(t, s)
	b := backupNow(t, s)
	if _, e := s.ReplaceStudio(ctx, "old", b.ID, nil); !errors.Is(e, ErrStudioChanged) {
		t.Fatal(e)
	}
	if _, e := s.ReplaceStudio(ctx, b.Epoch, "missing", nil); e == nil {
		t.Fatal("reset without backup")
	}
	if _, e := s.db.Exec(`UPDATE projects SET body='{"changed":true}'`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ReplaceStudio(ctx, b.Epoch, b.ID, nil); !errors.Is(e, ErrStudioChanged) {
		t.Fatal(e)
	}
	fresh := backupNow(t, s)
	bad := b
	bad.ID = randomStudioID()
	var tables databaseSnapshot
	json.Unmarshal([]byte(b.Database), &tables)
	tables["system_execution_replays"] = []json.RawMessage{json.RawMessage(`{"id":"broken","execution_id":"missing","created_at":"2026-11-15T00:00:00Z","body":{}}`)}
	raw, _ := json.Marshal(tables)
	bad.Database = string(raw)
	bad.DatabaseHash = bytesHash(raw)
	bad.Digest = backupDigest(bad)
	if _, e := s.ReplaceStudio(ctx, fresh.Epoch, fresh.ID, &bad); e == nil {
		t.Fatal("invalid FK restore succeeded")
	}
	after := backupNow(t, s)
	if after.DatabaseHash != fresh.DatabaseHash || after.Epoch != fresh.Epoch {
		t.Fatal("failed restore changed studio")
	}
	if _, e := s.StudioBackup(ctx, bad.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("failed target persisted", e)
	}
	bad = b
	bad.Database += " "
	if _, e := s.ReplaceStudio(ctx, fresh.Epoch, fresh.ID, &bad); e == nil {
		t.Fatal("tampered backup accepted")
	}
}
func TestStudioMaintenanceFencesOtherServerInstances(t *testing.T) {
	s, url := maintenanceStore(t)
	other, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	ctx := context.Background()
	b := backupNow(t, s)
	release, _, e := other.StudioRequest(ctx, b.Epoch)
	if e != nil {
		t.Fatal(e)
	}
	// An exclusive reset cannot proceed while another instance serves an API call.
	deadline, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()
	_, e = s.ReplaceStudio(deadline, b.Epoch, b.ID, nil)
	if e == nil {
		t.Fatal("reset bypassed request lease")
	}
	release()
	state, e := s.ReplaceStudio(ctx, b.Epoch, b.ID, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = other.StudioRequest(ctx, b.Epoch); !errors.Is(e, ErrStudioChanged) {
		t.Fatal("stale tab accepted", e)
	}
	release, epoch, e := other.StudioRequest(ctx, state.Epoch)
	if e != nil || epoch != state.Epoch {
		t.Fatal(epoch, e)
	}
	release()
}
func TestStudioBackupValidation(t *testing.T) {
	s, _ := maintenanceStore(t)
	b := backupNow(t, s)
	for _, mutate := range []func(*StudioBackup){
		func(x *StudioBackup) { x.Format = "unknown" }, func(x *StudioBackup) { x.CreatedAt = "bad" },
		func(x *StudioBackup) { x.Browser.Preferences = map[string]string{"unrelated": "no"} },
		func(x *StudioBackup) { x.Browser.Drafts = []BrowserDraft{{Key: "draft", Data: "broken"}} },
		func(x *StudioBackup) {
			var tables databaseSnapshot
			json.Unmarshal([]byte(x.Database), &tables)
			delete(tables, "projects")
			raw, _ := json.Marshal(tables)
			x.Database = string(raw)
			x.DatabaseHash = bytesHash(raw)
		},
		func(x *StudioBackup) {
			var tables databaseSnapshot
			json.Unmarshal([]byte(x.Database), &tables)
			tables["projects"] = []json.RawMessage{json.RawMessage(`{"id":"x","body":{},"updated_at":"2026-11-15T00:00:00Z","extra":true}`)}
			raw, _ := json.Marshal(tables)
			x.Database = string(raw)
			x.DatabaseHash = bytesHash(raw)
		},
	} {
		bad := b
		mutate(&bad)
		bad.Digest = backupDigest(bad)
		if ValidateStudioBackup(bad) == nil {
			t.Fatal("invalid backup accepted", bad)
		}
	}
}
