package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func wfBool(value bool) TypedValue     { return TypedValue{Type: "boolean", Boolean: &value} }
func wfInteger(value int64) TypedValue { return TypedValue{Type: "integer", Integer: &value} }
func wfText(value string) TypedValue   { return TypedValue{Type: "string", String: &value} }
func wfFixture() WorkflowDefinition {
	end := func(id, outcome string) WorkflowNode {
		return WorkflowNode{ID: id, Name: id, Kind: "end", End: &WorkflowEnd{Outcome: outcome, Outputs: map[string]WorkflowBinding{"result": {Variable: "flag"}}}}
	}
	return WorkflowDefinition{SchemaVersion: 1, ID: "test.interactive", Version: "1.0.0", Name: "Interactive workflow", Author: "Tests", License: "MIT", DataContracts: []DataContractLock{}, Decisions: []ComponentBundle{}, Machines: []StateMachineLock{}, Owners: []WorkflowOwner{}, Inputs: []BehaviorPort{{Name: "name", Type: "string"}}, Outputs: []BehaviorPort{{Name: "result", Type: "boolean"}}, Variables: []WorkflowVariable{{Name: "flag", Type: "boolean", Initial: wfBool(false)}, {Name: "enabled", Type: "boolean", Initial: wfBool(true)}}, Entry: "question",
		Nodes:   []WorkflowNode{{ID: "question", Name: "Question", Kind: "task", Task: &WorkflowTask{Kind: "human", Title: "Review request", Instructions: "Record an explicit response.", Request: map[string]WorkflowBinding{"name": {Input: "name"}}, Response: []BehaviorPort{{Name: "answer", Type: "boolean"}}, Assign: map[string]string{"flag": "answer"}}, Branch: &WorkflowBranch{Variable: "flag", Then: "accepted", Else: "rejected"}}, end("accepted", "accepted"), end("rejected", "rejected"), end("interrupted", "interrupted")},
		Signals: []WorkflowSignal{{Name: "interrupt", Inputs: []BehaviorPort{{Name: "answer", Type: "boolean"}}, When: "enabled", Set: map[string]WorkflowBinding{"flag": {Input: "answer"}}, Target: "interrupted"}}}
}
func wfLock(t testing.TB, w WorkflowDefinition) WorkflowLock {
	t.Helper()
	digest, err := WorkflowDigest(w)
	if err != nil {
		t.Fatal(err, ValidateWorkflowDefinition(w))
	}
	return WorkflowLock{Workflow: w, Digest: digest}
}
func wfCopy(t testing.TB, w WorkflowDefinition) WorkflowDefinition {
	t.Helper()
	encoded, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var copy WorkflowDefinition
	if err := json.Unmarshal(encoded, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}
func wfScenario() WorkflowScenario {
	return WorkflowScenario{SchemaVersion: 1, Name: "Independent inputs", Inputs: map[string]TypedValue{"name": wfText("Person")}, Events: []WorkflowEvent{}}
}

func TestInteractiveWorkflowDefinitionContextAndNominalBindings(t *testing.T) {
	w := wfFixture()
	lock := wfLock(t, w)
	if issues := ValidateWorkflowLock(lock); len(issues) > 0 {
		t.Fatal(issues)
	}
	ctx, issues := BuildWorkflowContext(w)
	if len(issues) > 0 || len(ctx.Nodes) != 4 || ctx.Variables["flag"].Type != "boolean" || ctx.Inputs["name"].Type != "string" || ctx.Signals["interrupt"].When != "enabled" {
		t.Fatal(ctx, issues)
	}
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowDefinition)
	}{
		{"unknown schema", func(w *WorkflowDefinition) { w.SchemaVersion = 2 }},
		{"missing arrays", func(w *WorkflowDefinition) { w.Decisions = nil }},
		{"duplicate nodes", func(w *WorkflowDefinition) { w.Nodes[1].ID = "question" }},
		{"duplicate variables", func(w *WorkflowDefinition) { w.Variables[1].Name = "flag" }},
		{"duplicate response ports", func(w *WorkflowDefinition) {
			w.Nodes[0].Task.Response = append(w.Nodes[0].Task.Response, w.Nodes[0].Task.Response[0])
		}},
		{"unsupported metadata", func(w *WorkflowDefinition) { w.Nodes[0].End = w.Nodes[1].End }},
		{"unsupported task", func(w *WorkflowDefinition) { w.Nodes[0].Task.Kind = "script" }},
		{"ambiguous binding", func(w *WorkflowDefinition) {
			w.Nodes[0].Task.Request["name"] = WorkflowBinding{Input: "name", Variable: "flag"}
		}},
		{"missing source", func(w *WorkflowDefinition) { w.Nodes[0].Task.Request["name"] = WorkflowBinding{Input: "missing"} }},
		{"wrong assignment type", func(w *WorkflowDefinition) { w.Nodes[0].Task.Response[0].Type = "string" }},
		{"missing assigned output", func(w *WorkflowDefinition) { w.Nodes[0].Task.Assign["flag"] = "missing" }},
		{"missing assigned variable", func(w *WorkflowDefinition) { w.Nodes[0].Task.Assign["missing"] = "answer" }},
		{"missing root output", func(w *WorkflowDefinition) { w.Nodes[1].End.Outputs = map[string]WorkflowBinding{} }},
		{"extra root output", func(w *WorkflowDefinition) { w.Nodes[1].End.Outputs["extra"] = WorkflowBinding{Variable: "flag"} }},
		{"end continuation", func(w *WorkflowDefinition) { w.Nodes[1].Next = "question" }},
		{"next and branch", func(w *WorkflowDefinition) { w.Nodes[0].Next = "accepted" }},
		{"dangling branch", func(w *WorkflowDefinition) { w.Nodes[0].Branch.Then = "missing" }},
		{"nonboolean branch", func(w *WorkflowDefinition) { w.Variables[0].Type = "integer"; w.Variables[0].Initial = wfInteger(0) }},
		{"signal input context", func(w *WorkflowDefinition) { w.Signals[0].Set["flag"] = WorkflowBinding{Input: "name"} }},
		{"missing signal condition", func(w *WorkflowDefinition) { w.Signals[0].When = "missing" }},
		{"dangling signal", func(w *WorkflowDefinition) { w.Signals[0].Target = "missing" }},
		{"nan coordinate", func(w *WorkflowDefinition) { w.Nodes[0].Position.X = math.NaN() }},
		{"infinite coordinate", func(w *WorkflowDefinition) { w.Nodes[0].Position.Y = math.Inf(1) }},
		{"outside coordinate", func(w *WorkflowDefinition) { w.Nodes[0].Position.X = 1000001 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := wfCopy(t, w)
			test.mutate(&candidate)
			if issues := ValidateWorkflowDefinition(candidate); len(issues) == 0 {
				t.Fatal("invalid workflow accepted")
			}
		})
	}
	data, value := structuredTestRecord()
	w.DataContracts = []DataContractLock{data}
	w.Variables = append(w.Variables, WorkflowVariable{Name: "subject", Type: "record", RecordRef: value.RecordRef, Initial: value})
	w.Nodes[0].Task.Response = append(w.Nodes[0].Task.Response, BehaviorPort{Name: "record", Type: "record", RecordRef: value.RecordRef})
	w.Nodes[0].Task.Assign["subject"] = "record"
	if issues := ValidateWorkflowDefinition(w); len(issues) > 0 {
		t.Fatal(issues)
	}
	wrong := *value.RecordRef
	wrong.Record = "Other"
	w.Nodes[0].Task.Response[1].RecordRef = &wrong
	if issues := ValidateWorkflowDefinition(w); len(issues) == 0 {
		t.Fatal("equal-shaped unresolved nominal type accepted")
	}
	other := data.Contract
	other.ID = "test.other-domain"
	digest, err := DataContractDigest(other)
	if err != nil {
		t.Fatal(err)
	}
	w.DataContracts = append(w.DataContracts, DataContractLock{Contract: other, Digest: digest})
	wrong = *value.RecordRef
	wrong.ContractID, wrong.Digest = other.ID, digest
	w.Nodes[0].Task.Response[1].RecordRef = &wrong
	if issues := ValidateWorkflowDefinition(w); !issueAt(issues, "nodes[0].task.assign.subject") {
		t.Fatal("equal-shaped resolved nominal types treated as interchangeable", issues)
	}
}

