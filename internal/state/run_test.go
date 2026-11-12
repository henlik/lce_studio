package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
func textValue(v string) model.TypedValue     { return model.TypedValue{Type: "string", String: &v} }
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
func machineLock(t testing.TB, machine model.StateMachine) model.StateMachineLock {
	t.Helper()
	digest, err := model.StateMachineDigest(machine)
	if err != nil {
		t.Fatal(err, model.ValidateStateMachine(machine))
	}
	return model.StateMachineLock{Machine: machine, Digest: digest}
}
func counterMachine(t testing.TB) model.StateMachineLock {
	t.Helper()
	p := definition("test.counter-decision", model.Behavior{
		Inputs:  []model.BehaviorPort{{Name: "value", Type: "integer"}, {Name: "ticks", Type: "integer"}, {Name: "delta", Type: "integer"}, {Name: "permit", Type: "boolean"}, {Name: "notify", Type: "boolean"}},
		Outputs: []model.BehaviorPort{{Name: "accepted", Type: "boolean"}, {Name: "next", Type: "integer"}, {Name: "ticks", Type: "integer"}, {Name: "emit", Type: "boolean"}, {Name: "delta", Type: "integer"}},
		Nodes: []model.BehaviorNode{
			{ID: "next", Op: "add", Inputs: map[string]model.BehaviorBinding{"left": input("value"), "right": input("delta")}},
			{ID: "ticks", Op: "add", Inputs: map[string]model.BehaviorBinding{"left": input("ticks"), "right": literal(integer(1))}},
		}, Bindings: map[string]model.BehaviorBinding{"accepted": input("permit"), "next": output("next"), "ticks": output("ticks"), "emit": input("notify"), "delta": input("delta")},
	})
	m := model.StateMachine{SchemaVersion: 1, ID: "test.counter", Version: "1.0.0", Name: "Owned counter", Author: "Tests", License: "MIT", DataContracts: []model.DataContractLock{},
		State:   []model.StateSlot{{Name: "value", Type: "integer", Durability: "durable", Initial: integer(10)}, {Name: "ticks", Type: "integer", Durability: "volatile", Initial: integer(5)}, {Name: "label", Type: "string", Durability: "durable", Initial: textValue("initial")}},
		Effects: []model.StateEffect{{Name: "Changed", Payload: []model.BehaviorPort{{Name: "amount", Type: "integer"}, {Name: "value", Type: "integer"}}}},
		Commands: []model.StateCommand{{Name: "change", Inputs: []model.BehaviorPort{{Name: "delta", Type: "integer"}, {Name: "permit", Type: "boolean"}, {Name: "notify", Type: "boolean"}}, Outputs: []model.BehaviorPort{{Name: "result", Type: "integer"}}, Decision: bundle(t, p),
			Bindings: map[string]model.StateBinding{"value": {State: "value"}, "ticks": {State: "ticks"}, "delta": {Input: "delta"}, "permit": {Input: "permit"}, "notify": {Input: "notify"}}, AcceptedOutput: "accepted", Writes: map[string]string{"value": "next", "ticks": "ticks"}, Results: map[string]string{"result": "next"}, Effects: []model.StateEffectBinding{{Effect: "Changed", WhenOutput: "emit", Payload: map[string]string{"amount": "delta", "value": "next"}}}}},
	}
	return machineLock(t, m)
}
func command(instance, id string, rev, delta int64, permit, notify bool, fault string) model.StateStep {
	return model.StateStep{Kind: "command", InstanceID: instance, CommandID: id, Command: "change", ExpectedRevision: revision(rev), Inputs: map[string]model.TypedValue{"delta": integer(delta), "permit": boolean(permit), "notify": boolean(notify)}, Fault: fault}
}
func scenario(steps ...model.StateStep) model.StateScenario {
	return model.StateScenario{SchemaVersion: 1, Name: "Counter experiment", Instances: []model.StateInstance{{ID: "one", Name: "First owner", Initial: emptyValues()}}, Steps: steps, Expected: []model.StateExpectation{}}
}
func runOK(t testing.TB, lock model.StateMachineLock, scenario model.StateScenario) Result {
	t.Helper()
	r, err := Run(context.Background(), lock, scenario)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > MaxResultBytes {
		t.Fatalf("invalid public evidence: %d bytes, %v", len(encoded), err)
	}
	if r.Instances == nil || r.Steps == nil || r.Expectations == nil {
		t.Fatal("null result arrays")
	}
	for _, instance := range r.Instances {
		if instance.State == nil || instance.Receipts == nil || instance.Effects == nil {
			t.Fatal("null instance collections")
		}
	}
	for _, step := range r.Steps {
		if step.Before == nil || step.After == nil {
			t.Fatal("null snapshots")
		}
		n, err := stepSize(step)
		actual, _ := json.Marshal(step)
		if err != nil || n != len(actual) {
			t.Fatalf("incremental step accounting %d != %d: %v", n, len(actual), err)
		}
	}
	return r
}

