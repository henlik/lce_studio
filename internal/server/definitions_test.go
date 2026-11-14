package server

import (
	"context"
	"encoding/json"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"testing"
)

func TestDefinitionsAPI(t *testing.T) {
	s := testServer(t)
	ctx := context.Background()
	p := model.ExampleDataFixtures()[0].Contract.Contract
	if _, _, err := s.store.InstallDataContract(ctx, p); err != nil {
		t.Fatal(err)
	}
	catalog, err := s.store.SystemCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var aspect system.AspectRef
	for _, e := range catalog {
		if e.Kind == "data" {
			aspect = system.AspectRef{Slot: "patient", Kind: e.Kind, ID: e.ID, Version: e.Version, Digest: e.Digest}
		}
	}
	d := system.ComponentDefinition{SchemaVersion: 1, ID: "test.api-reception", Name: "Reception", Version: "1.0.0", Aspects: []system.AspectRef{aspect}}
	for _, path := range []string{"/api/systems/definitions/validate", "/api/systems/definitions", "/api/systems/definitions"} {
		w := req(t, s, "POST", path, d)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	w := req(t, s, "POST", "/api/systems/definitions/aspect", aspect)
	var editor struct {
		Workspace string `json:"workspace"`
		Body      struct {
			Contract model.DataContractLock `json:"contract"`
		} `json:"body"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &editor); err != nil || w.Code != 200 || editor.Workspace != "data" || editor.Body.Contract.Digest != aspect.Digest || editor.Body.Contract.Contract.ID != p.ID {
		t.Fatalf("exact editor: %d %s %v", w.Code, w.Body.String(), err)
	}
	d.Name = "Conflict"
	if w := req(t, s, "POST", "/api/systems/definitions", d); w.Code != 409 {
		t.Fatalf("conflict: %d", w.Code)
	}
	d.Aspects[0].Version = "99.0.0"
	if w := req(t, s, "POST", "/api/systems/definitions", d); w.Code != 422 {
		t.Fatalf("missing pin: %d", w.Code)
	}
	if w := req(t, s, "GET", "/api/systems/definitions", nil); w.Code != 200 {
		t.Fatalf("list: %d", w.Code)
	}
	aspect.Digest = "bad"
	if w := req(t, s, "POST", "/api/systems/definitions/aspect", aspect); w.Code != 422 {
		t.Fatalf("bad pin: %d", w.Code)
	}
	if w := req(t, New(nil, ""), "GET", "/api/systems/definitions", nil); w.Code != 503 {
		t.Fatalf("no store: %d", w.Code)
	}
}