func TestInteractiveWorkflowLoopsRequireCompletionOrInterruptEscape(t *testing.T) {
	w := wfFixture()
	w.Nodes = []WorkflowNode{w.Nodes[0], w.Nodes[3]}
	w.Nodes[0].Branch = nil
	w.Nodes[0].Next = "question"
	if issues := ValidateWorkflowDefinition(w); len(issues) > 0 {
		t.Fatal("interrupt-only waiting loop rejected", issues)
	}
	w.Signals = []WorkflowSignal{}
	if issues := ValidateWorkflowDefinition(w); len(issues) == 0 {
		t.Fatal("closed loop/unreachable terminal accepted")
	}
	w = wfFixture()
	w.Nodes = append(w.Nodes, WorkflowNode{ID: "unused", Name: "Unused", Kind: "end", End: &WorkflowEnd{Outcome: "unused", Outputs: map[string]WorkflowBinding{"result": {Variable: "flag"}}}})
	if issues := ValidateWorkflowDefinition(w); len(issues) == 0 {
		t.Fatal("unreachable node accepted")
	}
}

func TestInteractiveWorkflowScenarioEventRejectionsRemainModeled(t *testing.T) {
	lock := wfLock(t, wfFixture())
	scenario := wfScenario()
	scenario.Events = []WorkflowEvent{{ID: "response", Kind: "response", TaskID: "task99", Values: map[string]TypedValue{"unexpected": wfText("still structurally typed")}}, {ID: "signal", Kind: "signal", Signal: "undeclared", Values: map[string]TypedValue{}}, {ID: "signal", Kind: "signal", Signal: "interrupt", Values: map[string]TypedValue{"answer": wfInteger(1)}}}
	if issues := ValidateWorkflowScenario(lock, scenario); len(issues) > 0 {
		t.Fatal("runtime-rejected event rejected statically", issues)
	}
	scenario.Expected = &WorkflowExpectation{Status: "waiting", Outcome: "wrongOutcome", ActiveTaskNode: "", Variables: map[string]TypedValue{"flag": wfBool(true)}, Owners: []StateExpectation{}, Tasks: 5, CompletedTasks: 9, CancelledTasks: 7, EventReceipts: 30}
	if issues := ValidateWorkflowScenario(lock, scenario); len(issues) > 0 {
		t.Fatal("independent wrong expectations rejected", issues)
	}
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowScenario)
	}{
		{"null inputs", func(s *WorkflowScenario) { s.Inputs = nil }},
		{"missing input", func(s *WorkflowScenario) { s.Inputs = map[string]TypedValue{} }},
		{"extra input", func(s *WorkflowScenario) { s.Inputs["extra"] = wfBool(false) }},
		{"wrong input", func(s *WorkflowScenario) { s.Inputs["name"] = wfInteger(0) }},
		{"null events", func(s *WorkflowScenario) { s.Events = nil }},
		{"bad event kind", func(s *WorkflowScenario) { s.Events[0].Kind = "callback" }},
		{"cross-kind metadata", func(s *WorkflowScenario) { s.Events[0].Signal = "interrupt" }},
		{"null event values", func(s *WorkflowScenario) { s.Events[0].Values = nil }},
		{"unsafe event integer", func(s *WorkflowScenario) {
			s.Events[0].Values = map[string]TypedValue{"x": wfInteger(MaxBehaviorInteger + 1)}
		}},
		{"missing expected variable", func(s *WorkflowScenario) { s.Expected.Variables["missing"] = wfBool(false) }},
		{"wrong expected type", func(s *WorkflowScenario) { s.Expected.Variables["flag"] = wfInteger(0) }},
		{"negative count", func(s *WorkflowScenario) { s.Expected.Tasks = -1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, _ := json.Marshal(scenario)
			var candidate WorkflowScenario
			if err := json.Unmarshal(encoded, &candidate); err != nil {
				t.Fatal(err)
			}
			test.mutate(&candidate)
			if issues := ValidateWorkflowScenario(lock, candidate); len(issues) == 0 {
				t.Fatal("invalid scenario accepted")
			}
		})
	}
}

