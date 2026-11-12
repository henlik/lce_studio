package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"software-studio/internal/model"
)

func integer(v int64) model.TypedValue        { return model.TypedValue{Type: "integer", Integer: &v} }
func boolean(v bool) model.TypedValue         { return model.TypedValue{Type: "boolean", Boolean: &v} }
func revision(v int64) *int64                 { return &v }
func input(name string) model.BehaviorBinding { return model.BehaviorBinding{Input: name} }
func output(node string) model.BehaviorBinding {
	return model.BehaviorBinding{Node: node, Port: "result"}
}
func literal(v model.TypedValue) model.BehaviorBinding { return model.BehaviorBinding{Literal: &v} }
func packageLock(t testing.TB, p model.ComponentPackage) model.PackageLock {
	t.Helper()
	digest, err := model.ComponentDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	return model.PackageLock{Package: p, Digest: digest}
}
func componentRef(lock model.PackageLock) model.ComponentRef {
	return model.ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}
}
func bundle(t testing.TB, p model.ComponentPackage, dependencies ...model.PackageLock) model.ComponentBundle {
	t.Helper()
	lock := packageLock(t, p)
	return model.ComponentBundle{SchemaVersion: 1, Root: componentRef(lock), Packages: append([]model.PackageLock{lock}, dependencies...)}
}
func definition(id string, b model.Behavior) model.ComponentPackage {
	return model.ComponentPackage{SchemaVersion: 2, ID: id, Version: "1.0.0", Name: id, Author: "Tests", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, Behavior: &b}
}
func machineLock(t testing.TB, m model.StateMachine) model.StateMachineLock {
	t.Helper()
	digest, err := model.StateMachineDigest(m)
	if err != nil {
		t.Fatal(err, model.ValidateStateMachine(m))
	}
	return model.StateMachineLock{Machine: m, Digest: digest}
}
func modelLock(t testing.TB, m model.DeliveryModel) model.DeliveryModelLock {
	t.Helper()
	digest, err := model.DeliveryModelDigest(m)
	if err != nil {
		t.Fatal(err, model.ValidateDeliveryModel(m))
	}
	return model.DeliveryModelLock{Model: m, Digest: digest}
}
func relayMachine(t testing.TB) model.StateMachineLock {
	port := func(name, kind string) model.BehaviorPort { return model.BehaviorPort{Name: name, Type: kind} }
	p := definition("test.relay-decision", model.Behavior{Inputs: []model.BehaviorPort{port("value", "integer"), port("ticks", "integer"), port("amount", "integer"), port("permit", "boolean")}, Outputs: []model.BehaviorPort{port("next", "integer"), port("ticks", "integer"), port("amount", "integer"), port("permit", "boolean")},
		Nodes:    []model.BehaviorNode{{ID: "next", Op: "add", Inputs: map[string]model.BehaviorBinding{"left": input("value"), "right": input("amount")}}, {ID: "ticks", Op: "add", Inputs: map[string]model.BehaviorBinding{"left": input("ticks"), "right": literal(integer(1))}}},
		Bindings: map[string]model.BehaviorBinding{"next": output("next"), "ticks": output("ticks"), "amount": input("amount"), "permit": input("permit")}})
	command := model.StateCommand{Name: "add", Inputs: []model.BehaviorPort{port("amount", "integer"), port("permit", "boolean")}, Outputs: []model.BehaviorPort{port("value", "integer"), port("ticks", "integer")}, Decision: bundle(t, p),
		Bindings: map[string]model.StateBinding{"value": {State: "value"}, "ticks": {State: "ticks"}, "amount": {Input: "amount"}, "permit": {Input: "permit"}}, AcceptedOutput: "permit", Writes: map[string]string{"value": "next", "ticks": "ticks"}, Results: map[string]string{"value": "next", "ticks": "ticks"}, Effects: []model.StateEffectBinding{{Effect: "Notice", Payload: map[string]string{"amount": "amount", "permit": "permit"}}}}
	denied := command
	denied.Name = "deny"
	denied.Bindings = map[string]model.StateBinding{"value": {State: "value"}, "ticks": {State: "ticks"}, "amount": {Input: "amount"}, "permit": {Literal: func() *model.TypedValue { value := boolean(false); return &value }()}}
	return machineLock(t, model.StateMachine{SchemaVersion: 1, ID: "test.relay", Version: "1.0.0", Name: "Generic relay", Author: "Tests", License: "MIT", DataContracts: []model.DataContractLock{},
		State: []model.StateSlot{{Name: "value", Type: "integer", Durability: "durable", Initial: integer(0)}, {Name: "ticks", Type: "integer", Durability: "volatile", Initial: integer(0)}}, Effects: []model.StateEffect{{Name: "Notice", Payload: command.Inputs}}, Commands: []model.StateCommand{command, denied}})
}
func relayModel(t testing.TB) model.DeliveryModelLock {
	machine := relayMachine(t)
	ref := model.DeliveryMachineRef{MachineID: machine.Machine.ID, Version: machine.Machine.Version, Digest: machine.Digest}
	m := model.DeliveryModel{SchemaVersion: 1, ID: "test.network", Version: "1.0.0", Name: "Relay network", Author: "Tests", License: "MIT", Machines: []model.StateMachineLock{machine}, Instances: []model.DeliveryInstance{}, Routes: []model.DeliveryRoute{}}
	for _, id := range []string{"a", "b", "c"} {
		m.Instances = append(m.Instances, model.DeliveryInstance{ID: id, Name: id, Machine: ref})
	}
	for _, route := range []struct{ id, source, target, command string }{{"ab", "a", "b", "add"}, {"bc", "b", "c", "add"}, {"ba", "b", "a", "add"}, {"ac", "a", "c", "add"}, {"adeny", "a", "b", "deny"}} {
		m.Routes = append(m.Routes, model.DeliveryRoute{ID: route.id, Name: route.id, SourceInstanceID: route.source, Effect: "Notice", TargetInstanceID: route.target, Command: route.command, Inputs: map[string]string{"amount": "amount", "permit": "permit"}})
	}
	return modelLock(t, m)
}
func direct(instance, id string, rev, amount int64) model.DeliveryStep {
	return model.DeliveryStep{Kind: "command", InstanceID: instance, CommandID: id, Command: "add", ExpectedRevision: revision(rev), Inputs: map[string]model.TypedValue{"amount": integer(amount), "permit": boolean(true)}}
}
func dispatch(route, command, alias string, rev int64) model.DeliveryStep {
	return model.DeliveryStep{Kind: "dispatch", RouteID: route, SourceCommandID: command, Message: alias, ExpectedRevision: revision(rev)}
}
func downstream(route, source, alias string, rev int64) model.DeliveryStep {
	return model.DeliveryStep{Kind: "dispatch", RouteID: route, SourceMessage: source, Message: alias, ExpectedRevision: revision(rev)}
}
func deliver(alias, fault string) model.DeliveryStep {
	return model.DeliveryStep{Kind: "deliver", Message: alias, Fault: fault}
}
func scenario(steps ...model.DeliveryStep) model.DeliveryScenario {
	return model.DeliveryScenario{SchemaVersion: 1, Name: "Relay experiment", Initial: []model.DeliveryInitial{{InstanceID: "a", Initial: map[string]model.TypedValue{}}, {InstanceID: "b", Initial: map[string]model.TypedValue{}}, {InstanceID: "c", Initial: map[string]model.TypedValue{}}}, Steps: steps, Expected: []model.StateExpectation{}, ExpectedMessages: []model.DeliveryMessageExpectation{}}
}
func runOK(t testing.TB, lock model.DeliveryModelLock, scenario model.DeliveryScenario) Result {
	t.Helper()
	result, err := Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > MaxResultBytes {
		t.Fatalf("invalid public evidence: %d %v", len(encoded), err)
	}
	if result.Instances == nil || result.Messages == nil || result.Steps == nil || result.Expectations == nil || result.MessageExpectations == nil {
		t.Fatal("null result arrays")
	}
	for i, step := range result.Steps {
		if step.Index != i || (step.State != nil && step.State.Index != i) {
			t.Fatal("outer/kernel indices diverged")
		}
	}
	return result
}

