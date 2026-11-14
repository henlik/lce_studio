package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
	"testing"
)

func TestM1IntegratedAcceptance(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pack, err := s.InstallM1Example(ctx)
	if err != nil {
		if v, ok := err.(*systemrun.Invalid); ok {
			t.Fatal(v.Issues)
		}
		t.Fatal(err)
	}
	if len(pack.Scenarios) != 6 {
		t.Fatal("incomplete acceptance suite")
	}
	for i, c := range pack.Scenarios {
		t.Run(c.Name, func(t *testing.T) {
			run, err := s.ExecuteSystemScenario(ctx, fmt.Sprintf("m1-%d", i), pack.Design, c.Scenario)
			if err != nil {
				t.Fatal(err)
			}
			if !run.Report.Scenario.Passed {
				t.Fatal(run.Report.Scenario.Assertions)
			}
			replay, err := s.ReplaySystemExecution(ctx, fmt.Sprintf("replay-m1-%d", i), run.ID)
			if err != nil || replay.Status != "matched" {
				t.Fatal(err, replay)
			}
		})
	}
}

func m1Copy[T any](v T) T {
	b, _ := json.Marshal(v)
	var result T
	_ = json.Unmarshal(b, &result)
	return result
}

func TestM1BindingsCorrelationAndPortableEvidence(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	pack, err := s.InstallM1Example(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.InstallM1Example(ctx); err != nil {
		t.Fatal("idempotent install", err)
	}
	p, err := s.PrepareSystemExecution(ctx, pack.Design)
	if err != nil {
		t.Fatal(err)
	}
	if p.Plan.EngineVersion != systemrun.UnifiedVersion || len(p.ProviderChecks) != 4 {
		t.Fatal("provider evidence", p)
	}
	for _, check := range p.ProviderChecks {
		if !check.Satisfied || len(check.Required) == 0 {
			t.Fatal(check)
		}
	}
	for name, edit := range map[string]func(*system.SystemDesign){
		"missing capability": func(d *system.SystemDesign) {
			d.Components[0].Bindings[0].RequiredCapabilities = []string{"live-network"}
		},
		"wrong workflow provider": func(d *system.SystemDesign) { d.Components[0].Bindings[2].ResourceID = "state-model" },
		"missing owner":           func(d *system.SystemDesign) { d.Components[0].WorkflowOwners = nil },
		"wrong machine": func(d *system.SystemDesign) {
			d.Components[0].WorkflowOwners[0].ComponentID = "ledger"
			d.Components[0].WorkflowOwners[0].StateSlot = "ledger"
		},
		"missing route": func(d *system.SystemDesign) { d.Components[0].WorkflowMessages = nil },
		"wrong response type": func(d *system.SystemDesign) {
			d.Components[0].WorkflowMessages[0].Response["accepted"] = "totalReserved"
		},
		"wrong source": func(d *system.SystemDesign) { d.Components[0].WorkflowMessages[0].SourceNodeID = "review" },
	} {
		t.Run(name, func(t *testing.T) {
			d := m1Copy(pack.Design)
			edit(&d)
			if _, err := s.PrepareSystemExecution(ctx, d); err == nil {
				t.Fatal("accepted invalid binding")
			}
		})
	}
	for name, edit := range map[string]func(*systemrun.Scenario){
		"forged broker response": func(c *systemrun.Scenario) {
			c.Steps = c.Steps[:2]
			c.Steps = append(c.Steps, systemrun.ScenarioStep{Kind: "workflow_event", Workflow: &systemrun.WorkflowCall{InstanceID: "reservation", Event: &model.WorkflowEvent{ID: "forged", Kind: "response", TaskID: "task2", Values: map[string]model.TypedValue{"accepted": {Type: "boolean", Boolean: m1Bool(true)}}}}})
		},
		"wrong task":        func(c *systemrun.Scenario) { c.Steps[2].Workflow.TaskID = "task99" },
		"wrong instance":    func(c *systemrun.Scenario) { c.Steps[2].Workflow.InstanceID = "unknown" },
		"reserved event ID": func(c *systemrun.Scenario) { c.Steps[1].Workflow.Event.ID = "ack_forged" },
		"changed duplicate start": func(c *systemrun.Scenario) {
			c.Steps[12].Workflow.Inputs["request"].Record["quantity"] = model.TypedValue{Type: "integer", Integer: m1Int(3)}
		},
		"duplicate expectation": func(c *systemrun.Scenario) { c.ExpectedMessages = append(c.ExpectedMessages, c.ExpectedMessages[0]) },
		"malformed input":       func(c *systemrun.Scenario) { delete(c.Steps[0].Workflow.Inputs, "request") },
	} {
		t.Run(name, func(t *testing.T) {
			c := m1Copy(pack.Scenarios[0].Scenario)
			edit(&c)
			if _, err := systemrun.RunScenario(ctx, p.Plan, c); err == nil {
				t.Fatal("accepted invalid scenario")
			}
		})
	}
	wrong := m1Copy(pack.Scenarios[0].Scenario)
	wrong.ExpectedWorkflows[0].Outcome = "wrongOutcome"
	report, err := systemrun.RunScenario(ctx, p.Plan, wrong)
	if err != nil || report.Scenario.Passed {
		t.Fatal("independent expectation", err)
	}
	// A later owner command reads the state already committed by the workflow.
	mixed := m1Copy(pack.Scenarios[0].Scenario)
	stockID := systemrun.Key("stock", "stock")
	revision := int64(1)
	inputs := m1Copy(mixed.Steps[0].Workflow.Inputs)
	inputs["request"].Record["quantity"] = model.TypedValue{Type: "integer", Integer: m1Int(1)}
	mixed.Steps = append(mixed.Steps, systemrun.ScenarioStep{Kind: "command", TaskID: stockID, Calls: []systemrun.StateCall{{TaskID: stockID, Command: "reserve", CommandID: "laterOwnerCall", ExpectedRevision: &revision}}, Inputs: []systemrun.Inputs{{TaskID: stockID, Values: inputs}}, ExpectedStatuses: map[string]string{stockID: "committed"}})
	mixed.Expected = nil
	mixed.ExpectedWorkflows = nil
	mixed.ExpectedMessages = nil
	report, err = systemrun.RunScenario(ctx, p.Plan, mixed)
	if err != nil || !report.Scenario.Passed {
		t.Fatal("shared command", err)
	}
	for _, final := range report.Scenario.Final {
		if final.TaskID == stockID {
			if *final.Instance.State["available"].Integer != 2 || final.Instance.Revision != 2 {
				t.Fatal("owner was copied", final)
			}
			if !reflect.DeepEqual(final.Instance, report.Scenario.Workflows[0].Result.Owners[0]) {
				t.Fatal("workflow snapshot diverged from shared owner")
			}
		}
	}
	run, err := s.ExecuteSystemScenario(ctx, "m1-portable", pack.Design, pack.Scenarios[0].Scenario)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.PlanHash) != 64 || len(run.ReportHash) != 64 || run.Report.EngineVersion != "system-scenario-4" || run.Report.ClaimsEvaluated {
		t.Fatal("provenance", run)
	}
	if _, err = s.db.ExecContext(ctx, "DELETE FROM component_definitions; DELETE FROM workflow_packages; DELETE FROM delivery_models; DELETE FROM state_machines; DELETE FROM component_packages; DELETE FROM data_contracts"); err != nil {
		t.Fatal(err)
	}
	replay, err := s.ReplaySystemExecution(ctx, "m1-after-catalog-deletion", run.ID)
	if err != nil || replay.Status != "matched" || !reflect.DeepEqual(*replay.Report, run.Report) {
		t.Fatal("portable replay", err, replay.Status)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = systemrun.RunScenario(cancelled, p.Plan, pack.Scenarios[0].Scenario); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
}

func TestM1WorkflowWithoutStateAndInstanceBound(t *testing.T) {
	w := model.WorkflowDefinition{SchemaVersion: 1, ID: "test.end-only", Version: "1.0.0", Name: "End only", Author: "LCE Studio", License: "MIT", DataContracts: []model.DataContractLock{}, Decisions: []model.ComponentBundle{}, Machines: []model.StateMachineLock{}, Owners: []model.WorkflowOwner{}, Inputs: []model.BehaviorPort{}, Outputs: []model.BehaviorPort{}, Variables: []model.WorkflowVariable{}, Signals: []model.WorkflowSignal{}, Entry: "done", Nodes: []model.WorkflowNode{{ID: "done", Name: "Done", Kind: "end", End: &model.WorkflowEnd{Outcome: "done", Outputs: map[string]model.WorkflowBinding{}}}}}
	digest, _ := model.WorkflowDigest(w)
	plan, err := systemrun.PrepareUnified(nil, nil, nil, []systemrun.WorkflowTask{{ID: "flow", Name: "Flow", Lock: model.WorkflowLock{Workflow: w, Digest: digest}, Provider: system.ExecutionProvider{ID: "workflow", Kind: "modeled-workflow", Version: "1.0.0"}}})
	if err != nil {
		if v, ok := err.(*systemrun.Invalid); ok {
			t.Fatal(v.Issues)
		}
		t.Fatal(err)
	}
	scenario := systemrun.Scenario{Name: "No shared owners", Steps: []systemrun.ScenarioStep{{Kind: "workflow_start", Workflow: &systemrun.WorkflowCall{WorkflowID: "flow", InstanceID: "first", Inputs: map[string]model.TypedValue{}}, ExpectedWorkflowStatus: "completed"}}}
	report, err := systemrun.RunScenario(context.Background(), plan, scenario)
	if err != nil || !report.Scenario.Passed {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		step := m1Copy(scenario.Steps[0])
		step.Workflow.InstanceID = fmt.Sprintf("extra%d", i)
		scenario.Steps = append(scenario.Steps, step)
	}
	if _, err = systemrun.RunScenario(context.Background(), plan, scenario); err == nil {
		t.Fatal("unbounded workflow instances")
	}
}

func m1Bool(v bool) *bool  { return &v }
func m1Int(v int64) *int64 { return &v }