func TestInteractiveWorkflowExpectedJSONRequiresExplicitZeroAndEmpty(t *testing.T) {
	valid := `{"status":"waiting","outcome":"","activeTaskNode":"","variables":{},"owners":[],"tasks":0,"completedTasks":0,"cancelledTasks":0,"eventReceipts":0}`
	var expected WorkflowExpectation
	if err := json.Unmarshal([]byte(valid), &expected); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"status", "outcome", "activeTaskNode", "tasks", "completedTasks", "cancelledTasks", "eventReceipts"} {
		for _, null := range []bool{false, true} {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(valid), &fields); err != nil {
				t.Fatal(err)
			}
			if null {
				fields[name] = json.RawMessage("null")
			} else {
				delete(fields, name)
			}
			encoded, _ := json.Marshal(fields)
			if err := json.Unmarshal(encoded, &expected); err == nil {
				t.Fatalf("accepted absent/null %s, null=%v", name, null)
			}
		}
	}
	for _, raw := range []string{"null", strings.Replace(valid, `"tasks":0`, `"tasks":1.0`, 1), strings.Replace(valid, `"tasks":0`, `"tasks":"0"`, 1), strings.Replace(valid, `"tasks":0`, `"tasks":9007199254740992`, 1), strings.TrimSuffix(valid, "}") + `,"unknown":true}`, valid + " {}"} {
		err := json.Unmarshal([]byte(raw), &expected)
		if strings.Contains(raw, "9007199254740992") {
			if err == nil {
				scenario := wfScenario()
				scenario.Expected = &expected
				if issues := ValidateWorkflowScenario(wfLock(t, wfFixture()), scenario); len(issues) == 0 {
					t.Fatal("unsafe count accepted")
				}
			}
		} else if err == nil {
			t.Fatalf("malformed expectation accepted: %s", raw)
		}
	}
	var scenario WorkflowScenario
	decoder := json.NewDecoder(bytes.NewBufferString(`{"schemaVersion":1,"name":"x","inputs":{},"events":[],"unknown":true}`))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&scenario); err == nil {
		t.Fatal("strict scenario decoder accepted unknown field")
	}
}