func TestDownstreamBeforeAckCyclesFanoutAndIndependentReference(t *testing.T) {
	lock := relayModel(t)
	s := scenario(dispatch("ab", "First", "first", 0), downstream("bc", "first", "down", 0), deliver("down", ""), direct("a", "First", 0, 2), dispatch("ab", "First", "first", 0),
		deliver("first", "drop_before_receive"), deliver("first", "before_recipient_commit"), deliver("first", "after_recipient_commit_before_ack"), downstream("bc", "first", "down", 0), deliver("down", ""),
		model.DeliveryStep{Kind: "restart", InstanceID: "b"}, deliver("first", "after_recipient_commit_before_ack"), deliver("first", "before_recipient_commit"), dispatch("ab", "First", "first", 0), dispatch("ab", "First", "first", 1), deliver("first", "drop_before_receive"),
		downstream("ba", "first", "cycle", 1), deliver("cycle", ""), dispatch("ac", "First", "fanout", 1), deliver("fanout", ""), dispatch("adeny", "First", "denied", 1), deliver("denied", ""), direct("b", "Manual", 1, 1), direct("a", "Second", 2, 1), dispatch("ab", "Second", "stale", 0), deliver("stale", ""), deliver("stale", "after_recipient_commit_before_ack"))
	s.Expected = []model.StateExpectation{{InstanceID: "a", State: map[string]model.TypedValue{"value": integer(5), "ticks": integer(3)}, Revision: 3, Receipts: 3, Effects: 3}, {InstanceID: "b", State: map[string]model.TypedValue{"value": integer(3), "ticks": integer(1)}, Revision: 2, Receipts: 3, Effects: 2}, {InstanceID: "c", State: map[string]model.TypedValue{"value": integer(4), "ticks": integer(2)}, Revision: 2, Receipts: 2, Effects: 2}}
	s.ExpectedMessages = []model.DeliveryMessageExpectation{{Name: "first", Status: "acknowledged", Attempts: 5}, {Name: "down", Status: "acknowledged", Attempts: 1}, {Name: "cycle", Status: "acknowledged", Attempts: 1}, {Name: "fanout", Status: "acknowledged", Attempts: 1}, {Name: "denied", Status: "acknowledged", Attempts: 1, Accepted: boolean(false).Boolean}, {Name: "stale", Status: "pending", Attempts: 2}, {Name: "never", Status: "absent", Attempts: 0}}
	r := runOK(t, lock, s)
	want := []string{"missing_intent", "missing_source_message", "missing_message", "committed", "dispatched", "dropped", "recipient_unacknowledged", "acknowledgement_lost", "dispatched", "acknowledged", "restarted", "acknowledgement_lost", "acknowledged", "dispatch_replayed", "dispatch_conflict", "already_acknowledged", "dispatched", "acknowledged", "dispatched", "acknowledged", "dispatched", "acknowledged", "committed", "committed", "dispatched", "recipient_unacknowledged", "recipient_unacknowledged"}
	for i, status := range want {
		if r.Steps[i].Status != status {
			t.Fatalf("step%d got%s want%s", i, r.Steps[i].Status, status)
		}
	}
	if !r.Verified || !r.Passed || r.Operations != 18 || r.Invocations != 9 || len(r.Messages) != 6 {
		t.Fatalf("reference totals differ: ops%d calls%d passed%v messages%d", r.Operations, r.Invocations, r.Passed, len(r.Messages))
	}
	if r.Steps[5].State != nil || r.Steps[6].State.Status != "rolled_back" || r.Steps[11].State.Status != "replayed" || !r.Steps[11].FaultApplied || r.Steps[12].FaultApplied || r.Steps[15].State != nil || r.Steps[15].FaultApplied {
		t.Fatal("fault/retry boundary changed")
	}
	if r.Messages[0].ExpectedRevision != 0 || *r.Messages[0].Acknowledgement.Outputs["ticks"].Integer != 1 || *r.Steps[12].State.After["ticks"].Integer != 0 {
		t.Fatal("replay rebased envelope or replaced current state with its historical reply")
	}
	for _, i := range []int{25, 26} {
		if r.Steps[i].State.Status != "revision_conflict" || r.Steps[i].FaultApplied {
			t.Fatal("stale revision silently rebased or reached ack-loss point")
		}
	}
	if r.Messages[1].SourceCommandID != r.Messages[0].ID || r.Messages[2].SourceCommandID != r.Messages[0].ID {
		t.Fatal("downstream selector lost original recipient command identity")
	}
	identity, _ := json.Marshal([]string{r.Messages[0].SourceIntentID, "ab"})
	digest := sha256.Sum256(identity)
	wantID := "msg_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:]))
	if r.Messages[0].ID != wantID || len(wantID) != 56 || r.Messages[0].ID == r.Messages[3].ID {
		t.Fatal("message identity truncated or omitted route")
	}
	if !reflect.DeepEqual(r, runOK(t, lock, s)) {
		t.Fatal("fresh delivery run changed deterministic evidence")
	}
}

