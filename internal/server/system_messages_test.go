package server

import (
	"encoding/json"
	"software-studio/internal/store"
	"software-studio/internal/system"
	"testing"
)

func TestSystemMessageAPIBoundaries(t *testing.T) {
	s := testServer(t)
	w := req(t, s, "POST", "/api/systems/message-example", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var e system.MessageExamplePack
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if w = req(t, s, "POST", "/api/systems/message-example", nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = req(t, s, "POST", "/api/systems/execution/prepare", map[string]any{"design": e.Design})
	var p store.ExecutionPreparation
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || w.Code != 200 || len(p.Plan.Channels) != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	channel := p.Plan.Channels[0].ID
	scenario := map[string]any{"name": "Missing intent", "steps": []any{map[string]any{"kind": "dispatch", "message": map[string]any{"channelId": channel, "routeId": "reservationToLedger", "sourceCommandId": "neverCommitted", "alias": "absent", "expectedRevision": 0}, "expectedMessageStatus": "missing_intent"}}, "expected": []any{}, "expectedMessages": []any{map[string]any{"channelId": channel, "name": "absent", "status": "absent", "attempts": 0}}}
	w = req(t, s, "POST", "/api/systems/execution/run", map[string]any{"design": e.Design, "scenario": scenario})
	var run store.SystemExecution
	if err := json.Unmarshal(w.Body.Bytes(), &run); err != nil || w.Code != 200 || !run.Report.Scenario.Passed {
		t.Fatal(w.Code, w.Body.String())
	}
	w = req(t, s, "POST", "/api/systems/executions/"+run.ID+"/replays", map[string]any{})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	assertion := scenario["expectedMessages"].([]any)[0].(map[string]any)
	for _, bad := range []map[string]any{{"channelId": channel, "name": "absent", "status": "absent"}, {"channelId": channel, "name": "absent", "status": "absent", "attempts": nil}, {"channelId": channel, "name": "absent", "status": "absent", "attempts": 0, "extra": true}} {
		scenario["expectedMessages"] = []any{bad}
		w = req(t, s, "POST", "/api/systems/execution/run", map[string]any{"design": e.Design, "scenario": scenario})
		if w.Code != 400 {
			t.Fatal("malformed assertion accepted", w.Code, w.Body.String())
		}
	}
	scenario["expectedMessages"] = []any{assertion}
	scenario["steps"].([]any)[0].(map[string]any)["message"].(map[string]any)["channelId"] = "missing"
	if w = req(t, s, "POST", "/api/systems/execution/run", map[string]any{"design": e.Design, "scenario": scenario}); w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = req(t, s, "GET", "/api/systems/message-example", nil); w.Code != 405 {
		t.Fatal(w.Code)
	}
}