func TestInteractiveWorkflowBoundsAndStableDigest(t *testing.T) {
	w := wfFixture()
	first := wfLock(t, w)
	copy := wfCopy(t, w)
	copy.Nodes[0].Task.Request = map[string]WorkflowBinding{"name": {Input: "name"}}
	second := wfLock(t, copy)
	if first.Digest != second.Digest {
		t.Fatal("map allocation changed typed digest")
	}
	copy.Nodes[0].Position.X = 1
	if changed := wfLock(t, copy); changed.Digest == first.Digest {
		t.Fatal("layout absent from exact digest")
	}
	first.Digest = strings.Repeat("0", 64)
	if issues := ValidateWorkflowLock(first); len(issues) == 0 {
		t.Fatal("tampered lock accepted")
	}
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowDefinition)
	}{
		{"variables", func(w *WorkflowDefinition) {
			for len(w.Variables) < 49 {
				w.Variables = append(w.Variables, w.Variables[0])
			}
		}},
		{"nodes", func(w *WorkflowDefinition) {
			for len(w.Nodes) < 97 {
				w.Nodes = append(w.Nodes, w.Nodes[0])
			}
		}},
		{"task response", func(w *WorkflowDefinition) {
			for len(w.Nodes[0].Task.Response) < 9 {
				w.Nodes[0].Task.Response = append(w.Nodes[0].Task.Response, w.Nodes[0].Task.Response[0])
			}
		}},
		{"signals", func(w *WorkflowDefinition) {
			for len(w.Signals) < 9 {
				w.Signals = append(w.Signals, w.Signals[0])
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := wfCopy(t, w)
			test.mutate(&candidate)
			if issues := ValidateWorkflowDefinition(candidate); len(issues) == 0 {
				t.Fatal("over-budget definition accepted")
			}
		})
	}
	values := map[string]TypedValue{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		values[name] = wfText(strings.Repeat("<", 4096))
	}
	if issues := ValidateWorkflowValues(values, []DataContractLock{}, 8, MaxStateValuesBytes); len(issues) == 0 {
		t.Fatal("escaped JSON map amplification accepted")
	}
	s := wfScenario()
	s.Events = make([]WorkflowEvent, 65)
	if issues := ValidateWorkflowScenario(wfLock(t, w), s); len(issues) == 0 {
		t.Fatal("65 events accepted")
	}
}

