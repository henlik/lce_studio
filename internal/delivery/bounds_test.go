package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/state"
)

func heavyRelay(t testing.TB, id string) model.StateMachineLock {
	base := relayMachine(t).Machine
	leafBody := model.Behavior{Inputs: []model.BehaviorPort{{Name: "value", Type: "integer"}, {Name: "amount", Type: "integer"}}, Outputs: []model.BehaviorPort{{Name: "result", Type: "integer"}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{}}
	previous := input("value")
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("node%02d", i)
		leafBody.Nodes = append(leafBody.Nodes, model.BehaviorNode{ID: name, Op: "add", Inputs: map[string]model.BehaviorBinding{"left": previous, "right": input("amount")}})
		previous = output(name)
	}
	leafBody.Bindings["result"] = previous
	leaf := packageLock(t, definition("test.heavy-leaf", leafBody))
	root := *base.Commands[0].Decision.Packages[0].Package.Behavior
	root.Nodes = []model.BehaviorNode{}
	root.Bindings = map[string]model.BehaviorBinding{"ticks": input("ticks"), "amount": input("amount"), "permit": input("permit")}
	previous = input("value")
	ref := componentRef(leaf)
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("call%02d", i)
		root.Nodes = append(root.Nodes, model.BehaviorNode{ID: name, Component: &ref, Inputs: map[string]model.BehaviorBinding{"value": previous, "amount": input("amount")}})
		previous = output(name)
	}
	root.Bindings["next"] = previous
	base.ID, base.Commands = id, base.Commands[:1]
	base.Commands[0].Decision = bundle(t, definition("test.heavy-root", root), leaf)
	return machineLock(t, base)
}

func TestGlobalWorkIncludesSenderAndRecipientAcrossMachines(t *testing.T) {
	sender, receiver := heavyRelay(t, "test.heavy-source"), heavyRelay(t, "test.heavy-target")
	ref := func(lock model.StateMachineLock) model.DeliveryMachineRef {
		return model.DeliveryMachineRef{MachineID: lock.Machine.ID, Version: lock.Machine.Version, Digest: lock.Digest}
	}
	m := model.DeliveryModel{SchemaVersion: 1, ID: "test.work", Version: "1.0.0", Name: "Shared work", Author: "Tests", License: "MIT", Machines: []model.StateMachineLock{sender, receiver}, Instances: []model.DeliveryInstance{{ID: "a", Name: "Sender", Machine: ref(sender)}, {ID: "b", Name: "Receiver", Machine: ref(receiver)}}, Routes: []model.DeliveryRoute{{ID: "ab", Name: "Transfer", SourceInstanceID: "a", Effect: "Notice", TargetInstanceID: "b", Command: "add", Inputs: map[string]string{"amount": "amount", "permit": "permit"}}}}
	lock := modelLock(t, m)
	s := scenario()
	s.Initial = s.Initial[:2]
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("request%d", i)
		s.Steps = append(s.Steps, direct("a", id, int64(i), 1), dispatch("ab", id, id, int64(i)), deliver(id, ""))
	}
	r := runOK(t, lock, s)
	if r.Operations != 4096 || r.Invocations != 144 || *r.Instances[0].State["value"].Integer != 2048 || *r.Instances[1].State["value"].Integer != 2048 {
		t.Fatal("independent expanded work/state reference differs")
	}
	s.Steps = append(s.Steps, direct("a", "overflow", 8, 1))
	if r, err := Run(context.Background(), lock, s); !errors.Is(err, state.ErrLimit) || r.EngineVersion != "" || !strings.Contains(err.Error(), "global decision work") {
		t.Fatalf("global work reset across machines: %+v %v", r, err)
	}
}

