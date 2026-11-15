package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/store"
)

func TestStudioMaintenanceHTTP(t *testing.T) {
	s := testServer(t)
	getStatus := func() store.StudioStatus {
		t.Helper()
		w := req(t, s, "GET", "/api/studio/status", nil)
		var state store.StudioStatus
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &state) != nil {
			t.Fatal(w.Code, w.Body)
		}
		return state
	}
	state := getStatus()
	if w := req(t, s, "POST", "/api/projects", model.ExampleProject()); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w := req(t, s, "POST", "/api/studio/backup", map[string]any{"epoch": state.Epoch, "browser": map[string]any{"drafts": []any{}, "preferences": map[string]string{}}})
	var b store.StudioBackup
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &b) != nil {
		t.Fatal(w.Code, w.Body)
	}
	payload := map[string]any{"epoch": state.Epoch, "backupId": b.ID}
	if w = req(t, s, "POST", "/api/studio/reset", payload); w.Code != 422 {
		t.Fatal(w.Code, w.Body)
	}
	if getStatus() != state {
		t.Fatal("missing confirmation mutated state")
	}
	payload["confirmation"] = "RESET"
	raw, _ := json.Marshal(payload)
	r := httptest.NewRequest("POST", "http://example.com/api/studio/reset", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://attacker.invalid")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross origin reset", w.Code)
	}
	if w = req(t, s, "POST", "/api/studio/reset", payload); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w = req(t, s, "GET", "/api/projects", nil); w.Code != 200 || w.Body.String() != "[]\n" {
		t.Fatal(w.Code, w.Body)
	}
	p, _ := json.Marshal(model.ExampleProject())
	r = httptest.NewRequest("POST", "/api/projects", bytes.NewReader(p))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Studio-Epoch", state.Epoch)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal("old tab repopulated reset", w.Code, w.Body)
	}
	if w = req(t, s, "GET", "/api/studio/backups", nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if w = req(t, s, "POST", "/api/studio/validate-backup", map[string]any{"backup": b}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	state = getStatus()
	w = req(t, s, "POST", "/api/studio/backup", map[string]any{"epoch": state.Epoch, "browser": map[string]any{"drafts": []any{}, "preferences": map[string]string{}}})
	var safe store.StudioBackup
	json.Unmarshal(w.Body.Bytes(), &safe)
	if w = req(t, s, "POST", "/api/studio/restore", map[string]any{"epoch": state.Epoch, "backupId": safe.ID, "backup": b, "confirmation": "RESTORE"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if getStatus().RestoreID != b.ID {
		t.Fatal("not restored")
	}
	if w = req(t, s, "GET", "/api/projects", nil); w.Code != 200 || w.Body.String() == "[]\n" {
		t.Fatal(w.Code, w.Body)
	}
}