func wfOwnedFixture(t *testing.T) WorkflowDefinition {
	t.Helper()
	w := wfFixture()
	machine := stateTestMachineLock(t, stateTestMachine(t))
	w.Machines = []StateMachineLock{machine}
	w.Owners = []WorkflowOwner{{ID: "counter", Name: "Owned counter", Machine: DeliveryMachineRef{MachineID: machine.Machine.ID, Version: machine.Machine.Version, Digest: machine.Digest}, Initial: map[string]WorkflowBinding{"count": {Input: "count"}}}}
	w.Inputs = append(w.Inputs, BehaviorPort{Name: "count", Type: "integer"})
	w.Variables = append(w.Variables, WorkflowVariable{Name: "count", Type: "integer", Initial: wfInteger(0)}, WorkflowVariable{Name: "acceptedCommand", Type: "boolean", Initial: wfBool(false)})
	p := compositeTestPackage("test.workflow-decision")
	bundle, issues := PrepareComponentBundle(p, []PackageLock{})
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	w.Decisions = []ComponentBundle{bundle}
	w.Entry = "check"
	w.Nodes = append(w.Nodes,
		WorkflowNode{ID: "check", Name: "Pure check", Kind: "decision", Next: "write", Decision: &WorkflowDecision{Component: bundle.Root, Inputs: map[string]WorkflowBinding{"value": {Variable: "flag"}}, Assign: map[string]string{"flag": "result"}}},
		WorkflowNode{ID: "write", Name: "Owned write", Kind: "command", Next: "question", Command: &WorkflowCommand{Owner: "counter", Command: "set", Inputs: map[string]WorkflowBinding{"value": {Input: "count"}}, Assign: map[string]string{"count": "value"}, AcceptedVariable: "acceptedCommand", ErrorNext: "rejected"}})
	return w
}

