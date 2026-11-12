package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"software-studio/internal/model"
)

func textValue(text string) model.TypedValue { return model.TypedValue{Type: "string", String: &text} }
func integerValue(n int64) model.TypedValue  { return model.TypedValue{Type: "integer", Integer: &n} }

func simpleWorkflow(t *testing.T) (model.WorkflowLock, model.WorkflowScenario) {
	t.Helper()
	w := model.WorkflowDefinition{SchemaVersion: 1, ID: "test.text-workflow", Version: "1.0.0", Name: "Text fulfilment", Description: "Explicit typed response", Author: "LCE Studio", License: "MIT",
		DataContracts: []model.DataContractLock{}, Decisions: []model.ComponentBundle{}, Machines: []model.StateMachineLock{}, Owners: []model.WorkflowOwner{},
		Inputs: []model.BehaviorPort{{Name: "label", Type: "string"}}, Outputs: []model.BehaviorPort{{Name: "text", Type: "string"}},
		Variables: []model.WorkflowVariable{{Name: "text", Type: "string", Initial: textValue("initial")}}, Entry: "review",
		Nodes: []model.WorkflowNode{
			{ID: "review", Name: "Review text", Kind: "task", Next: "done", Task: &model.WorkflowTask{Kind: "human", Title: "Review", Instructions: "Provide the result explicitly.", Request: map[string]model.WorkflowBinding{"label": {Input: "label"}}, Response: []model.BehaviorPort{{Name: "result", Type: "string"}}, Assign: map[string]string{"text": "result"}}},
			{ID: "done", Name: "Done", Kind: "end", End: &model.WorkflowEnd{Outcome: "fulfilled", Outputs: map[string]model.WorkflowBinding{"text": {Variable: "text"}}}},
		}, Signals: []model.WorkflowSignal{}}
	digest, err := model.WorkflowDigest(w)
	if err != nil {
		t.Fatal(err)
	}
	s := model.WorkflowScenario{SchemaVersion: 1, Name: "Explicit fulfilment", Inputs: map[string]model.TypedValue{"label": textValue("draft")}, Events: []model.WorkflowEvent{{ID: "answer", Kind: "response", TaskID: "task1", Values: map[string]model.TypedValue{"result": textValue("approved")}}}}
	return model.WorkflowLock{Workflow: w, Digest: digest}, s
}

func TestExperimentOwnershipAndDeterminism(t *testing.T) {
	lock, scenario := simpleWorkflow(t)
	beforeLock, _ := json.Marshal(lock)
	beforeScenario, _ := json.Marshal(scenario)
	first, err := Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("replaying immutable input changed the result")
	}
	if first.Status != "completed" || first.Outcome != "fulfilled" || first.Verified || first.Passed || len(first.Steps) != 2 || first.Task != nil || len(first.Tasks) != 1 || *first.Outputs["text"].String != "approved" {
		t.Fatalf("unexpected result: %+v", first)
	}
	*first.Tasks[0].Request["label"].String = "changed request"
	*first.Tasks[0].Values["result"].String = "changed response"
	*first.Variables["text"].String = "changed variable"
	first.Tasks[0].Response[0].Name = "changed port"
	if *first.Outputs["text"].String != "approved" {
		t.Fatal("returned outputs alias variables")
	}
	afterLock, _ := json.Marshal(lock)
	afterScenario, _ := json.Marshal(scenario)
	if string(beforeLock) != string(afterLock) || string(beforeScenario) != string(afterScenario) {
		t.Fatal("returned evidence aliases the caller's definition or scenario")
	}
	third, err := Run(context.Background(), lock, scenario)
	if err != nil || !reflect.DeepEqual(second, third) {
		t.Fatalf("mutating old evidence changed a new experiment: %v", err)
	}
	scenario.Events = []model.WorkflowEvent{}
	waiting, err := Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	*waiting.Task.Request["label"].String = "changed active task"
	if *waiting.Tasks[0].Request["label"].String != "draft" {
		t.Fatal("active task aliases journal snapshot")
	}
}

func TestDecisionErrorIsRecordedButCancellationIsNot(t *testing.T) {
	lock, scenario := simpleWorkflow(t)
	p := model.ComponentPackage{SchemaVersion: 2, ID: "test.overflow", Version: "1.0.0", Name: "Overflow", Author: "LCE Studio", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"},
		Behavior: &model.Behavior{Inputs: []model.BehaviorPort{{Name: "n", Type: "integer"}}, Outputs: []model.BehaviorPort{{Name: "result", Type: "integer"}},
			Nodes: []model.BehaviorNode{{ID: "add", Op: "add", Inputs: map[string]model.BehaviorBinding{"left": {Input: "n"}, "right": {Literal: func() *model.TypedValue { v := integerValue(1); return &v }()}}}}, Bindings: map[string]model.BehaviorBinding{"result": {Node: "add", Port: "result"}}}}
	digest, err := model.ComponentDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	ref := model.ComponentRef{PackageID: p.ID, Version: p.Version, Digest: digest}
	lock.Workflow.Decisions = []model.ComponentBundle{{SchemaVersion: 1, Root: ref, Packages: []model.PackageLock{{Package: p, Digest: digest}}}}
	lock.Workflow.Entry = "overflow"
	n := integerValue(model.MaxBehaviorInteger)
	lock.Workflow.Nodes = append([]model.WorkflowNode{{ID: "overflow", Name: "Overflow", Kind: "decision", Decision: &model.WorkflowDecision{Component: ref, Inputs: map[string]model.WorkflowBinding{"n": {Literal: &n}}, Assign: map[string]string{}}, Next: "review"}}, lock.Workflow.Nodes...)
	lock.Digest, err = model.WorkflowDigest(lock.Workflow)
	if err != nil {
		t.Fatal(err)
	}
	scenario.Events = []model.WorkflowEvent{}
	result, err := Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || len(result.Steps) != 1 || result.Steps[0].Message == "" || result.Task != nil || result.Operations != 0 || len(result.Receipts) != 0 {
		t.Fatalf("ordinary decision failure was lost: %+v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = Run(ctx, lock, scenario)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("cancellation returned partial evidence: %+v %v", result, err)
	}
}

func TestEventHashExcludesIDButBindsEveryRequestField(t *testing.T) {
	event := model.WorkflowEvent{ID: "first", Kind: "response", TaskID: "task1", Values: map[string]model.TypedValue{"result": textValue("approved")}}
	original, err := eventFingerprint(event)
	if err != nil {
		t.Fatal(err)
	}
	event.ID = "second"
	same, _ := eventFingerprint(event)
	if original != same {
		t.Fatal("event ID leaked into request fingerprint")
	}
	event.TaskID = "task2"
	other, _ := eventFingerprint(event)
	if original == other {
		t.Fatal("task generation was omitted from request fingerprint")
	}
	// Pin the independent request object, including explicit empty signal.
	raw := []byte(`{"kind":"response","taskId":"task1","signal":"","values":{"result":{"type":"string","string":"approved"}}}`)
	if original != hash(raw) {
		t.Fatal("fingerprint does not use the frozen canonical request encoding")
	}
}
