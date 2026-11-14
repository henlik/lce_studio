package store

import (
	"context"
	"reflect"
	"software-studio/internal/model"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
	"testing"
)

func TestConnectedStateProviderPersistenceAndPortableReplay(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e, err := s.InstallStateExecutionExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.InstallStateExecutionExample(ctx); err != nil {
		t.Fatal("reinstall", err)
	}
	p, err := s.PrepareSystemExecution(ctx, e.Design)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Plan.Inputs) != 1 || len(p.Plan.Tasks) != 3 {
		t.Fatal(p.Plan)
	}
	tenant, resource, q := "tenantDefault", "resourceDefault", int64(2)
	in := []systemrun.Inputs{{TaskID: p.Plan.Inputs[0].TaskID, Values: map[string]model.TypedValue{"request": {Type: "record", RecordRef: p.Plan.Inputs[0].RecordRef, Record: map[string]model.TypedValue{"tenantId": {Type: "string", String: &tenant}, "resourceId": {Type: "string", String: &resource}, "quantity": {Type: "integer", Integer: &q}}}}}}
	run, err := s.ExecuteSystem(ctx, "state-handoff", e.Design, in)
	if err != nil {
		t.Fatal(err)
	}
	if *run.Report.Results[2].Outputs["remaining"].Integer != 3 || run.Report.Results[1].State == nil {
		t.Fatal(run.Report)
	}
	saved, err := s.SystemExecution(ctx, run.ID)
	if err != nil || !reflect.DeepEqual(saved, run) {
		t.Fatal("roundtrip", err)
	}
	zero := int64(0)
	scenario := systemrun.Scenario{Name: "Repeated handoff", Steps: []systemrun.ScenarioStep{{Kind: "invoke", Inputs: in, Calls: []systemrun.StateCall{{TaskID: systemrun.Key("stock", "stock"), CommandID: "reserveA", ExpectedRevision: &zero}}, ExpectedStatuses: map[string]string{systemrun.Key("stock", "stock"): "committed"}}, {Kind: "invoke", Inputs: in, Calls: []systemrun.StateCall{{TaskID: systemrun.Key("stock", "stock"), CommandID: "reserveA", ExpectedRevision: &zero}}, ExpectedStatuses: map[string]string{systemrun.Key("stock", "stock"): "replayed"}}}}
	shared, err := s.ExecuteSystemScenario(ctx, "shared-handoff", e.Design, scenario)
	if err != nil || !shared.Report.Scenario.Passed {
		t.Fatal("shared scenario", err)
	}
	stored, err := s.SystemExecution(ctx, shared.ID)
	if err != nil || !reflect.DeepEqual(stored, shared) {
		t.Fatal("shared roundtrip", err)
	}
	if _, err = s.ExecuteSystemScenario(ctx, shared.ID, e.Design, scenario); err == nil {
		t.Fatal("overwrote scenario evidence")
	}
	scenario.Steps[1].ExpectedStatuses[systemrun.Key("stock", "stock")] = "committed"
	failed, err := s.ExecuteSystemScenario(ctx, "failed-expectation", e.Design, scenario)
	if err != nil || failed.Report.Scenario.Passed {
		t.Fatal("failed verdict not recordable", err)
	}
	for i := range scenario.Steps {
		scenario.Steps[i].ExpectedStatuses = nil
	}
	unverified, err := s.ExecuteSystemScenario(ctx, "unverified-expectation", e.Design, scenario)
	if err != nil || unverified.Report.Scenario.Verified || unverified.Report.Scenario.Passed {
		t.Fatal("missing expectations promoted to pass", err)
	}
	scenario.Steps[0].Calls = nil
	if _, err = s.ExecuteSystemScenario(ctx, "malformed-scenario", e.Design, scenario); err == nil {
		t.Fatal("bad envelope accepted")
	}
	if _, err = s.SystemExecution(ctx, "malformed-scenario"); err != ErrNotFound {
		t.Fatal("bad scenario persisted", err)
	}
	e.Design.Components[1].Bindings[0].ResourceID = "missing"
	if _, err = s.ExecuteSystem(ctx, "bad-provider", e.Design, in); err == nil {
		t.Fatal("missing provider accepted")
	}
	if _, err = s.SystemExecution(ctx, "bad-provider"); err != ErrNotFound {
		t.Fatal("failure persisted", err)
	}
	e.Design.Components[1].Bindings[0].ResourceID = "stock-model"
	e.Design.Components[1].Bindings[0].Requirement = "state/wrong-slot"
	if _, err = s.PrepareSystemExecution(ctx, e.Design); err == nil {
		t.Fatal("unknown requirement accepted")
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM state_machines`); err != nil {
		t.Fatal(err)
	}
	sharedReplay, err := systemrun.RunScenario(ctx, stored.Plan, *stored.Scenario)
	if err != nil || !reflect.DeepEqual(sharedReplay, stored.Report) {
		t.Fatal("shared portable replay", err)
	}
	replay, err := systemrun.Run(ctx, saved.Plan, saved.Inputs)
	if err != nil || !reflect.DeepEqual(replay, saved.Report) {
		t.Fatal("portable replay", err)
	}
	for _, execution := range []SystemExecution{saved, stored, failed, unverified} {
		proof, err := s.ReplaySystemExecution(ctx, "verify-"+execution.ID, execution.ID)
		if err != nil || proof.Status != "matched" || proof.Report == nil || !reflect.DeepEqual(*proof.Report, execution.Report) {
			t.Fatalf("state/scenario verification %+v %v", proof, err)
		}
		if execution.ID == failed.ID && proof.Report.Scenario.Passed {
			t.Fatal("reproducing a failed scenario promoted it to passing")
		}
	}
}
