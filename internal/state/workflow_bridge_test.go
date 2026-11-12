package state

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestWorkflowProjectionOffersOnlyIndependentScalarMetadata(t *testing.T) {
	ctx := context.Background()
	lock := counterMachine(t)
	session, err := NewSession(ctx, lock, scenario().Instances, &Budget{})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := session.Revision("one")
	if err != nil || revision != 0 {
		t.Fatalf("unexpected initial revision %d %v", revision, err)
	}
	var projected Projection
	step, err := session.Apply(ctx, 0, command("one", "first", 0, 2, true, true, ""), func(p Projection) error {
		projected = p
		if !p.ReceiptPresent || !p.ReceiptAccepted || p.OutputBytes["result"] == 0 {
			t.Fatalf("missing receipt metadata: %+v", p)
		}
		// This mutable map contains byte counts only; changing it cannot alter
		// the proposed receipt, accepted decision, or any owned typed value.
		p.OutputBytes["result"] = -1
		p.BooleanOutputs["invented"] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if *step.Receipt.Outputs["result"].Integer != 12 || !step.Receipt.Accepted {
		t.Fatal("metadata mutation rewrote proposed owner data")
	}
	raw, _ := json.Marshal(step)
	if projected.StepBytes != len(raw) {
		t.Fatalf("projected step bytes differ: %d != %d", projected.StepBytes, len(raw))
	}
	revision, err = session.Revision("one")
	if err != nil || revision != 1 {
		t.Fatalf("unexpected committed revision %d %v", revision, err)
	}
	if _, err = session.Revision("missing"); err == nil {
		t.Fatal("unknown owner revision accepted")
	}
}

func TestWorkflowWorkReservationRejectsOverflowAndCannotReset(t *testing.T) {
	budget := &Budget{}
	for _, value := range []int{-1, math.MaxInt} {
		if err := budget.ReserveWork(value, 0); !errors.Is(err, ErrLimit) {
			t.Fatalf("invalid reservation accepted: %d %v", value, err)
		}
	}
	if err := budget.ReserveWork(MaxOperations, MaxInvocations); err != nil {
		t.Fatal(err)
	}
	if err := budget.ReserveWork(0, 0); err != nil {
		t.Fatal(err)
	}
	if err := budget.ReserveWork(1, 0); !errors.Is(err, ErrLimit) {
		t.Fatal("operation budget was reset")
	}
	if err := budget.ReserveWork(0, 1); !errors.Is(err, ErrLimit) {
		t.Fatal("invocation budget was reset")
	}
}
