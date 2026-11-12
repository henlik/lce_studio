package state

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"software-studio/internal/model"
)

func TestSessionPreservesKernelAndOwnsDefinitions(t *testing.T) {
	ctx := context.Background()
	lock := counterMachine(t)
	scenario := scenario(command("one", "first", 0, 2, true, true, ""), model.StateStep{Kind: "restart", InstanceID: "one"}, command("one", "first", 0, 2, true, true, "before_commit"))
	scenario.Instances[0].Initial["ticks"] = integer(20)
	want := runOK(t, lock, scenario)
	host, err := NewSession(ctx, lock, scenario.Instances, &Budget{})
	if err != nil {
		t.Fatal(err)
	}
	// The session must not consult caller-owned pointers after initialization.
	delete(lock.Machine.Commands[0].Writes, "value")
	*lock.Machine.State[0].Initial.Integer = 999
	*scenario.Instances[0].Initial["ticks"].Integer = 99
	for i, step := range scenario.Steps {
		called := false
		got, err := host.Apply(ctx, i, step, func(p Projection) error {
			called = true
			if p.StepBytes <= 0 || p.InstanceBytes <= 0 {
				t.Fatal("missing projected evidence sizes")
			}
			return nil
		})
		if err != nil || !called || !reflect.DeepEqual(got, want.Steps[i]) {
			t.Fatalf("session diverged from legacy step%d: %+v %v", i, got, err)
		}
		*got.After["value"].Integer = 777
		if got.Receipt != nil {
			*got.Receipt.Outputs["result"].Integer = 888
		}
	}
	got, err := host.Snapshot(ctx)
	if err != nil || !reflect.DeepEqual(got, want.Instances) {
		t.Fatalf("session state aliased caller/result: %+v %v", got, err)
	}
	effect, found := host.Effect("one", "first", "Changed")
	if !found {
		t.Fatal("committed effect missing")
	}
	*effect.Payload["value"].Integer = 777
	if again, _ := host.Effect("one", "first", "Changed"); *again.Payload["value"].Integer != 12 {
		t.Fatal("effect lookup exposes internal state")
	}
}

func TestSessionAdmissionPrecedesCommitAndSharedWorkDoesNotReset(t *testing.T) {
	ctx := context.Background()
	lock := counterMachine(t)
	host, err := NewSession(ctx, lock, scenario().Instances, &Budget{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Apply(ctx, 0, command("one", "first", 0, 2, true, true, ""), func(Projection) error { return ErrLimit }); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if host.engine.instances["one"].revision != 0 || len(host.engine.instances["one"].receipts) != 0 {
		t.Fatal("rejected admission published a commit")
	}
	for _, failure := range []bool{false, true} {
		lock = heavyMachine(t, failure)
		budget := &Budget{}
		hosts := make([]*Session, 2)
		for i := range hosts {
			hosts[i], err = NewSession(ctx, lock, []model.StateInstance{{ID: fmt.Sprintf("owner%d", i), Name: "Owner", Initial: emptyValues()}}, budget)
			if err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < 17; i++ {
			owner, rev := i%2, int64(i/2)
			if failure {
				rev = 0
			}
			step := model.StateStep{Kind: "command", InstanceID: fmt.Sprintf("owner%d", owner), CommandID: fmt.Sprintf("request%d", i), Command: "change", ExpectedRevision: revision(rev), Inputs: emptyValues()}
			_, err := hosts[owner].Apply(ctx, i, step, nil)
			if i < 16 && err != nil {
				t.Fatal(err)
			}
			if i == 16 && !errors.Is(err, ErrLimit) {
				t.Fatalf("failure=%v global work reset between hosts: %v", failure, err)
			}
		}
	}
}
