package systemrun

import (
	"context"
	"fmt"
	"reflect"
	"software-studio/internal/model"
	"strings"
	"testing"
)

func fixtureTask(t *testing.T, id string, join bool) Task {
	t.Helper()
	p := model.ComponentPackage{SchemaVersion: 2, ID: "test." + id, Version: "1.0.0", Name: id, Description: "Test pure task", Author: "Test", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, Behavior: &model.Behavior{Inputs: []model.BehaviorPort{{Name: "value", Type: "boolean"}}, Outputs: []model.BehaviorPort{{Name: "result", Type: "boolean"}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{"result": {Input: "value"}}}}
	if join {
		p.Behavior.Inputs = append(p.Behavior.Inputs, model.BehaviorPort{Name: "other", Type: "boolean"})
		p.Behavior.Nodes = []model.BehaviorNode{{ID: "join", Op: "and", Inputs: map[string]model.BehaviorBinding{"left": {Input: "value"}, "right": {Input: "other"}}}}
		p.Behavior.Bindings["result"] = model.BehaviorBinding{Node: "join", Port: "result"}
	}
	bundle, issues := model.PrepareComponentBundle(p, nil)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	return Task{ID: id, ComponentID: id, Slot: "root", Name: id, Bundle: bundle}
}
func TestDeterministicFanoutJoinTraceAndInputSnapshot(t *testing.T) {
	tasks := []Task{fixtureTask(t, "d", true), fixtureTask(t, "b", false), fixtureTask(t, "a", false), fixtureTask(t, "c", false)}
	links := []Link{{ID: "ab", Source: "a", Target: "b", Output: "result", Input: "value"}, {ID: "ac", Source: "a", Target: "c", Output: "result", Input: "value"}, {ID: "bd", Source: "b", Target: "d", Output: "result", Input: "value"}, {ID: "cd", Source: "c", Target: "d", Output: "result", Input: "other"}}
	plan, err := Prepare(tasks, links)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Order, []string{"a", "b", "c", "d"}) {
		t.Fatal(plan.Order)
	}
	yes := true
	inputs := []Inputs{{TaskID: "a", Values: map[string]model.TypedValue{"value": {Type: "boolean", Boolean: &yes}}}}
	result, err := Run(context.Background(), plan, inputs)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Run(context.Background(), plan, inputs)
	if err != nil || !reflect.DeepEqual(result, again) {
		t.Fatal("nondeterministic run", err)
	}
	if len(result.Events) != 12 || len(result.Results) != 4 || !*result.Results[3].Outputs["result"].Boolean || result.ClaimsEvaluated {
		t.Fatalf("bad result %+v", result)
	}
	for i, e := range result.Events {
		if e.Sequence != i+1 {
			t.Fatal("trace sequence gap")
		}
	}
	yes = false
	if !*result.Results[0].Outputs["result"].Boolean {
		t.Fatal("result mutated through caller input")
	}
}
func TestCyclesAndIncompleteInputsFailWithoutPartialSuccess(t *testing.T) {
	tasks := []Task{fixtureTask(t, "a", false), fixtureTask(t, "b", false)}
	links := []Link{{ID: "ab", Source: "a", Target: "b", Output: "result", Input: "value"}, {ID: "ba", Source: "b", Target: "a", Output: "result", Input: "value"}}
	if _, err := Prepare(tasks, links); err == nil {
		t.Fatal("cycle accepted")
	}
	plan, err := Prepare(tasks, links[:1])
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	for _, inputs := range [][]Inputs{nil, {{TaskID: "unknown", Values: nil}}, {{TaskID: "a", Values: map[string]model.TypedValue{"value": {Type: "boolean", Boolean: &yes}}}, {TaskID: "b", Values: map[string]model.TypedValue{"value": {Type: "boolean", Boolean: &yes}}}}, {{TaskID: "a"}, {TaskID: "a"}}} {
		r, err := Run(context.Background(), plan, inputs)
		if err == nil || len(r.Events) > 0 || len(r.Results) > 0 {
			t.Fatal("invalid input produced partial success", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, plan, nil); err == nil {
		t.Fatal("cancel ignored")
	}
}

func TestAggregateExecutionBudgetRejectsWithoutPartialReport(t *testing.T) {
	task := fixtureTask(t, "chain", false)
	p := task.Bundle.Packages[0].Package
	for i := 0; i < 8; i++ {
		binding := model.BehaviorBinding{Input: "value"}
		if i > 0 {
			binding = model.BehaviorBinding{Node: fmt.Sprintf("n%d", i-1), Port: "result"}
		}
		p.Behavior.Nodes = append(p.Behavior.Nodes, model.BehaviorNode{ID: fmt.Sprintf("n%d", i), Op: "identity", Inputs: map[string]model.BehaviorBinding{"value": binding}})
	}
	p.Behavior.Bindings["result"] = model.BehaviorBinding{Node: "n7", Port: "result"}
	b, issues := model.PrepareComponentBundle(p, nil)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	rootPackage := fixtureTask(t, "many-calls", false).Bundle.Packages[0].Package
	for i := 0; i < 10; i++ {
		binding := model.BehaviorBinding{Input: "value"}
		if i > 0 {
			binding = model.BehaviorBinding{Node: fmt.Sprintf("c%d", i-1), Port: "result"}
		}
		ref := b.Root
		rootPackage.Behavior.Nodes = append(rootPackage.Behavior.Nodes, model.BehaviorNode{ID: fmt.Sprintf("c%d", i), Component: &ref, Inputs: map[string]model.BehaviorBinding{"value": binding}})
	}
	rootPackage.Behavior.Bindings["result"] = model.BehaviorBinding{Node: "c9", Port: "result"}
	b, issues = model.PrepareComponentBundle(rootPackage, b.Packages)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	tasks := []Task{}
	inputs := []Inputs{}
	yes := true
	for i := 0; i < 64; i++ {
		id := fmt.Sprintf("t%02d", i)
		tasks = append(tasks, Task{ID: id, ComponentID: id, Slot: "root", Bundle: b})
		inputs = append(inputs, Inputs{TaskID: id, Values: map[string]model.TypedValue{"value": {Type: "boolean", Boolean: &yes}}})
	}
	plan, err := Prepare(tasks, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), plan, inputs)
	if err == nil || !strings.Contains(err.Error(), "budget") || len(report.Events) > 0 {
		t.Fatal("aggregate budget not enforced", err)
	}
	if _, err = Prepare(append(tasks, tasks[0]), nil); err == nil {
		t.Fatal("task limit ignored")
	}
}