func TestHistoricalAckAndRecipientEvidenceHaveNoAliases(t *testing.T) {
	lock := relayModel(t)
	s := scenario(direct("a", "First", 0, 2), dispatch("ab", "First", "first", 0), deliver("first", ""))
	beforeModel, _ := json.Marshal(lock)
	beforeScenario, _ := json.Marshal(s)
	r := runOK(t, lock, s)
	*r.Messages[0].Inputs["amount"].Integer = 99
	*r.Messages[0].Acknowledgement.Outputs["value"].Integer = 77
	*r.Steps[2].State.After["value"].Integer = 66
	*r.Instances[1].Effects[0].Payload["amount"].Integer = 55
	if *r.Steps[2].State.Receipt.Outputs["value"].Integer != 2 || *r.Instances[1].State["value"].Integer != 2 || *r.Instances[1].Receipts[0].Outputs["value"].Integer != 2 || *r.Steps[2].State.Decision.Outputs["next"].Integer != 2 {
		t.Fatal("ack/message/state snapshots share mutable values")
	}
	afterModel, _ := json.Marshal(lock)
	afterScenario, _ := json.Marshal(s)
	if string(beforeModel) != string(afterModel) || string(beforeScenario) != string(afterScenario) {
		t.Fatal("result changed supplied model/scenario")
	}
	if again := runOK(t, lock, s); *again.Messages[0].Inputs["amount"].Integer != 2 {
		t.Fatal("result changed a fresh run")
	}
	if r.Verified || r.Passed {
		t.Fatal("no expectations implies verification")
	}
	s.ExpectedMessages = []model.DeliveryMessageExpectation{{Name: "first", Status: "pending", Attempts: 7}}
	r = runOK(t, lock, s)
	if !r.Verified || r.Passed || len(r.MessageExpectations[0].Issues) != 2 {
		t.Fatal("failed message assertions were not recordable")
	}
}