func TestProtocolMatchesIndependentCounterReference(t *testing.T) {
	lock := counterMachine(t)
	s := scenario(
		command("one", "first", 0, 3, true, true, ""),
		command("one", "first", 0, 3, true, true, "before_commit"),
		command("one", "first", 0, 4, true, true, "after_commit_before_reply"),
		command("one", "denied", 0, 2, false, true, ""),
		command("one", "denied", 1, 2, false, true, ""),
		command("one", "denied", 1, 2, false, true, "after_commit_before_reply"),
		command("one", "retried", 1, 2, true, true, "before_commit"),
		command("one", "retried", 1, 2, true, true, "after_commit_before_reply"),
		model.StateStep{Kind: "restart", InstanceID: "one"},
		command("one", "retried", 1, 2, true, true, "before_commit"),
		command("one", "silent", 2, 0, true, false, ""),
		command("one", "first", 0, 3, true, true, ""),
		command("two", "first", 0, 1, true, true, ""),
	)
	s.Instances[0].Initial["ticks"] = integer(20)
	s.Instances = append(s.Instances, model.StateInstance{ID: "two", Name: "Second owner", Initial: emptyValues()})
	s.Expected = []model.StateExpectation{
		{InstanceID: "one", State: map[string]model.TypedValue{"value": integer(15), "ticks": integer(21), "label": textValue("initial")}, Revision: 3, Receipts: 4, Effects: 2},
		{InstanceID: "two", State: map[string]model.TypedValue{"value": integer(11), "ticks": integer(6), "label": textValue("initial")}, Revision: 1, Receipts: 1, Effects: 1},
	}
	r := runOK(t, lock, s)
	want := []string{"committed", "replayed", "id_conflict", "revision_conflict", "denied", "replayed", "rolled_back", "committed", "restarted", "replayed", "committed", "replayed", "committed"}
	for i, status := range want {
		if r.Steps[i].Status != status || r.Steps[i].Index != i {
			t.Fatalf("step %d: %+v", i, r.Steps[i])
		}
	}
	if !r.Verified || !r.Passed || r.Operations != 12 || r.Invocations != 6 {
		t.Fatalf("reference totals/verdict: %+v", r)
	}
	if !r.Steps[6].FaultApplied || r.Steps[6].ReplyLost || !r.Steps[7].FaultApplied || !r.Steps[7].ReplyLost {
		t.Fatal("commit fault semantics")
	}
	for _, i := range []int{1, 2, 5, 9} {
		if r.Steps[i].FaultApplied || r.Steps[i].ReplyLost || r.Steps[i].Decision != nil {
			t.Fatalf("replay/conflict reevaluated or injected fault: %d", i)
		}
	}
	if *r.Steps[6].After["ticks"].Integer != 21 || *r.Steps[8].After["ticks"].Integer != 20 {
		t.Fatal("rollback reset volatile state, or restart ignored override")
	}
	if *r.Steps[11].Receipt.Outputs["result"].Integer != 13 || r.Steps[11].Receipt.Revision != 1 || *r.Steps[11].After["value"].Integer != 15 || r.Steps[11].AfterRevision != 3 {
		t.Fatal("replay replaced current state with history")
	}
	if r.Steps[4].Receipt.Accepted || r.Steps[4].AfterRevision != 1 || *r.Steps[4].After["value"].Integer != 13 || *r.Steps[4].Receipt.Outputs["result"].Integer != 15 {
		t.Fatal("denial failed to discard writes while retaining its reply")
	}
	request := sha256.Sum256([]byte(`{"command":"change","expectedRevision":0,"inputs":{"delta":{"type":"integer","integer":3},"notify":{"type":"boolean","boolean":true},"permit":{"type":"boolean","boolean":true}}}`))
	effect := sha256.Sum256([]byte(`["one","first","Changed"]`))
	if r.Instances[0].Receipts[0].RequestHash != hex.EncodeToString(request[:]) || r.Instances[0].Effects[0].ID != hex.EncodeToString(effect[:]) {
		t.Fatal("hash protocol changed")
	}
	if r.Instances[0].Effects[0].ID == r.Instances[1].Effects[0].ID {
		t.Fatal("effect IDs omitted instance scope")
	}
	if !reflect.DeepEqual(r, runOK(t, lock, s)) {
		t.Fatal("fresh independent run was nondeterministic")
	}
}

