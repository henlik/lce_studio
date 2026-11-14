package systemrun

import (
	"context"
	"reflect"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"testing"
)

func stateFixture(t *testing.T) (Plan, []Inputs) {
	t.Helper()
	e := system.StateExecutionExample()
	tasks := []Task{}
	for i, p := range e.Packages {
		b, issues := model.PrepareComponentBundle(p, nil)
		if len(issues) > 0 {
			t.Fatal(issues)
		}
		id := []string{"request", "summary"}[i]
		tasks = append(tasks, Task{ID: id, Name: id, Bundle: b})
	}
	tasks = append(tasks, Task{ID: "stock", Name: "Stock", State: &StateTask{Machine: e.Machine, Provider: e.Design.ExecutionProviders[0]}})
	plan, err := Prepare(tasks, []Link{{ID: "in", Source: "request", Target: "stock", Output: "request", Input: "request"}, {ID: "out", Source: "stock", Target: "summary", Output: "remaining", Input: "remaining"}})
	if err != nil {
		t.Fatal(err)
	}
	tenant, resource, n := "tenantDefault", "resourceDefault", int64(2)
	value := model.TypedValue{Type: "record", RecordRef: e.Machine.Machine.Commands[0].Inputs[0].RecordRef, Record: map[string]model.TypedValue{"tenantId": {Type: "string", String: &tenant}, "resourceId": {Type: "string", String: &resource}, "quantity": {Type: "integer", Integer: &n}}}
	return plan, []Inputs{{TaskID: "request", Values: map[string]model.TypedValue{"request": value}}}
}
func TestStatefulHandoffCommitDenialAndFreshReplay(t *testing.T) {
	p, in := stateFixture(t)
	r, err := Run(context.Background(), p, in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != "stateful-dataflow" || r.EngineVersion != Version || len(r.Events) != 8 || r.ClaimsEvaluated {
		t.Fatal(r)
	}
	state := r.Results[1].State
	if state.Step.Status != "committed" || state.Instance.Revision != 1 || len(state.Instance.Receipts) != 1 || len(state.Instance.Effects) != 1 || *state.Instance.State["available"].Integer != 3 || *r.Results[2].Outputs["remaining"].Integer != 3 {
		t.Fatal(state)
	}
	again, err := Run(context.Background(), p, in)
	if err != nil || !reflect.DeepEqual(again, r) {
		t.Fatal("new run did not start fresh", err)
	}
	quantity := int64(9)
	in[0].Values["request"].Record["quantity"] = model.TypedValue{Type: "integer", Integer: &quantity}
	denied, err := Run(context.Background(), p, in)
	if err != nil {
		t.Fatal(err)
	}
	state = denied.Results[1].State
	if state.Step.Status != "denied" || state.Instance.Revision != 0 || len(state.Instance.Effects) != 0 || len(state.Instance.Receipts) != 1 || *denied.Results[2].Outputs["remaining"].Integer != 5 {
		t.Fatal("denial changed state", state)
	}
	if *r.Results[0].Outputs["request"].Record["quantity"].Integer != 2 {
		t.Fatal("input edit rewrote evidence")
	}
}
func TestStateProviderOwnershipAndRejection(t *testing.T) {
	p, in := stateFixture(t)
	task := p.Tasks[2]
	other := task
	other.ID = "other"
	other.Name = "Other"
	isolated, err := Prepare([]Task{task, other}, nil)
	if err != nil {
		t.Fatal(err)
	}
	values := in[0].Values
	r, err := Run(context.Background(), isolated, []Inputs{{TaskID: "stock", Values: values}, {TaskID: "other", Values: values}})
	if err != nil {
		t.Fatal(err)
	}
	a, b := r.Results[0].State.Instance, r.Results[1].State.Instance
	if *a.State["available"].Integer != 3 || *b.State["available"].Integer != 3 || a.Effects[0].ID == b.Effects[0].ID {
		t.Fatal("shared provider aliased owners", a, b)
	}
	task.State.Provider.Kind = "postgresql"
	if _, err = Prepare([]Task{task}, nil); err == nil {
		t.Fatal("live provider accepted")
	}
	task.State.Provider.Kind = "modeled-state"
	task.State.Machine.Machine.Commands = append(task.State.Machine.Machine.Commands, task.State.Machine.Machine.Commands[0])
	if _, err = Prepare([]Task{task}, nil); err == nil {
		t.Fatal("ambiguous commands accepted")
	}
}