type stepContext struct {
	calls atomic.Int32
	after int32
}

func (c *stepContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *stepContext) Done() <-chan struct{}       { return nil }
func (c *stepContext) Value(any) any               { return nil }
func (c *stepContext) Err() error {
	if c.calls.Add(1) >= c.after {
		return context.Canceled
	}
	return nil
}
func TestCancellationAndInvalidModelsReturnNoPartialEvidence(t *testing.T) {
	lock := relayModel(t)
	s := scenario(direct("a", "First", 0, 2), dispatch("ab", "First", "first", 0), deliver("first", "after_recipient_commit_before_ack"), deliver("first", ""))
	for _, after := range []int32{1, 10, 30} {
		if result, err := Run(&stepContext{after: after}, lock, s); !errors.Is(err, context.Canceled) || result.EngineVersion != "" || result.Steps != nil {
			t.Fatalf("cancelled delivery exposed partial evidence: %+v %v", result, err)
		}
	}
	lock.Digest = strings.Repeat("0", 64)
	if result, err := Run(context.Background(), lock, s); err == nil || result.EngineVersion != "" {
		t.Fatal("invalid model executed")
	}
}

func TestMessageCapacityRetainsOriginalEnvelopes(t *testing.T) {
	base := relayMachine(t)
	sender := base
	sender.Machine.ID = "test.sender"
	sender.Machine.Commands = sender.Machine.Commands[:1]
	sender = machineLock(t, sender.Machine)
	receiver := base.Machine
	receiver.ID, receiver.Commands = "test.receiver", []model.StateCommand{}
	for i := 0; i < 8; i++ {
		command := base.Machine.Commands[0]
		command.Name = fmt.Sprintf("add%d", i)
		receiver.Commands = append(receiver.Commands, command)
	}
	other := machineLock(t, receiver)
	m := model.DeliveryModel{SchemaVersion: 1, ID: "test.capacity", Version: "1.0.0", Name: "Message capacity", Author: "Tests", License: "MIT", Machines: []model.StateMachineLock{sender, other}, Instances: []model.DeliveryInstance{{ID: "a", Name: "Sender", Machine: model.DeliveryMachineRef{MachineID: sender.Machine.ID, Version: sender.Machine.Version, Digest: sender.Digest}}, {ID: "b", Name: "Receiver", Machine: model.DeliveryMachineRef{MachineID: other.Machine.ID, Version: other.Machine.Version, Digest: other.Digest}}}, Routes: []model.DeliveryRoute{}}
	for i := 0; i < 8; i++ {
		m.Routes = append(m.Routes, model.DeliveryRoute{ID: fmt.Sprintf("route%d", i), Name: "Route", SourceInstanceID: "a", Effect: "Notice", TargetInstanceID: "b", Command: fmt.Sprintf("add%d", i), Inputs: map[string]string{"amount": "amount", "permit": "permit"}})
	}
	lock := modelLock(t, m)
	s := scenario()
	s.Initial = s.Initial[:2]
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("request%d", i)
		s.Steps = append(s.Steps, direct("a", id, int64(i), 1))
		for j := 0; j < 8; j++ {
			s.Steps = append(s.Steps, dispatch(fmt.Sprintf("route%d", j), id, fmt.Sprintf("message%d", i*8+j), 0))
		}
	}
	if result := runOK(t, lock, s); len(result.Messages) != 32 || len(result.Instances[0].Effects) != 4 || result.Operations != 8 {
		t.Fatal("dispatch auto-delivered or lost exact capacity")
	}
	s.Steps = append(s.Steps, direct("a", "last", 4, 1), dispatch("route0", "last", "overflow", 0))
	if result, err := Run(context.Background(), lock, s); !errors.Is(err, ErrLimit) || result.EngineVersion != "" || !strings.Contains(err.Error(), "message capacity") {
		t.Fatalf("message capacity did not abort run: %+v %v", result, err)
	}
}