func structuredNetwork(t testing.TB, repeated int) (model.DeliveryModelLock, model.TypedValue) {
	t.Helper()
	contract := model.DataContract{SchemaVersion: 1, ID: "test.payload-data", Version: "1.0.0", Name: "Payload", Author: "Tests", License: "MIT", Records: []model.DataRecord{{Name: "Item", Fields: []model.DataField{{Name: "text", Type: "string"}}}}, Entities: []model.DataEntity{}, Relationships: []model.DataRelationship{}}
	digest, err := model.DataContractDigest(contract)
	if err != nil {
		t.Fatal(err)
	}
	data := model.DataContractLock{Contract: contract, Digest: digest}
	ref := &model.DataTypeRef{ContractID: contract.ID, Version: contract.Version, Digest: digest, Record: "Item"}
	port := func(name string) model.BehaviorPort {
		return model.BehaviorPort{Name: name, Type: "collection", RecordRef: ref}
	}
	p := definition("test.payload-decision", model.Behavior{Inputs: []model.BehaviorPort{port("value")}, Outputs: []model.BehaviorPort{port("value"), {Name: "accepted", Type: "boolean"}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{"value": input("value"), "accepted": literal(boolean(true))}})
	p.SchemaVersion, p.DataContracts = 3, []model.DataContractLock{data}
	command := model.StateCommand{Name: "send", Inputs: []model.BehaviorPort{port("value")}, Outputs: []model.BehaviorPort{}, Decision: bundle(t, p), Bindings: map[string]model.StateBinding{"value": {Input: "value"}}, AcceptedOutput: "accepted", Writes: map[string]string{}, Results: map[string]string{}, Effects: []model.StateEffectBinding{{Effect: "Notice", Payload: map[string]string{"value": "value"}}}}
	source := machineLock(t, model.StateMachine{SchemaVersion: 1, ID: "test.payload-source", Version: "1.0.0", Name: "Payload source", Author: "Tests", License: "MIT", DataContracts: []model.DataContractLock{}, State: []model.StateSlot{{Name: "active", Type: "boolean", Durability: "durable", Initial: boolean(true)}}, Effects: []model.StateEffect{{Name: "Notice", Payload: []model.BehaviorPort{port("value")}}}, Commands: []model.StateCommand{command}})
	target := source.Machine
	target.ID, target.Commands = "test.payload-target", []model.StateCommand{}
	for j := 0; j < 8; j++ {
		receive := command
		receive.Name, receive.Inputs, receive.Effects = fmt.Sprintf("receive%d", j), []model.BehaviorPort{}, []model.StateEffectBinding{}
		receive.Bindings = map[string]model.StateBinding{"value": {Input: "value0"}}
		for i := 0; i < repeated; i++ {
			receive.Inputs = append(receive.Inputs, port(fmt.Sprintf("value%d", i)))
		}
		target.Commands = append(target.Commands, receive)
	}
	receiver := machineLock(t, target)
	machineRef := func(lock model.StateMachineLock) model.DeliveryMachineRef {
		return model.DeliveryMachineRef{MachineID: lock.Machine.ID, Version: lock.Machine.Version, Digest: lock.Digest}
	}
	m := model.DeliveryModel{SchemaVersion: 1, ID: "test.payload-network", Version: "1.0.0", Name: "Payload network", Author: "Tests", License: "MIT", Machines: []model.StateMachineLock{source, receiver}, Instances: []model.DeliveryInstance{{ID: "a", Name: "Source", Machine: machineRef(source)}, {ID: "b", Name: "Target", Machine: machineRef(receiver)}}, Routes: []model.DeliveryRoute{}}
	for j := 0; j < 8; j++ {
		mapping := map[string]string{}
		for i := 0; i < repeated; i++ {
			mapping[fmt.Sprintf("value%d", i)] = "value"
		}
		m.Routes = append(m.Routes, model.DeliveryRoute{ID: fmt.Sprintf("route%d", j), Name: "Typed route", SourceInstanceID: "a", Effect: "Notice", TargetInstanceID: "b", Command: fmt.Sprintf("receive%d", j), Inputs: mapping})
	}
	text := strings.Repeat("x", 4096)
	items := make([]model.TypedValue, 4)
	for i := range items {
		items[i] = model.TypedValue{Type: "record", RecordRef: ref, Record: map[string]model.TypedValue{"text": {Type: "string", String: &text}}}
	}
	return modelLock(t, m), model.TypedValue{Type: "collection", RecordRef: ref, Collection: &items}
}

func TestMappedValuesAndEnvelopeAmplificationAreBoundedBeforePublication(t *testing.T) {
	lock, value := structuredNetwork(t, 5)
	s := scenario(model.DeliveryStep{Kind: "command", InstanceID: "a", CommandID: "First", Command: "send", ExpectedRevision: revision(0), Inputs: map[string]model.TypedValue{"value": value}}, dispatch("route0", "First", "first", 0))
	s.Initial = s.Initial[:2]
	if r, err := Run(context.Background(), lock, s); !errors.Is(err, ErrLimit) || r.EngineVersion != "" || !strings.Contains(err.Error(), "64 KiB") {
		t.Fatalf("mapped input amplification: %+v %v", r, err)
	}
	lock, value = structuredNetwork(t, 1)
	for len(*value.Collection) < 14 {
		*value.Collection = append(*value.Collection, cloneValue((*value.Collection)[0]))
	}
	s = scenario()
	s.Initial = s.Initial[:2]
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("request%d", i)
		s.Steps = append(s.Steps, model.DeliveryStep{Kind: "command", InstanceID: "a", CommandID: id, Command: "send", ExpectedRevision: revision(int64(i)), Inputs: map[string]model.TypedValue{"value": value}})
		for j := 0; j < 8; j++ {
			s.Steps = append(s.Steps, dispatch(fmt.Sprintf("route%d", j), id, fmt.Sprintf("message%d", i*8+j), 0))
		}
	}
	if r, err := Run(context.Background(), lock, s); !errors.Is(err, ErrLimit) || r.EngineVersion != "" || !strings.Contains(err.Error(), "serialized result") {
		t.Fatalf("large envelope amplification: %+v %v", r, err)
	}
	s.Steps = s.Steps[:9]
	r := runOK(t, lock, s)
	if len(r.Messages) != 8 || r.Operations != 0 || r.Invocations != 1 {
		t.Fatal("bounded envelopes were delivered automatically")
	}
	// Structured envelope fields and nominal references are independently owned.
	*r.Messages[0].Inputs["value0"].Collection = []model.TypedValue{}
	r.Messages[1].Inputs["value0"].RecordRef.Record = "Changed"
	if len(*r.Messages[2].Inputs["value0"].Collection) != 14 || r.Messages[2].Inputs["value0"].RecordRef.Record != "Item" || len(*r.Instances[0].Effects[0].Payload["value"].Collection) != 14 {
		t.Fatal("fanout shares mutable structured envelopes")
	}
}