func TestInteractiveWorkflowExactDecisionsOwnersAndCommands(t *testing.T) {
	w := wfOwnedFixture(t)
	lock := wfLock(t, w)
	ctx, issues := BuildWorkflowContext(w)
	if len(issues) > 0 || len(ctx.Owners) != 1 || len(ctx.Decisions) != 1 || ctx.Owners["counter"].Digest != w.Machines[0].Digest {
		t.Fatal(ctx, issues)
	}
	scenario := wfScenario()
	scenario.Inputs["count"] = wfInteger(4)
	scenario.Expected = &WorkflowExpectation{Status: "waiting", ActiveTaskNode: "question", Variables: map[string]TypedValue{}, Owners: []StateExpectation{{InstanceID: "counter", State: map[string]TypedValue{"count": wfInteger(4), "volatileCount": wfInteger(0)}, Revision: 1, Receipts: 1, Effects: 1}}}
	if issues := ValidateWorkflowScenario(lock, scenario); len(issues) > 0 {
		t.Fatal(issues)
	}
	before, _ := json.Marshal(w)
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowDefinition)
	}{
		{"stale decision digest", func(w *WorkflowDefinition) { w.Nodes[4].Decision.Component.Digest = strings.Repeat("0", 64) }},
		{"missing decision input", func(w *WorkflowDefinition) { delete(w.Nodes[4].Decision.Inputs, "value") }},
		{"extra decision input", func(w *WorkflowDefinition) { w.Nodes[4].Decision.Inputs["extra"] = WorkflowBinding{Variable: "flag"} }},
		{"wrong decision type", func(w *WorkflowDefinition) { w.Nodes[4].Decision.Inputs["value"] = WorkflowBinding{Input: "count"} }},
		{"wrong decision output", func(w *WorkflowDefinition) { w.Nodes[4].Decision.Assign["count"] = "result" }},
		{"stale owner digest", func(w *WorkflowDefinition) { w.Owners[0].Machine.Digest = strings.Repeat("0", 64) }},
		{"variable initializer", func(w *WorkflowDefinition) { w.Owners[0].Initial["count"] = WorkflowBinding{Variable: "count"} }},
		{"wrong initializer type", func(w *WorkflowDefinition) { w.Owners[0].Initial["count"] = WorkflowBinding{Input: "name"} }},
		{"unknown initializer slot", func(w *WorkflowDefinition) { w.Owners[0].Initial["missing"] = WorkflowBinding{Input: "count"} }},
		{"missing command owner", func(w *WorkflowDefinition) { w.Nodes[5].Command.Owner = "missing" }},
		{"missing owned command", func(w *WorkflowDefinition) { w.Nodes[5].Command.Command = "missing" }},
		{"missing command input", func(w *WorkflowDefinition) { delete(w.Nodes[5].Command.Inputs, "value") }},
		{"wrong command output", func(w *WorkflowDefinition) { w.Nodes[5].Command.Assign["flag"] = "value" }},
		{"wrong accepted type", func(w *WorkflowDefinition) { w.Nodes[5].Command.AcceptedVariable = "count" }},
		{"overlapping accepted variable", func(w *WorkflowDefinition) { w.Nodes[5].Command.Assign["acceptedCommand"] = "value" }},
		{"missing error edge", func(w *WorkflowDefinition) { w.Nodes[5].Command.ErrorNext = "" }},
		{"dangling error edge", func(w *WorkflowDefinition) { w.Nodes[5].Command.ErrorNext = "missing" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := wfCopy(t, w)
			test.mutate(&candidate)
			if issues := ValidateWorkflowDefinition(candidate); len(issues) == 0 {
				t.Fatal("invalid mapping accepted")
			}
		})
	}
	after, _ := json.Marshal(w)
	if !bytes.Equal(before, after) {
		t.Fatal("validation mutated definition")
	}
	protocol := wfCopy(t, w)
	protocol.Machines[0].Machine.Commands[0].Outputs = append(protocol.Machines[0].Machine.Commands[0].Outputs, BehaviorPort{Name: "accepted", Type: "boolean"})
	protocol.Machines[0].Machine.Commands[0].Results["accepted"] = "accepted"
	protocol.Machines[0] = stateTestMachineLock(t, protocol.Machines[0].Machine)
	protocol.Owners[0].Machine.Digest = protocol.Machines[0].Digest
	if issues := ValidateWorkflowDefinition(protocol); len(issues) > 0 {
		t.Fatal(issues)
	}
	protocol.Nodes[5].Command.Assign["acceptedCommand"] = "accepted"
	if issues := ValidateWorkflowDefinition(protocol); !issueAt(issues, "nodes[5].command.assign") {
		t.Fatal("well-typed acceptance/output destination ambiguity accepted", issues)
	}
	delete(scenario.Expected.Owners[0].State, "volatileCount")
	if issues := ValidateWorkflowScenario(lock, scenario); len(issues) == 0 {
		t.Fatal("incomplete owner expectation accepted")
	}
}

func TestInteractiveWorkflowIndependentClosuresRejectGlobalConflicts(t *testing.T) {
	w := wfOwnedFixture(t)
	// An unused but exact independently closed bundle shares the owned command's
	// package safely. Editing that identity is a conflict even though no workflow
	// decision node refers to it.
	shared := w.Machines[0].Machine.Commands[0].Decision
	w.Decisions = append(w.Decisions, shared)
	if issues := ValidateWorkflowDefinition(w); len(issues) > 0 {
		t.Fatal(issues)
	}
	changed := shared.Packages[0].Package
	changed.Name += " conflicting"
	other, issues := PrepareComponentBundle(changed, []PackageLock{})
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	w.Decisions[1] = other
	if issues := ValidateWorkflowDefinition(w); len(issues) == 0 {
		t.Fatal("cross-machine conflicting component identity accepted")
	}

	w = wfFixture()
	data, value := structuredTestRecord()
	p := compositeTestPackage("test.structured-workflow-decision")
	p.SchemaVersion = 3
	p.DataContracts = []DataContractLock{data}
	bundle, issues := PrepareComponentBundle(p, []PackageLock{})
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	w.Decisions = []ComponentBundle{bundle}
	w.DataContracts = []DataContractLock{data}
	w.Variables = append(w.Variables, WorkflowVariable{Name: "subject", Type: "record", RecordRef: value.RecordRef, Initial: value})
	if issues := ValidateWorkflowDefinition(w); len(issues) > 0 {
		t.Fatal(issues)
	}
	changedData := data.Contract
	changedData.Name += " conflicting"
	digest, err := DataContractDigest(changedData)
	if err != nil {
		t.Fatal(err)
	}
	w.DataContracts[0] = DataContractLock{Contract: changedData, Digest: digest}
	if issues := ValidateWorkflowDefinition(w); len(issues) == 0 {
		t.Fatal("cross-closure data identity conflict accepted")
	}

	w = wfFixture()
	complete := ExampleComponentBundles()[0]
	if len(complete.Packages) < 2 {
		t.Fatal("fixture requires dependency")
	}
	incomplete := complete
	incomplete.Packages = complete.Packages[:1]
	w.Decisions = []ComponentBundle{incomplete, {SchemaVersion: 1, Root: componentRefForWorkflow(complete.Packages[1]), Packages: complete.Packages[1:2]}}
	if issues := ValidateWorkflowDefinition(w); len(issues) == 0 {
		t.Fatal("independent bundle borrowed sibling closure")
	}
}