func TestOrdinaryErrorsRollbackAndAllowRetry(t *testing.T) {
	lock := counterMachine(t)
	s := scenario(command("one", "retry", 0, model.MaxBehaviorInteger, true, true, "before_commit"), command("one", "retry", 0, 1, true, true, ""))
	r := runOK(t, lock, s)
	if r.Steps[0].Status != "evaluation_error" || r.Steps[0].Message == "" || r.Steps[0].Decision != nil || r.Steps[0].Receipt != nil || r.Steps[0].FaultApplied || r.Steps[1].Status != "committed" {
		t.Fatalf("error/retry protocol: %+v", r.Steps)
	}
	if r.Operations != 2 || r.Invocations != 1 || r.Instances[0].Revision != 1 || len(r.Instances[0].Receipts) != 1 {
		t.Fatal("failed decision was counted as completed or committed a receipt")
	}
	if r.Verified || r.Passed {
		t.Fatal("empty expectations imply verification")
	}
	s.Expected = []model.StateExpectation{{InstanceID: "one", State: map[string]model.TypedValue{"value": integer(999), "ticks": integer(6), "label": textValue("initial")}, Revision: 2, Receipts: 2, Effects: 2}}
	r = runOK(t, lock, s)
	if !r.Verified || r.Passed || len(r.Expectations[0].Issues) != 4 {
		t.Fatal("independent mismatches were not recordable")
	}
}

func TestAcceptedNoWritesStillIncrementsRevision(t *testing.T) {
	machine := counterMachine(t).Machine
	machine.Commands[0].Writes = map[string]string{}
	machine.Commands[0].Effects = []model.StateEffectBinding{}
	r := runOK(t, machineLock(t, machine), scenario(command("one", "noop", 0, 0, true, false, "")))
	if r.Instances[0].Revision != 1 || *r.Instances[0].State["ticks"].Integer != 5 || len(r.Instances[0].Effects) != 0 {
		t.Fatal("accepted no-write transaction semantics")
	}
}

func TestReceiptFingerprintIncludesRevisionButExcludesFault(t *testing.T) {
	lock := counterMachine(t)
	r := runOK(t, lock, scenario(command("one", "same", 0, 0, true, false, ""), command("one", "same", 1, 0, true, false, ""), command("one", "same", 0, 0, true, false, "after_commit_before_reply")))
	if r.Steps[1].Status != "id_conflict" || r.Steps[2].Status != "replayed" || r.Steps[2].ReplyLost || r.Steps[2].FaultApplied {
		t.Fatal("request identity or replay ordering changed")
	}
}

func TestEffectOrderCapacityAndSuppression(t *testing.T) {
	m := counterMachine(t).Machine
	m.Effects, m.Commands[0].Effects = []model.StateEffect{}, []model.StateEffectBinding{}
	for _, name := range []string{"Zulu", "Alpha", "Bravo", "Delta", "Echo", "Foxtrot", "Golf", "Hotel"} {
		m.Effects = append(m.Effects, model.StateEffect{Name: name, Payload: []model.BehaviorPort{{Name: "value", Type: "integer"}}})
		m.Commands[0].Effects = append(m.Commands[0].Effects, model.StateEffectBinding{Effect: name, Payload: map[string]string{"value": "next"}})
	}
	lock := machineLock(t, m)
	s := scenario()
	for i := 0; i < 16; i++ {
		s.Steps = append(s.Steps, command("one", fmt.Sprintf("request%d", i), int64(i), 0, true, false, ""))
	}
	r := runOK(t, lock, s)
	if len(r.Instances[0].Effects) != 128 {
		t.Fatal("exact effect capacity failed")
	}
	for i, effect := range r.Instances[0].Effects {
		if effect.Effect != m.Commands[0].Effects[i%8].Effect {
			t.Fatal("declared effect order was sorted or lost")
		}
	}
	s.Steps = append(s.Steps, command("one", "overflow", 16, 0, true, true, ""))
	if r, err := Run(context.Background(), lock, s); !errors.Is(err, ErrLimit) || r.EngineVersion != "" {
		t.Fatalf("effect capacity returned partial success: %+v %v", r, err)
	}
	// Rollback and denial never consume effect capacity, even with eight bindings.
	s.Steps[len(s.Steps)-1] = command("one", "denied", 16, 0, false, true, "")
	if r := runOK(t, lock, s); len(r.Instances[0].Effects) != 128 || len(r.Instances[0].Receipts) != 17 {
		t.Fatal("denial consumed effects")
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

func TestInvalidInputAndCancellationNeverReturnPartialEvidence(t *testing.T) {
	lock := counterMachine(t)
	s := scenario()
	for i := 0; i < 20; i++ {
		s.Steps = append(s.Steps, command("one", fmt.Sprintf("request%d", i), int64(i), 1, true, false, ""))
	}
	for _, after := range []int32{1, 10, 100} {
		if r, err := Run(&stepContext{after: after}, lock, s); !errors.Is(err, context.Canceled) || r.EngineVersion != "" || r.Steps != nil {
			t.Fatalf("cancellation exposed partial result: %+v %v", r, err)
		}
	}
	s.Steps[0].ExpectedRevision = nil
	if r, err := Run(context.Background(), lock, s); err == nil || r.EngineVersion != "" {
		t.Fatal("invalid scenario executed")
	}
	lock.Digest = strings.Repeat("0", 64)
	if r, err := Run(context.Background(), lock, s); err == nil || r.EngineVersion != "" {
		t.Fatal("invalid lock executed")
	}
}
