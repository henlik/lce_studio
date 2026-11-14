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
	replay, err := systemrun.Run(ctx, saved.Plan, saved.Inputs)
	if err != nil || !reflect.DeepEqual(replay, saved.Report) {
		t.Fatal("portable replay", err)
	}
}
