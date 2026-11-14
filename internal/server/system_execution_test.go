package server

import (
	"encoding/json"
	"software-studio/internal/model"
	"software-studio/internal/store"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"testing"
)

func TestPureSystemExecutionAPI(t *testing.T) {
	s := testServer(t)
	w := req(t, s, "POST", "/api/systems/connection-example", nil)
	var example system.ConnectionExamplePack
	if err := json.Unmarshal(w.Body.Bytes(), &example); err != nil || w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = req(t, s, "POST", "/api/systems/execution/prepare", map[string]any{"design": example.Design})
	var p store.ExecutionPreparation
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || w.Code != 200 || len(p.Plan.Inputs) != 1 {
		t.Fatal(w.Body.String())
	}
	port := p.Plan.Inputs[0]
	encounter, patient := "synthetic-encounter", "synthetic-patient"
	inputs := []systemrun.Inputs{{TaskID: port.TaskID, Values: map[string]model.TypedValue{port.Name: {Type: "record", RecordRef: port.RecordRef, Record: map[string]model.TypedValue{"encounterId": {Type: "string", String: &encounter}, "patientId": {Type: "string", String: &patient}}}}}}
	w = req(t, s, "POST", "/api/systems/execution/run", map[string]any{"design": example.Design, "inputs": inputs})
	var e store.SystemExecution
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || w.Code != 200 || len(e.Report.Events) != 5 || e.Report.ClaimsEvaluated {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = req(t, s, "GET", "/api/systems/executions/"+e.ID, nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = req(t, s, "GET", "/api/systems/executions?systemId="+example.Design.ID, nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	replayPath := "/api/systems/executions/" + e.ID + "/replays"
	w = req(t, s, "POST", replayPath, map[string]any{})
	var replay store.SystemReplay
	if err := json.Unmarshal(w.Body.Bytes(), &replay); err != nil || w.Code != 200 || replay.Status != "matched" || replay.ExecutionID != e.ID {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	if w = req(t, s, "GET", "/api/systems/execution-replays/"+replay.ID, nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = req(t, s, "GET", replayPath, nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = req(t, s, "POST", replayPath, map[string]any{"inputs": inputs}); w.Code != 400 {
		t.Fatal("replay input override accepted", w.Code)
	}
	if w = req(t, s, "POST", replayPath, json.RawMessage("null")); w.Code != 400 {
		t.Fatal("null replay request accepted", w.Code)
	}
	if w = req(t, s, "DELETE", replayPath, nil); w.Code != 405 {
		t.Fatal(w.Code)
	}
	if w = req(t, s, "POST", "/api/systems/executions/missing/replays", map[string]any{}); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = req(t, s, "GET", "/api/systems/executions/missing/replays", nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w = req(t, s, "GET", "/api/systems/execution-replays/missing", nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
	for i := 0; i < cap(s.runs); i++ {
		s.runs <- struct{}{}
	}
	w = req(t, s, "POST", replayPath, map[string]any{})
	for i := 0; i < cap(s.runs); i++ {
		<-s.runs
	}
	if w.Code != 429 {
		t.Fatal("replay ignored execution capacity", w.Code)
	}
	if w = req(t, s, "POST", "/api/systems/execution/run", map[string]any{"design": example.Design}); w.Code != 422 {
		t.Fatal("missing inputs", w.Code)
	}
	if w = req(t, s, "POST", "/api/systems/execution/prepare", map[string]any{"design": system.ReceptionExample()}); w.Code != 422 {
		t.Fatal("unsupported deployment", w.Code)
	}
	if w = req(t, s, "POST", "/api/systems/execution/run", map[string]any{"design": example.Design, "inputs": inputs, "databaseUrl": "postgres://untrusted"}); w.Code != 400 {
		t.Fatal("unexpected DSN accepted", w.Code)
	}
	if w = req(t, New(nil, ""), "POST", "/api/systems/execution/run", map[string]any{"design": example.Design, "inputs": inputs}); w.Code != 503 {
		t.Fatal("unavailable storage", w.Code)
	}
}