func TestInteractiveWorkflowMergedInitialAndAggregateByteBudgets(t *testing.T) {
	data, large, small := stateTestLargeRecord(t)
	w := wfFixture()
	w.DataContracts = []DataContractLock{data}
	w.Inputs = append(w.Inputs, BehaviorPort{Name: "payload", Type: "record", RecordRef: large.RecordRef})
	machine := stateTestMachine(t)
	machine.DataContracts = []DataContractLock{data}
	machine.State = []StateSlot{{Name: "first", Type: "record", RecordRef: large.RecordRef, Durability: "durable", Initial: large}, {Name: "second", Type: "record", RecordRef: small.RecordRef, Durability: "volatile", Initial: small}}
	machine.Commands[0].Writes = map[string]string{}
	lockedMachine := stateTestMachineLock(t, machine)
	w.Machines = []StateMachineLock{lockedMachine}
	w.Owners = []WorkflowOwner{{ID: "records", Name: "Owned records", Machine: DeliveryMachineRef{MachineID: machine.ID, Version: machine.Version, Digest: lockedMachine.Digest}, Initial: map[string]WorkflowBinding{"second": {Input: "payload"}}}}
	lock := wfLock(t, w)
	scenario := wfScenario()
	scenario.Inputs["payload"] = small
	if issues := ValidateWorkflowScenario(lock, scenario); len(issues) > 0 {
		t.Fatal(issues)
	}
	scenario.Inputs["payload"] = large
	if issues := ValidateWorkflowScenario(lock, scenario); !issueAt(issues, "owners[0].initial") {
		t.Fatal("merged owner state overflow accepted", issues)
	}

	w = wfFixture()
	w.DataContracts = []DataContractLock{data}
	lock = wfLock(t, w)
	scenario = wfScenario()
	for i := 0; i < 12; i++ {
		scenario.Events = append(scenario.Events, WorkflowEvent{ID: fmt.Sprintf("event%d", i), Kind: "signal", Signal: "unknown", Values: map[string]TypedValue{"payload": large}})
	}
	if issues := ValidateWorkflowScenario(lock, scenario); len(issues) > 0 {
		t.Fatal(issues)
	}
	scenario.Events = append(scenario.Events, scenario.Events[0])
	if issues := ValidateWorkflowScenario(lock, scenario); !issueAt(issues, "") {
		t.Fatal("aggregate scenario payload overflow accepted", issues)
	}

	values := map[string]TypedValue{}
	for i := 0; i < 6; i++ {
		values[fmt.Sprintf("value%d", i)] = large
	}
	if issues := ValidateWorkflowValues(values, []DataContractLock{data}, 48, MaxWorkflowVariablesBytes); len(issues) > 0 {
		t.Fatal(issues)
	}
	values["value6"] = large
	if issues := ValidateWorkflowValues(values, []DataContractLock{data}, 48, MaxWorkflowVariablesBytes); len(issues) == 0 {
		t.Fatal("aggregate variable overflow accepted")
	}

	// Each request literal is independently valid; total definition admission
	// must stop repeated values before full-document encoding can amplify them.
	for i := 0; i < 4; i++ {
		request := map[string]WorkflowBinding{}
		for j := 0; j < 8; j++ {
			request[fmt.Sprintf("value%d", j)] = WorkflowBinding{Literal: &large}
		}
		id := fmt.Sprintf("large%d", i)
		w.Nodes = append(w.Nodes, WorkflowNode{ID: id, Name: id, Kind: "task", Task: &WorkflowTask{Kind: "adapter", Title: id, Request: request, Response: []BehaviorPort{}, Assign: map[string]string{}}, Next: w.Entry})
		w.Entry = id
		if i == 2 {
			if issues := ValidateWorkflowDefinition(w); len(issues) > 0 {
				t.Fatal("bounded definition rejected", issues)
			}
		}
	}
	if issues := ValidateWorkflowDefinition(w); !issueAt(issues, "") {
		t.Fatal("aggregate definition overflow accepted", issues)
	}
}

