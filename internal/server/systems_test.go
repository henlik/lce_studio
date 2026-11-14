package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/store"
	"software-studio/internal/system"
)

func TestSystemValidationAndStorageUnavailable(t *testing.T) {
	s := New(nil, "")
	design := system.ReceptionExample()
	if w := req(t, s, "GET", "/api/systems/example", nil); w.Code != 200 {
		t.Fatalf("example: %d %s", w.Code, w.Body.String())
	}
	w := req(t, s, "POST", "/api/systems/validate", design)
	var validation model.Validation
	if err := json.Unmarshal(w.Body.Bytes(), &validation); err != nil || w.Code != 200 || !validation.Valid {
		t.Fatalf("validate: %d %s %v", w.Code, w.Body.String(), err)
	}
	design.Contract.Claims[0].Kind = "execute_shell"
	w = req(t, s, "POST", "/api/systems/validate", design)
	if err := json.Unmarshal(w.Body.Bytes(), &validation); err != nil || validation.Valid {
		t.Fatalf("unknown claim accepted: %s", w.Body.String())
	}
	if w := req(t, s, "POST", "/api/systems/conformance", map[string]any{"design": system.ReceptionExample()}); w.Code != 503 {
		t.Fatalf("missing storage simulated success: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "POST", "/api/systems/conformance", map[string]any{"design": system.ReceptionExample(), "databaseUrl": "postgres://untrusted"}); w.Code != 400 {
		t.Fatalf("client DSN accepted: %d", w.Code)
	}
}

func TestSystemsAPIAndConformanceEvidence(t *testing.T) {
	s := testServer(t)
	design := system.ReceptionExample()
	path := "/api/systems/" + design.ID
	if w := req(t, s, "PUT", path, design); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", path, nil); w.Code != 200 {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	wrong := design
	wrong.ID = "other"
	if w := req(t, s, "PUT", path, wrong); w.Code != 400 {
		t.Fatalf("path mismatch: %d", w.Code)
	}
	w := req(t, s, "POST", "/api/systems/conformance", map[string]any{"design": design})
	if w.Code != 200 {
		t.Fatalf("conformance: %d %s", w.Code, w.Body.String())
	}
	var evidence store.SystemEvidence
	if err := json.Unmarshal(w.Body.Bytes(), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.ID == "" || evidence.DesignHash == "" || evidence.Design.ID != design.ID {
		t.Fatalf("missing provenance: %#v", evidence)
	}
	if w := req(t, s, "GET", "/api/systems/evidence/"+evidence.ID, nil); w.Code != 200 {
		t.Fatalf("evidence: %d %s", w.Code, w.Body.String())
	}
	if w := req(t, s, "GET", "/api/systems/evidence?systemId="+design.ID, nil); w.Code != 200 {
		t.Fatalf("history: %d %s", w.Code, w.Body.String())
	}

	invalid := system.ReceptionExample()
	invalid.Components[1].Definition.Version = "2.0.0"
	if w := req(t, s, "POST", "/api/systems/conformance", map[string]any{"design": invalid}); w.Code != 422 {
		t.Fatalf("unsupported design ran: %d %s", w.Code, w.Body.String())
	}
	invalid.Contract.Claims[0].Kind = "unknown"
	if w := req(t, s, "PUT", path, invalid); w.Code != 422 {
		t.Fatalf("invalid contract saved: %d %s", w.Code, w.Body.String())
	}
	if err := s.store.Close(); err != nil {
		t.Fatal(err)
	}
	if w := req(t, s, "POST", "/api/systems/conformance", map[string]any{"design": design}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed database simulated success: %d %s", w.Code, w.Body.String())
	}
}
