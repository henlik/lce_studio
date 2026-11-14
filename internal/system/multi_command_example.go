package system

import "software-studio/internal/model"

// MultiCommandExample adds an explicit synthetic inventory adjustment operation.
// It is not a replenishment workflow, identity check or authorized live mutation.
func MultiCommandExample() StateExecutionExamplePack {
	e := StateExecutionExample()
	m := e.Machine.Machine
	m.Version = "2.0.0"
	m.Name = "Synthetic stock owner with adjustment"
	zero := int64(0)
	input := func(name string) model.BehaviorBinding { return model.BehaviorBinding{Input: name} }
	output := func(name string) model.BehaviorBinding { return model.BehaviorBinding{Node: name, Port: "result"} }
	p := model.ComponentPackage{SchemaVersion: 2, ID: "studio.example-stock-adjustment", Version: "1.0.0", Name: "Nonnegative stock adjustment", Description: "Synthetic decision: replace availability with an explicitly supplied nonnegative count", Author: "LCE Studio", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, Behavior: &model.Behavior{
		Inputs: []model.BehaviorPort{{Name: "available", Type: "integer"}, {Name: "count", Type: "integer"}}, Outputs: []model.BehaviorPort{{Name: "accepted", Type: "boolean"}, {Name: "remaining", Type: "integer"}},
		Nodes: []model.BehaviorNode{{ID: "valid", Op: "less_or_equal", Inputs: map[string]model.BehaviorBinding{"left": {Literal: &model.TypedValue{Type: "integer", Integer: &zero}}, "right": input("count")}}, {ID: "remaining", Op: "select", Inputs: map[string]model.BehaviorBinding{"condition": output("valid"), "trueValue": input("count"), "falseValue": input("available")}}}, Bindings: map[string]model.BehaviorBinding{"accepted": output("valid"), "remaining": output("remaining")}}}
	bundle, issues := model.PrepareComponentBundle(p, nil)
	if len(issues) > 0 {
		panic(issues)
	}
	m.Commands = append(m.Commands, model.StateCommand{Name: "adjust", Inputs: []model.BehaviorPort{{Name: "count", Type: "integer"}}, Outputs: []model.BehaviorPort{{Name: "accepted", Type: "boolean"}, {Name: "remaining", Type: "integer"}}, Decision: bundle, Bindings: map[string]model.StateBinding{"available": {State: "available"}, "count": {Input: "count"}}, AcceptedOutput: "accepted", Writes: map[string]string{"available": "remaining"}, Results: map[string]string{"accepted": "accepted", "remaining": "remaining"}, Effects: []model.StateEffectBinding{}})
	digest, err := model.StateMachineDigest(m)
	if err != nil {
		panic(err)
	}
	e.Machine = model.StateMachineLock{Machine: m, Digest: digest}
	e.Definition.Version = "2.0.0"
	e.Definition.Name = "Stock with reserve and adjust commands"
	e.Definition.Aspects[0].Version = m.Version
	e.Definition.Aspects[0].Digest = digest
	e.Design.ID = "multi-command-stock-example"
	e.Design.Name = "Shared stock commands"
	e.Design.Revision = "2.0.0"
	e.Design.Components[1].Definition.Version = e.Definition.Version
	e.Design.Components[1].StateEntries = []StateEntry{{Slot: "stock", Command: "reserve"}}
	e.Packages = append(e.Packages, p)
	return e
}
