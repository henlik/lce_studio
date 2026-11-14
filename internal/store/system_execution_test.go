package store

import (
	"context"
	"reflect"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
	"testing"
)

func encounterInputs(p systemrun.Plan) []systemrun.Inputs {
	port := p.Inputs[0]
	encounter, patient := "synthetic-1", "synthetic-patient-1"
	return []systemrun.Inputs{{TaskID: port.TaskID, Values: map[string]model.TypedValue{port.Name: {Type: "record", RecordRef: port.RecordRef, Record: map[string]model.TypedValue{"encounterId": {Type: "string", String: &encounter}, "patientId": {Type: "string", String: &patient}}}}}}
}
func TestSystemExecutionSavedReplayAndUnsupportedPaths(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	example, err := s.InstallConnectionExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := s.PrepareSystemExecution(ctx, example.Design)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Plan.Tasks) != 2 || len(prepared.Plan.Inputs) != 1 {
		t.Fatal(prepared.Plan)
	}
	inputs := encounterInputs(prepared.Plan)
	e, err := s.ExecuteSystem(ctx, "test-execution", example.Design, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Report.Events) != 5 || e.Report.Events[2].Kind != "transfer" || e.Report.ClaimsEvaluated {
		t.Fatal(e.Report)
	}
	saved, err := s.SystemExecution(ctx, e.ID)
	if err != nil || !reflect.DeepEqual(saved, e) {
		t.Fatal("snapshot roundtrip", err)
	}
	if _, err = s.ExecuteSystem(ctx, e.ID, example.Design, inputs); err == nil {
		t.Fatal("immutable execution overwritten")
	}
	rows, err := s.SystemExecutions(ctx, example.Design.ID)
	if err != nil || len(rows) != 1 || rows[0].TaskCount != 2 {
		t.Fatalf("history %+v %v", rows, err)
	}
	// Portable evidence replays without the catalog, including exact record contracts.
	if _, err = s.db.ExecContext(ctx, `DELETE FROM component_packages`); err != nil {
		t.Fatal(err)
	}
	replay, err := systemrun.Run(ctx, saved.Plan, saved.Inputs)
	if err != nil || !reflect.DeepEqual(replay, saved.Report) {
		t.Fatal("snapshot cannot replay independently", err)
	}
	if _, err = s.PrepareSystemExecution(ctx, example.Design); err == nil {
		t.Fatal("missing catalog accepted")
	}
	example, err = s.InstallConnectionExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PrepareSystemExecution(ctx, example.Mismatch); err == nil {
		t.Fatal("mismatched contracts executed")
	}
	if _, err = s.PrepareSystemExecution(ctx, system.ReceptionExample()); err == nil {
		t.Fatal("conformance/deployment silently treated as pure")
	}
	bad := example.Design
	bad.Connections = append([]system.Connection{}, bad.Connections...)
	bad.Connections[0].Kind = "event"
	if _, err = s.PrepareSystemExecution(ctx, bad); err == nil {
		t.Fatal("delivery semantics ignored")
	}
	if _, err = s.ExecuteSystem(ctx, "missing-inputs", example.Design, nil); err == nil {
		t.Fatal("missing input accepted")
	}
	if _, err = s.SystemExecution(ctx, "missing-inputs"); err != ErrNotFound {
		t.Fatal("failed execution saved success", err)
	}
}
func TestPureManifestInternalExecution(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := compositeLeaf("pure.leaf")
	record, _, err := s.InstallComponent(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	d := system.ComponentDefinition{SchemaVersion: 1, ID: "pure.manifest", Version: "1.0.0", Name: "Two stages", Aspects: []system.AspectRef{{Slot: "one", Kind: "component", ID: p.ID, Version: p.Version, Digest: record.Digest}, {Slot: "two", Kind: "component", ID: p.ID, Version: p.Version, Digest: record.Digest}}, Links: []system.AspectLink{{ID: "internal", Source: system.AspectEndpoint{Slot: "one", Interface: "outputs", Port: "result"}, Target: system.AspectEndpoint{Slot: "two", Interface: "inputs", Port: "value"}}}}
	if _, err = s.InstallDefinition(ctx, d); err != nil {
		t.Fatal(err)
	}
	design := system.ReceptionExample()
	design.Components = []system.ComponentInstance{{ID: "owner", Name: "Owner", Definition: system.DefinitionRef{Kind: "definition", ID: d.ID, Version: d.Version}}}
	design.Connections = nil
	design.DeploymentPlans = nil
	plan, err := s.PrepareSystemExecution(ctx, design)
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	e, err := s.ExecuteSystem(ctx, "manifest-run", design, []systemrun.Inputs{{TaskID: systemrun.Key("owner", "one"), Values: map[string]model.TypedValue{"value": {Type: "boolean", Boolean: &yes}}}})
	if err != nil || len(plan.Plan.Tasks) != 2 || len(e.Report.Results) != 2 || !*e.Report.Results[1].Outputs["result"].Boolean {
		t.Fatalf("internal flow %+v %v", e.Report, err)
	}
	machine := model.ExampleStateScenarios()[0].Machine
	_, _, err = s.InstallStateMachine(ctx, machine.Machine)
	if err != nil {
		t.Fatal(err)
	}
	d.Version = "2.0.0"
	d.Aspects = append(d.Aspects, system.AspectRef{Slot: "state", Kind: "state", ID: machine.Machine.ID, Version: machine.Machine.Version, Digest: machine.Digest})
	if _, err = s.InstallDefinition(ctx, d); err != nil {
		t.Fatal(err)
	}
	design.Components[0].Definition.Version = "2.0.0"
	if _, err = s.PrepareSystemExecution(ctx, design); err == nil {
		t.Fatal("stateful aspect ignored")
	}
}
