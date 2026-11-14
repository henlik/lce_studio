package systemrun

import (
	"context"
	"encoding/json"
	"reflect"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"testing"
)

func multiFixture(t *testing.T) (Plan, []Inputs) {
	t.Helper()
	p, in := stateFixture(t)
	e := system.MultiCommandExample()
	if issues := model.ValidateStateMachineLock(e.Machine); len(issues) > 0 {
		t.Fatal(issues)
	}
	p.Tasks[2].State.Machine = e.Machine
	p.Tasks[2].State.EntryCommand = "reserve"
	p, err := Prepare(p.Tasks, p.Links)
	if err != nil {
		t.Fatal(err)
	}
	return p, in
}
func ownerStep(id, command string, revision int64, values map[string]model.TypedValue, status, fault string) ScenarioStep {
	return ScenarioStep{Kind: "command", TaskID: "stock", Inputs: []Inputs{{TaskID: "stock", Values: values}}, Calls: []StateCall{{TaskID: "stock", Command: command, CommandID: id, ExpectedRevision: &revision, Fault: fault}}, ExpectedStatuses: map[string]string{"stock": status}}
}
func count(n int64) map[string]model.TypedValue {
	return map[string]model.TypedValue{"count": {Type: "integer", Integer: &n}}
}
func TestMultiCommandSharedOwnershipAndHistoricalReceipt(t *testing.T) {
	p, in := multiFixture(t)
	ctx := context.Background()
	zero := int64(0)
	if p.EngineVersion != MultiCommandVersion {
		t.Fatal(p.EngineVersion)
	}
	wave := ScenarioStep{Kind: "invoke", Inputs: in, Calls: []StateCall{{TaskID: "stock", CommandID: "reserveA", ExpectedRevision: &zero}}, ExpectedStatuses: map[string]string{"stock": "committed", "summary": "completed"}}
	scenario := Scenario{Name: "Independent reserve/adjust outcomes", Steps: []ScenarioStep{wave, ownerStep("adjustA", "adjust", 1, count(8), "committed", ""), ownerStep("reserveA", "reserve", 0, in[0].Values, "replayed", ""), ownerStep("reserveA", "adjust", 0, count(4), "id_conflict", ""), ownerStep("stale", "adjust", 1, count(4), "revision_conflict", ""), ownerStep("rollback", "adjust", 2, count(9), "rolled_back", "before_commit"), ownerStep("lost", "adjust", 2, count(7), "reply_lost", "after_commit_before_reply"), {Kind: "restart", TaskID: "stock"}, ownerStep("lost", "adjust", 2, count(7), "replayed", ""), ownerStep("denied", "adjust", 3, count(-1), "denied", "")}}
	r, err := RunScenario(ctx, p, scenario)
	if err != nil || r.EngineVersion != "system-scenario-2" || !r.Scenario.Passed {
		t.Fatalf("%+v %v", r, err)
	}
	final := r.Scenario.Final[0].Instance
	if *final.State["available"].Integer != 7 || *final.State["acceptedSinceRestart"].Integer != 0 || final.Revision != 3 || len(final.Receipts) != 4 || len(final.Effects) != 1 {
		t.Fatal(final)
	}
	for _, result := range r.Results {
		if result.ScenarioStep == 3 && (*result.Outputs["remaining"].Integer != 3 || *result.State.Instance.State["available"].Integer != 8) {
			t.Fatal("historical reply rewrote current state", result)
		}
		if result.ScenarioStep == 7 && (len(result.Outputs) != 0 || result.State.Step.Receipt == nil) {
			t.Fatal("lost reply emitted outputs", result)
		}
		if result.ScenarioStep > 1 && (result.TaskID != "stock" || result.State.Command == "") {
			t.Fatal("direct call activated another task", result)
		}
	}
	for _, event := range r.Events {
		if event.Kind == "transfer" && event.ScenarioStep > 1 {
			t.Fatal("direct command routed through graph", event)
		}
	}
	again, err := RunScenario(ctx, p, scenario)
	if err != nil || !reflect.DeepEqual(r, again) {
		t.Fatal("non-reproducible multi-command report", err)
	}
	for i := range scenario.Steps {
		scenario.Steps[i].ExpectedStatuses = nil
	}
	unverified, err := RunScenario(ctx, p, scenario)
	if err != nil || unverified.Scenario.Verified || unverified.Scenario.Passed {
		t.Fatal("missing assertions passed", err)
	}
}
func TestMultiCommandSelectionTypesAndIsolation(t *testing.T) {
	p, in := multiFixture(t)
	ctx := context.Background()
	task := p.Tasks[2]
	task.State.EntryCommand = ""
	if _, err := Prepare([]Task{task}, nil); err == nil {
		t.Fatal("missing entry accepted")
	}
	task.State.EntryCommand = "unknown"
	if _, err := Prepare([]Task{task}, nil); err == nil {
		t.Fatal("unknown entry accepted")
	}
	task.State.EntryCommand = "adjust"
	if _, err := Prepare(p.Tasks, p.Links); err == nil {
		t.Fatal("reserve ports bound to adjust")
	}
	plan, err := Prepare([]Task{task}, nil)
	if err != nil || len(plan.Inputs) != 1 || plan.Inputs[0].Name != "count" {
		t.Fatal(plan, err)
	}
	r, err := Run(ctx, plan, []Inputs{{TaskID: "stock", Values: count(9)}})
	if err != nil || *r.Results[0].Outputs["remaining"].Integer != 9 || r.Results[0].State.Command != "adjust" {
		t.Fatal(r, err)
	}
	if _, err := Run(ctx, plan, []Inputs{{TaskID: "stock", Values: in[0].Values}}); err == nil {
		t.Fatal("wrong command inputs accepted")
	}
	other := task
	other.ID = "other"
	plan, err = Prepare([]Task{task, other}, nil)
	if err != nil {
		t.Fatal(err)
	}
	scenario := Scenario{Name: "Isolated owners", Steps: []ScenarioStep{ownerStep("a", "adjust", 0, count(9), "committed", "")}}
	r, err = RunScenario(ctx, plan, scenario)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Scenario.Final {
		want := int64(5)
		rev := int64(0)
		if f.TaskID == "stock" {
			want = 9
			rev = 1
		}
		if *f.Instance.State["available"].Integer != want || f.Instance.Revision != rev {
			t.Fatal("owner alias", f)
		}
	}
}
func TestOwnerCommandRejectsMalformedSteps(t *testing.T) {
	p, _ := multiFixture(t)
	base := Scenario{Name: "Malformed call", Steps: []ScenarioStep{ownerStep("a", "adjust", 0, count(8), "committed", "")}}
	cases := []func(*ScenarioStep){func(s *ScenarioStep) { s.TaskID = "unknown" }, func(s *ScenarioStep) { s.Calls[0].Command = "unknown" }, func(s *ScenarioStep) { s.Calls[0].TaskID = "request" }, func(s *ScenarioStep) { s.Calls[0].ExpectedRevision = nil }, func(s *ScenarioStep) { s.Calls[0].CommandID = "" }, func(s *ScenarioStep) { s.Calls[0].Fault = "unexpected" }, func(s *ScenarioStep) { s.Calls = append(s.Calls, s.Calls[0]) }, func(s *ScenarioStep) { s.Inputs = nil }, func(s *ScenarioStep) { s.Inputs[0].TaskID = "other" }, func(s *ScenarioStep) {
		s.Inputs[0].Values["extra"] = model.TypedValue{Type: "integer", Integer: new(int64)}
	}, func(s *ScenarioStep) { s.Inputs[0].Values = map[string]model.TypedValue{} }, func(s *ScenarioStep) { s.ExpectedStatuses["summary"] = "completed" }, func(s *ScenarioStep) { s.Kind = "invoke"; s.TaskID = "" }, func(s *ScenarioStep) { s.Kind = "restart" }}
	for i, mutate := range cases {
		b, _ := json.Marshal(base)
		var scenario Scenario
		_ = json.Unmarshal(b, &scenario)
		mutate(&scenario.Steps[0])
		r, err := RunScenario(context.Background(), p, scenario)
		if err == nil || len(r.Results) != 0 || r.Scenario != nil {
			t.Fatalf("malformed case %d returned evidence %+v %v", i, r, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunScenario(ctx, p, base); err == nil {
		t.Fatal("cancelled run accepted")
	}
}
