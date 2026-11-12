package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/state"
	"software-studio/internal/testutil"
)

func catalogStateExample(t *testing.T) model.StateExample {
	t.Helper()
	examples := model.ExampleStateScenarios()
	if len(examples) == 0 {
		t.Fatal("no state examples")
	}
	if issues := model.ValidateStateMachineLock(examples[0].Machine); len(issues) > 0 {
		t.Fatal(issues)
	}
	return examples[0]
}

func TestStateCatalogVersioningAndReopen(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.StateMachines(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty catalog: %#v %v", records, err)
	}
	machine := catalogStateExample(t).Machine.Machine
	want, created, err := db.InstallStateMachine(ctx, machine)
	if err != nil || !created || want.Digest == "" || !reflect.DeepEqual(want.Machine, machine) {
		t.Fatalf("install: %#v %v %v", want, created, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, want.InstalledAt); err != nil {
		t.Fatal(err)
	}
	again, created, err := db.InstallStateMachine(ctx, machine)
	if err != nil || created || !reflect.DeepEqual(again, want) {
		t.Fatalf("reinstall: %#v %v %v", again, created, err)
	}
	machine.Description = "Changed metadata"
	if _, _, err := db.InstallStateMachine(ctx, machine); !errors.Is(err, ErrStateMachineConflict) {
		t.Fatalf("immutable conflict: %v", err)
	}
	machine.Version = "2.0.0"
	if _, created, err := db.InstallStateMachine(ctx, machine); err != nil || !created {
		t.Fatalf("new version: %v %v", created, err)
	}
	machine.ID = "a.first"
	if _, _, err := db.InstallStateMachine(ctx, machine); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	records, err := db.StateMachines(ctx)
	if err != nil || len(records) != 3 || records[0].Machine.ID != "a.first" || !reflect.DeepEqual(records[1], want) || records[2].Machine.Version != "2.0.0" {
		t.Fatalf("reopen/order: %#v %v", records, err)
	}
	exact, err := db.StateMachine(ctx, want.Machine.ID, want.Machine.Version, want.Digest)
	if err != nil || !reflect.DeepEqual(exact, want) {
		t.Fatalf("exact read: %#v %v", exact, err)
	}
	if _, err := db.StateMachine(ctx, want.Machine.ID, want.Machine.Version, "wrong"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong digest: %v", err)
	}
	if records, err := db.Components(ctx); err != nil || len(records) != 0 {
		t.Fatalf("machine installed component dependencies: %#v %v", records, err)
	}
	if records, err := db.DataContracts(ctx); err != nil || len(records) != 0 {
		t.Fatalf("machine installed data dependencies: %#v %v", records, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := db.InstallStateMachine(cancelled, machine); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled install: %v", err)
	}
}

func TestConcurrentStateInstallationIsImmutable(t *testing.T) {
	databaseURL := testutil.DatabaseURL(t)
	stores := make([]*Store, 2)
	for i := range stores {
		db, err := Open(databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		stores[i] = db
	}
	machine := catalogStateExample(t).Machine.Machine
	type result struct {
		record  StateMachineRecord
		created bool
		err     error
	}
	const workers = 12
	for _, mixed := range []bool{false, true} {
		results := make(chan result, workers)
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			go func(i int) {
				m := machine
				if mixed {
					m.ID = "test.competing"
					m.Description = fmt.Sprintf("Variant %d", i%2)
				}
				<-start
				record, created, err := stores[i%len(stores)].InstallStateMachine(context.Background(), m)
				results <- result{record, created, err}
			}(i)
		}
		close(start)
		created, conflicts := 0, 0
		var first *StateMachineRecord
		for i := 0; i < workers; i++ {
			r := <-results
			if errors.Is(r.err, ErrStateMachineConflict) {
				conflicts++
				continue
			}
			if r.err != nil {
				t.Fatal(r.err)
			}
			if r.created {
				created++
			}
			if first == nil {
				first = &r.record
			} else if !reflect.DeepEqual(*first, r.record) {
				t.Fatal("concurrent successful installs disagree")
			}
		}
		wantConflicts := 0
		if mixed {
			wantConflicts = workers / 2
		}
		if created != 1 || conflicts != wantConflicts {
			t.Fatalf("mixed=%v: %d created %d conflicts", mixed, created, conflicts)
		}
	}
}

func TestStateCatalogCapacityIsAtomicAndIndependent(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	machine := catalogStateExample(t).Machine.Machine
	for i := 0; i < MaxStateMachineVersions-1; i++ {
		m := machine
		m.ID = fmt.Sprintf("test.machine-%03d", i)
		if _, _, err := db.InstallStateMachine(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	const workers = 8
	results := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func(i int) {
			m := machine
			m.ID = fmt.Sprintf("test.race-%d", i)
			<-start
			_, _, err := db.InstallStateMachine(ctx, m)
			results <- err
		}(i)
	}
	close(start)
	created, full := 0, 0
	for i := 0; i < workers; i++ {
		err := <-results
		if err == nil {
			created++
		} else if errors.Is(err, ErrStateCatalogFull) {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if created != 1 || full != workers-1 {
		t.Fatalf("capacity race: %d created %d full", created, full)
	}
	machine.ID = "test.machine-000"
	if _, created, err := db.InstallStateMachine(ctx, machine); err != nil || created {
		t.Fatalf("reinstall at capacity: %v %v", created, err)
	}
	machine.Description = "Changed"
	if _, _, err := db.InstallStateMachine(ctx, machine); !errors.Is(err, ErrStateMachineConflict) {
		t.Fatalf("conflict at capacity: %v", err)
	}
	if _, created, err := db.InstallComponent(ctx, catalogPackage()); err != nil || !created {
		t.Fatalf("state capacity affected components: %v %v", created, err)
	}
	if _, created, err := db.InstallDataContract(ctx, catalogDataContract()); err != nil || !created {
		t.Fatalf("state capacity affected data: %v %v", created, err)
	}
}

func TestStateRunSnapshotsAndHistory(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.StateRuns(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty history: %#v %v", records, err)
	}
	example := catalogStateExample(t)
	result, err := state.Run(ctx, example.Machine, example.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	first := StateRunRecord{ID: "state-000", CreatedAt: "2026-01-01T02:00:00+02:00", Machine: example.Machine, Scenario: example.Scenario, Result: result}
	if err := db.SaveStateRun(ctx, first); err != nil {
		t.Fatal(err)
	}
	changed := first
	changed.Scenario.Name = "Replacement"
	if err := db.SaveStateRun(ctx, changed); err == nil {
		t.Fatal("existing state evidence was replaced")
	}
	for i := 1; i <= 55; i++ {
		r := first
		r.ID = fmt.Sprintf("state-%03d", i)
		r.CreatedAt = time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		r.Result.Verified = true
		r.Result.Passed = i%2 == 0
		if err := db.SaveStateRun(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	bad := first
	bad.ID = "bad-time"
	bad.CreatedAt = "yesterday"
	if err := db.SaveStateRun(ctx, bad); err == nil {
		t.Fatal("invalid creation time accepted")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	history, err := db.StateRuns(ctx)
	if err != nil || len(history) != 50 || history[0].ID != "state-055" || history[49].ID != "state-006" || history[0].MachineID != example.Machine.Machine.ID || history[0].MachineVersion != example.Machine.Machine.Version || history[0].StepCount != len(result.Steps) || !history[0].Verified || history[0].Passed {
		t.Fatalf("history: %#v %v", history, err)
	}
	detail, err := db.StateRun(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(detail, first) {
		t.Fatalf("original evidence changed or trimmed: %#v %v", detail, err)
	}
	detail.Machine.Machine.State[0].Name = "changed"
	detail.Result.Instances[0].Revision = 999
	again, err := db.StateRun(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(again, first) {
		t.Fatalf("read mutated snapshot: %#v %v", again, err)
	}
	if _, err := db.StateRun(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing evidence: %v", err)
	}
	if records, err := db.StateMachines(ctx); err != nil || len(records) != 0 {
		t.Fatalf("run installed a machine: %#v %v", records, err)
	}
}
