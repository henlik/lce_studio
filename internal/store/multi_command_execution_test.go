package store

import (
	"context"
	"encoding/json"
	"reflect"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
	"testing"
)

func TestMultiCommandSystemPersistenceAndReplay(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e, err := s.InstallMultiCommandExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.InstallMultiCommandExample(ctx); err != nil {
		t.Fatal("reinstall", err)
	}
	if _, err = s.InstallStateExecutionExample(ctx); err != nil {
		t.Fatal("old example no longer installable", err)
	}
	p, err := s.PrepareSystemExecution(ctx, e.Design)
	if err != nil {
		t.Fatal(err)
	}
	tenant, resource, q := "tenantDefault", "resourceDefault", int64(2)
	in := []systemrun.Inputs{{TaskID: p.Plan.Inputs[0].TaskID, Values: map[string]model.TypedValue{"request": {Type: "record", RecordRef: p.Plan.Inputs[0].RecordRef, Record: map[string]model.TypedValue{"tenantId": {Type: "string", String: &tenant}, "resourceId": {Type: "string", String: &resource}, "quantity": {Type: "integer", Integer: &q}}}}}}
	run, err := s.ExecuteSystem(ctx, "multi-graph", e.Design, in)
	if err != nil || run.Report.EngineVersion != systemrun.MultiCommandVersion || *run.Report.Results[2].Outputs["remaining"].Integer != 3 {
		t.Fatal(run.Report, err)
	}
	owner := systemrun.Key("stock", "stock")
	zero, one := int64(0), int64(1)
	eight := int64(8)
	scenario := systemrun.Scenario{Name: "Shared command owner", Steps: []systemrun.ScenarioStep{
		{Kind: "invoke", Inputs: in, Calls: []systemrun.StateCall{{TaskID: owner, CommandID: "reserveA", ExpectedRevision: &zero}}},
		{Kind: "command", TaskID: owner, Inputs: []systemrun.Inputs{{TaskID: owner, Values: map[string]model.TypedValue{"count": {Type: "integer", Integer: &eight}}}}, Calls: []systemrun.StateCall{{TaskID: owner, Command: "adjust", CommandID: "adjustA", ExpectedRevision: &one}}, ExpectedStatuses: map[string]string{owner: "committed"}},
		{Kind: "command", TaskID: owner, Inputs: []systemrun.Inputs{{TaskID: owner, Values: in[0].Values}}, Calls: []systemrun.StateCall{{TaskID: owner, Command: "reserve", CommandID: "reserveA", ExpectedRevision: &zero}}, ExpectedStatuses: map[string]string{owner: "replayed"}},
	}}
	shared, err := s.ExecuteSystemScenario(ctx, "multi-scenario", e.Design, scenario)
	if err != nil || !shared.Report.Scenario.Passed || shared.Report.EngineVersion != "system-scenario-2" {
		t.Fatal(shared.Report, err)
	}
	final := shared.Report.Scenario.Final[0].Instance
	if *final.State["available"].Integer != 8 || final.Revision != 2 || len(final.Receipts) != 2 {
		t.Fatal(final)
	}
	saved, err := s.SystemExecution(ctx, shared.ID)
	if err != nil || !reflect.DeepEqual(saved, shared) {
		t.Fatal("snapshot roundtrip", err)
	}
	copyDesign := func() system.SystemDesign {
		b, _ := json.Marshal(e.Design)
		var d system.SystemDesign
		_ = json.Unmarshal(b, &d)
		return d
	}
	for _, entries := range [][]system.StateEntry{nil, {{Slot: "stock", Command: "missing"}}, {{Slot: "unknown", Command: "reserve"}}, {{Slot: "stock", Command: "adjust"}}, {{Slot: "stock", Command: "reserve"}, {Slot: "stock", Command: "adjust"}}} {
		d := copyDesign()
		d.Components[1].StateEntries = entries
		if _, err = s.PrepareSystemExecution(ctx, d); err == nil {
			t.Fatal("bad entry or mismatched link accepted", entries)
		}
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM state_machines; DELETE FROM component_packages`); err != nil {
		t.Fatal(err)
	}
	for _, execution := range []SystemExecution{run, shared} {
		proof, err := s.ReplaySystemExecution(ctx, "verify-"+execution.ID, execution.ID)
		if err != nil || proof.Status != "matched" || proof.Report == nil || !reflect.DeepEqual(*proof.Report, execution.Report) {
			t.Fatal("catalog-independent replay", proof, err)
		}
	}
}
