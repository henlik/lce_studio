package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/model"
)

func heavyMachine(t testing.TB, failure bool) model.StateMachineLock {
	t.Helper()
	body := model.Behavior{Inputs: []model.BehaviorPort{{Name: "value", Type: "integer"}}, Outputs: []model.BehaviorPort{{Name: "result", Type: "integer"}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{}}
	previous := input("value")
	for i := 0; i < 32; i++ {
		id := fmt.Sprintf("node%02d", i)
		body.Nodes = append(body.Nodes, model.BehaviorNode{ID: id, Op: "add", Inputs: map[string]model.BehaviorBinding{"left": previous, "right": literal(integer(1))}})
		previous = output(id)
	}
	body.Bindings["result"] = previous
	leaf := packageLock(t, definition("test.heavy-leaf", body))
	root := model.Behavior{Inputs: []model.BehaviorPort{{Name: "value", Type: "integer"}}, Outputs: []model.BehaviorPort{{Name: "result", Type: "integer"}, {Name: "accepted", Type: "boolean"}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{"accepted": literal(boolean(true))}}
	previous = input("value")
	ref := componentRef(leaf)
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("call%02d", i)
		root.Nodes = append(root.Nodes, model.BehaviorNode{ID: id, Component: &ref, Inputs: map[string]model.BehaviorBinding{"value": previous}})
		previous = output(id)
	}
	root.Bindings["result"] = previous
	initial := integer(0)
	if failure {
		initial = integer(model.MaxBehaviorInteger)
	}
	machine := model.StateMachine{SchemaVersion: 1, ID: "test.heavy", Version: "1.0.0", Name: "Expanded work", Author: "Tests", License: "MIT", DataContracts: []model.DataContractLock{},
		State: []model.StateSlot{{Name: "value", Type: "integer", Durability: "durable", Initial: initial}}, Effects: []model.StateEffect{},
		Commands: []model.StateCommand{{Name: "change", Inputs: []model.BehaviorPort{}, Outputs: []model.BehaviorPort{{Name: "result", Type: "integer"}}, Decision: bundle(t, definition("test.heavy-root", root), leaf), Bindings: map[string]model.StateBinding{"value": {State: "value"}}, AcceptedOutput: "accepted", Writes: map[string]string{"value": "result"}, Results: map[string]string{"result": "result"}, Effects: []model.StateEffectBinding{}}},
	}
	return machineLock(t, machine)
}

func TestGlobalWorkBoundaryAndFailedDecisionReservations(t *testing.T) {
	for _, failure := range []bool{false, true} {
		lock := heavyMachine(t, failure)
		s := scenario()
		for i := 0; i < 16; i++ {
			rev := int64(i)
			if failure {
				rev = 0
			}
			s.Steps = append(s.Steps, model.StateStep{Kind: "command", InstanceID: "one", CommandID: fmt.Sprintf("request%d", i), Command: "change", ExpectedRevision: revision(rev), Inputs: emptyValues()})
		}
		r := runOK(t, lock, s)
		if failure {
			if r.Operations != 0 || r.Invocations != 0 || r.Instances[0].Revision != 0 || len(r.Instances[0].Receipts) != 0 {
				t.Fatal("failed partial work was reported as completed work")
			}
			for _, step := range r.Steps {
				if step.Status != "evaluation_error" {
					t.Fatal("expected failed decision")
				}
			}
		} else if r.Operations != 4096 || r.Invocations != 144 || *r.Instances[0].State["value"].Integer != 4096 || r.Instances[0].Revision != 16 {
			t.Fatal("expanded numeric reference or exact4096 boundary failed")
		}
		rev := int64(16)
		if failure {
			rev = 0
		}
		s.Steps = append(s.Steps, model.StateStep{Kind: "command", InstanceID: "one", CommandID: "overflow", Command: "change", ExpectedRevision: revision(rev), Inputs: emptyValues()})
		if r, err := Run(context.Background(), lock, s); !errors.Is(err, ErrLimit) || r.EngineVersion != "" || !strings.Contains(err.Error(), "global decision work") {
			t.Fatalf("failure=%v unbounded decision work: %+v %v", failure, r, err)
		}
	}
}

func TestRestartSnapshotAmplificationIsBounded(t *testing.T) {
	m := counterMachine(t).Machine
	for i := 0; i < 4; i++ {
		m.State = append(m.State, model.StateSlot{Name: fmt.Sprintf("text%d", i), Type: "string", Durability: "durable", Initial: textValue(strings.Repeat("x", 4096))})
	}
	lock := machineLock(t, m)
	s := scenario()
	for i := 0; i < 64; i++ {
		s.Steps = append(s.Steps, model.StateStep{Kind: "restart", InstanceID: "one"})
	}
	if r, err := Run(context.Background(), lock, s); !errors.Is(err, ErrLimit) || r.EngineVersion != "" || !strings.Contains(err.Error(), "serialized result") {
		t.Fatalf("snapshot amplification: %+v %v", r, err)
	}
	s.Steps = s.Steps[:60]
	r := runOK(t, lock, s)
	if len(r.Steps) != 60 || r.Operations != 0 || r.Invocations != 0 {
		t.Fatal("bounded restart trace unexpectedly consumed decision work")
	}
}

func collectionMachine(t testing.TB) (model.StateMachineLock, model.TypedValue) {
	t.Helper()
	contract := model.DataContract{SchemaVersion: 1, ID: "test.record-data", Version: "1.0.0", Name: "Record data", Author: "Tests", License: "MIT", Records: []model.DataRecord{{Name: "Item", Fields: []model.DataField{{Name: "name", Type: "string"}}}}, Entities: []model.DataEntity{}, Relationships: []model.DataRelationship{}}
	digest, err := model.DataContractDigest(contract)
	if err != nil {
		t.Fatal(err)
	}
	data := model.DataContractLock{Contract: contract, Digest: digest}
	ref := &model.DataTypeRef{ContractID: contract.ID, Version: contract.Version, Digest: digest, Record: "Item"}
	items := []model.TypedValue{{Type: "record", RecordRef: ref, Record: map[string]model.TypedValue{"name": textValue("original")}}}
	value := model.TypedValue{Type: "collection", RecordRef: ref, Collection: &items}
	port := func(name string) model.BehaviorPort {
		return model.BehaviorPort{Name: name, Type: "collection", RecordRef: ref}
	}
	p := definition("test.collection-decision", model.Behavior{Inputs: []model.BehaviorPort{port("value")}, Outputs: []model.BehaviorPort{port("result"), {Name: "accepted", Type: "boolean"}}, Nodes: []model.BehaviorNode{{ID: "copy", Op: "identity", Inputs: map[string]model.BehaviorBinding{"value": input("value")}}}, Bindings: map[string]model.BehaviorBinding{"result": output("copy"), "accepted": literal(boolean(true))}})
	p.SchemaVersion, p.DataContracts = 3, []model.DataContractLock{data}
	machine := model.StateMachine{SchemaVersion: 1, ID: "test.collection-owner", Version: "1.0.0", Name: "Collection owner", Author: "Tests", License: "MIT", DataContracts: []model.DataContractLock{}, State: []model.StateSlot{{Name: "items", Type: "collection", RecordRef: ref, Durability: "durable", Initial: value}},
		Effects:  []model.StateEffect{{Name: "Changed", Payload: []model.BehaviorPort{port("items")}}},
		Commands: []model.StateCommand{{Name: "replace", Inputs: []model.BehaviorPort{port("value")}, Outputs: []model.BehaviorPort{port("result")}, Decision: bundle(t, p), Bindings: map[string]model.StateBinding{"value": {Input: "value"}}, AcceptedOutput: "accepted", Writes: map[string]string{"items": "result"}, Results: map[string]string{"result": "result"}, Effects: []model.StateEffectBinding{{Effect: "Changed", Payload: map[string]string{"items": "result"}}}}},
	}
	return machineLock(t, machine), cloneValue(value)
}

func TestStructuredSnapshotsReceiptsAndEffectsAreIsolated(t *testing.T) {
	lock, value := collectionMachine(t)
	*(*value.Collection)[0].Record["name"].String = "replacement"
	s := scenario(model.StateStep{Kind: "command", InstanceID: "one", CommandID: "replace", Command: "replace", ExpectedRevision: revision(0), Inputs: map[string]model.TypedValue{"value": value}})
	s.Steps = append(s.Steps, s.Steps[0])
	beforeLock, _ := json.Marshal(lock)
	beforeScenario, _ := json.Marshal(s)
	r := runOK(t, lock, s)
	pristine := runOK(t, lock, s)
	*(*r.Instances[0].State["items"].Collection)[0].Record["name"].String = "state changed"
	r.Instances[0].State["items"].RecordRef.Record = "Changed"
	*(*r.Steps[0].Before["items"].Collection)[0].Record["name"].String = "before changed"
	*(*r.Steps[0].Receipt.Outputs["result"].Collection)[0].Record["name"].String = "receipt changed"
	r.Steps[0].Receipt.EffectIDs[0] = "changed"
	*(*r.Steps[0].Decision.Outputs["result"].Collection)[0].Record["name"].String = "decision changed"
	*(*r.Instances[0].Effects[0].Payload["items"].Collection)[0].Record["name"].String = "effect changed"
	for _, unchanged := range []model.TypedValue{r.Steps[0].After["items"], r.Steps[1].Before["items"], r.Steps[1].After["items"], r.Steps[1].Receipt.Outputs["result"], r.Instances[0].Receipts[0].Outputs["result"], r.Steps[0].Decision.Events[len(r.Steps[0].Decision.Events)-1].Outputs["result"]} {
		if *(*unchanged.Collection)[0].Record["name"].String != "replacement" || unchanged.RecordRef.Record != "Item" {
			t.Fatal("mutable alias crossed evidence boundaries")
		}
	}
	afterLock, _ := json.Marshal(lock)
	afterScenario, _ := json.Marshal(s)
	if string(beforeLock) != string(afterLock) || string(beforeScenario) != string(afterScenario) || !reflect.DeepEqual(pristine, runOK(t, lock, s)) {
		t.Fatal("returned evidence modified the original model or a later run")
	}
}

func TestPublicResultAndEffectAmplificationBeforeCommit(t *testing.T) {
	base, value := collectionMachine(t)
	for i := 0; i < 3; i++ {
		*value.Collection = append(*value.Collection, cloneValue((*value.Collection)[0]))
	}
	for i := range *value.Collection {
		(*value.Collection)[i].Record["name"] = textValue(strings.Repeat("x", 4096))
	}
	encoded, _ := json.Marshal(value)
	if len(encoded) <= 16*1024 || len(encoded) > 20*1024 {
		t.Fatalf("bad amplification fixture: %d", len(encoded))
	}
	for _, results := range []bool{false, true} {
		// Each value is valid; repeating it into five destinations crosses the
		// state-specific map budget while the pure decision remains small.
		copyJSON, _ := json.Marshal(base.Machine)
		var machine model.StateMachine
		if err := json.Unmarshal(copyJSON, &machine); err != nil {
			t.Fatal(err)
		}
		ports := []model.BehaviorPort{}
		mapping := map[string]string{}
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("items%d", i)
			ports = append(ports, model.BehaviorPort{Name: name, Type: "collection", RecordRef: value.RecordRef})
			mapping[name] = "result"
		}
		if results {
			machine.Commands[0].Outputs, machine.Commands[0].Results = ports, mapping
		} else {
			machine.Effects[0].Payload, machine.Commands[0].Effects[0].Payload = ports, mapping
		}
		s := scenario(model.StateStep{Kind: "command", InstanceID: "one", CommandID: "replace", Command: "replace", ExpectedRevision: revision(0), Inputs: map[string]model.TypedValue{"value": value}})
		if r, err := Run(context.Background(), machineLock(t, machine), s); !errors.Is(err, ErrLimit) || r.EngineVersion != "" || !strings.Contains(err.Error(), "64 KiB") {
			t.Fatalf("results=%v amplification escaped bounds: %+v %v", results, r, err)
		}
	}
}
