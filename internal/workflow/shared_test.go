package workflow

import (
	"context"
	"errors"
	"software-studio/internal/state"
	"testing"
)

func TestSharedCheckpointCancellationWithoutOwners(t *testing.T) {
	lock, scenario := simpleWorkflow(t)
	session, err := NewSharedSession(context.Background(), lock, scenario.Inputs, map[string]SharedOwner{}, &state.Budget{}, "test_", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for name, call := range map[string]func() error{
		"snapshot": func() error { _, err := session.Snapshot(cancelled); return err },
		"restart":  func() error { _, err := session.Restart(cancelled); return err },
		"event":    func() error { _, err := session.Apply(cancelled, scenario.Events[0]); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
	result, err := session.Snapshot(context.Background())
	if err != nil || result.Status != "waiting" || result.Task.ID != "task1" || len(result.Receipts) != 0 {
		t.Fatal("cancelled operation changed checkpoint", err, result)
	}
}
