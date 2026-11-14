package server

import (
	"encoding/json"
	"software-studio/internal/system"
	"testing"
)

func TestSystemConnectionExampleAPI(t *testing.T) {
	s := testServer(t)
	w := req(t, s, "POST", "/api/systems/connection-example", nil)
	var fixture system.ConnectionExamplePack
	if err := json.Unmarshal(w.Body.Bytes(), &fixture); err != nil || w.Code != 200 {
		t.Fatalf("example %d %s %v", w.Code, w.Body.String(), err)
	}
	for _, scenario := range []struct {
		d      system.SystemDesign
		valid  bool
		status string
	}{{fixture.Design, true, "checked"}, {fixture.Mismatch, false, "invalid"}, {system.ReceptionExample(), true, "incomplete"}} {
		w = req(t, s, "POST", "/api/systems/resolve", scenario.d)
		var r system.Resolution
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil || w.Code != 200 || r.Valid != scenario.valid || r.Compatibility != scenario.status {
			t.Fatalf("resolve %s %v", w.Body.String(), err)
		}
	}
	if w = req(t, s, "POST", "/api/systems/conformance", map[string]any{"design": fixture.Design}); w.Code != 422 {
		t.Fatal("static fixture ran as clinical implementation", w.Code)
	}
	fixture.Design.Contract.Claims = nil
	fixture.Design.Connections[0].TargetPort = nil
	if w = req(t, s, "POST", "/api/systems/interfaces", fixture.Design); w.Code != 200 {
		t.Fatal(w.Code)
	}
	var inspection struct {
		Ports []system.InstancePort `json:"ports"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &inspection); err != nil || len(inspection.Ports) != 4 {
		t.Fatal("interfaces unavailable for draft", w.Body.String())
	}
}