func TestInteractiveWorkflowUnionLimitsAcrossIndependentBundles(t *testing.T) {
	w := wfFixture()
	for i := 0; i < 32; i++ {
		leaf := compositeTestLock(t, compositeTestPackage(fmt.Sprintf("test.leaf%d", i)))
		ref := compositeTestRef(leaf)
		root := compositeTestPackage(fmt.Sprintf("test.root%d", i))
		root.SchemaVersion = 3
		data := DataContract{SchemaVersion: 1, ID: fmt.Sprintf("test.data%d", i), Version: "1.0.0", Name: "Data", Author: "Tests", License: "MIT", Records: []DataRecord{{Name: "Record", Fields: []DataField{{Name: "value", Type: "boolean"}}}}, Entities: []DataEntity{}, Relationships: []DataRelationship{}}
		digest, err := DataContractDigest(data)
		if err != nil {
			t.Fatal(err)
		}
		root.DataContracts = []DataContractLock{{Contract: data, Digest: digest}}
		root.Behavior.Nodes = []BehaviorNode{{ID: "call", Component: &ref, Inputs: map[string]BehaviorBinding{"value": {Input: "value"}}}}
		root.Behavior.Bindings = map[string]BehaviorBinding{"result": {Node: "call", Port: "result"}}
		bundle, issues := PrepareComponentBundle(root, []PackageLock{leaf})
		if len(issues) > 0 {
			t.Fatal(issues)
		}
		w.Decisions = append(w.Decisions, bundle)
	}
	if issues := ValidateWorkflowDefinition(w); len(issues) > 0 {
		t.Fatal("exactly64 components/32 data rejected", issues)
	}
	contracts, issues := WorkflowDataContracts(w)
	if len(issues) > 0 || len(contracts) != 32 {
		t.Fatal(len(contracts), issues)
	}
	tooManyData := wfCopy(t, w)
	extraData := contracts[0].Contract
	extraData.ID = "test.extra-data"
	digest, err := DataContractDigest(extraData)
	if err != nil {
		t.Fatal(err)
	}
	tooManyData.DataContracts = []DataContractLock{{Contract: extraData, Digest: digest}}
	if issues := ValidateWorkflowDefinition(tooManyData); !issueAt(issues, "dataContracts") {
		t.Fatal("33 global data locks accepted", issues)
	}

	extraPackage := compositeTestLock(t, compositeTestPackage("test.extra-leaf"))
	extraRef := compositeTestRef(extraPackage)
	root := w.Decisions[0].Packages[0].Package
	dependencies := append([]PackageLock{}, w.Decisions[0].Packages[1:]...)
	root.Behavior.Nodes = append(root.Behavior.Nodes, BehaviorNode{ID: "extra", Component: &extraRef, Inputs: map[string]BehaviorBinding{"value": {Node: "call", Port: "result"}}})
	root.Behavior.Bindings["result"] = BehaviorBinding{Node: "extra", Port: "result"}
	dependencies = append(dependencies, extraPackage)
	w.Decisions[0], issues = PrepareComponentBundle(root, dependencies)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	if issues := ValidateWorkflowDefinition(w); !issueAt(issues, "decisions") {
		t.Fatal("65 global component locks accepted", issues)
	}
}
