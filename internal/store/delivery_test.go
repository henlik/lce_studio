package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"software-studio/internal/delivery"
	"software-studio/internal/model"
	"software-studio/internal/testutil"
)

func catalogDeliveryExample(t *testing.T) model.DeliveryExample {
	t.Helper()
	examples := model.ExampleDeliveryScenarios()
	if len(examples) == 0 {
		t.Fatal("no delivery examples")
	}
	if issues := model.ValidateDeliveryModelLock(examples[0].Model); len(issues) > 0 {
		t.Fatal(issues)
	}
	return examples[0]
}

func TestDeliveryCatalogVersioningAndReopen(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.DeliveryModels(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty catalog: %#v %v", records, err)
	}
	definition := catalogDeliveryExample(t).Model.Model
	want, created, err := db.InstallDeliveryModel(ctx, definition)
	if err != nil || !created || want.Digest == "" || !reflect.DeepEqual(want.Model, definition) {
		t.Fatalf("install: %#v %v %v", want, created, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, want.InstalledAt); err != nil {
		t.Fatal(err)
	}
	again, created, err := db.InstallDeliveryModel(ctx, definition)
	if err != nil || created || !reflect.DeepEqual(again, want) {
		t.Fatalf("reinstall: %#v %v %v", again, created, err)
	}
	definition.Description = "Changed metadata"
	if _, _, err := db.InstallDeliveryModel(ctx, definition); !errors.Is(err, ErrDeliveryModelConflict) {
		t.Fatalf("immutable conflict: %v", err)
	}
	definition.Version = "2.0.0"
	if _, created, err := db.InstallDeliveryModel(ctx, definition); err != nil || !created {
		t.Fatalf("new version: %v %v", created, err)
	}
	definition.ID = "a.first"
	if _, _, err := db.InstallDeliveryModel(ctx, definition); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	records, err := db.DeliveryModels(ctx)
	if err != nil || len(records) != 3 || records[0].Model.ID != "a.first" || !reflect.DeepEqual(records[1], want) || records[2].Model.Version != "2.0.0" {
		t.Fatalf("reopen/order: %#v %v", records, err)
	}
	exact, err := db.DeliveryModel(ctx, want.Model.ID, want.Model.Version, want.Digest)
	if err != nil || !reflect.DeepEqual(exact, want) {
		t.Fatalf("exact read: %#v %v", exact, err)
	}
	if _, err := db.DeliveryModel(ctx, want.Model.ID, want.Model.Version, "wrong"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong digest: %v", err)
	}
	if records, err := db.Components(ctx); err != nil || len(records) != 0 {
		t.Fatalf("definition installed component dependencies: %#v %v", records, err)
	}
	if records, err := db.DataContracts(ctx); err != nil || len(records) != 0 {
		t.Fatalf("definition installed data dependencies: %#v %v", records, err)
	}
	if records, err := db.StateMachines(ctx); err != nil || len(records) != 0 {
		t.Fatalf("definition installed state dependencies: %#v %v", records, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := db.InstallDeliveryModel(cancelled, definition); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled install: %v", err)
	}
}

func TestConcurrentDeliveryInstallationIsImmutable(t *testing.T) {
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
	definition := catalogDeliveryExample(t).Model.Model
	type result struct {
		record  DeliveryModelRecord
		created bool
		err     error
	}
	const workers = 12
	for _, mixed := range []bool{false, true} {
		results := make(chan result, workers)
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			go func(i int) {
				m := definition
				if mixed {
					m.ID = "test.competing"
					m.Description = fmt.Sprintf("Variant %d", i%2)
				}
				<-start
				record, created, err := stores[i%len(stores)].InstallDeliveryModel(context.Background(), m)
				results <- result{record, created, err}
			}(i)
		}
		close(start)
		created, conflicts := 0, 0
		var first *DeliveryModelRecord
		for i := 0; i < workers; i++ {
			r := <-results
			if errors.Is(r.err, ErrDeliveryModelConflict) {
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

func TestDeliveryCatalogCapacityIsAtomicAndIndependent(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	definition := catalogDeliveryExample(t).Model.Model
	for i := 0; i < MaxDeliveryModelVersions-1; i++ {
		m := definition
		m.ID = fmt.Sprintf("test.definition-%03d", i)
		if _, _, err := db.InstallDeliveryModel(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	const workers = 8
	results := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func(i int) {
			m := definition
			m.ID = fmt.Sprintf("test.race-%d", i)
			<-start
			_, _, err := db.InstallDeliveryModel(ctx, m)
			results <- err
		}(i)
	}
	close(start)
	created, full := 0, 0
	for i := 0; i < workers; i++ {
		err := <-results
		if err == nil {
			created++
		} else if errors.Is(err, ErrDeliveryCatalogFull) {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if created != 1 || full != workers-1 {
		t.Fatalf("capacity race: %d created %d full", created, full)
	}
	definition.ID = "test.definition-000"
	if _, created, err := db.InstallDeliveryModel(ctx, definition); err != nil || created {
		t.Fatalf("reinstall at capacity: %v %v", created, err)
	}
	definition.Description = "Changed"
	if _, _, err := db.InstallDeliveryModel(ctx, definition); !errors.Is(err, ErrDeliveryModelConflict) {
		t.Fatalf("conflict at capacity: %v", err)
	}
	if _, created, err := db.InstallComponent(ctx, catalogPackage()); err != nil || !created {
		t.Fatalf("delivery capacity affected components: %v %v", created, err)
	}
	if _, created, err := db.InstallDataContract(ctx, catalogDataContract()); err != nil || !created {
		t.Fatalf("delivery capacity affected data: %v %v", created, err)
	}
	if _, created, err := db.InstallStateMachine(ctx, catalogStateExample(t).Machine.Machine); err != nil || !created {
		t.Fatalf("delivery capacity affected state: %v %v", created, err)
	}
}

func TestDeliveryRunSnapshotsAndHistory(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.DeliveryRuns(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty history: %#v %v", records, err)
	}
	example := catalogDeliveryExample(t)
	result, err := delivery.Run(ctx, example.Model, example.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	first := DeliveryRunRecord{ID: "delivery-000", CreatedAt: "2026-01-01T02:00:00+02:00", Model: example.Model, Scenario: example.Scenario, Result: result}
	if err := db.SaveDeliveryRun(ctx, first); err != nil {
		t.Fatal(err)
	}
	changed := first
	changed.Scenario.Name = "Replacement"
	if err := db.SaveDeliveryRun(ctx, changed); err == nil {
		t.Fatal("existing delivery evidence was replaced")
	}
	for i := 1; i <= 55; i++ {
		r := first
		r.ID = fmt.Sprintf("delivery-%03d", i)
		r.CreatedAt = time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		r.Result.Verified = true
		r.Result.Passed = i%2 == 0
		if err := db.SaveDeliveryRun(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	bad := first
	bad.ID = "bad-time"
	bad.CreatedAt = "yesterday"
	if err := db.SaveDeliveryRun(ctx, bad); err == nil {
		t.Fatal("invalid creation time accepted")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	history, err := db.DeliveryRuns(ctx)
	if err != nil || len(history) != 50 || history[0].ID != "delivery-055" || history[49].ID != "delivery-006" || history[0].ModelID != example.Model.Model.ID || history[0].ModelVersion != example.Model.Model.Version || history[0].StepCount != len(result.Steps) || history[0].MessageCount != len(result.Messages) || !history[0].Verified || history[0].Passed {
		t.Fatalf("history: %#v %v", history, err)
	}
	detail, err := db.DeliveryRun(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(detail, first) {
		t.Fatalf("original evidence changed or trimmed: %#v %v", detail, err)
	}
	detail.Model.Model.Machines[0].Machine.State[0].Name = "changed"
	detail.Result.Instances[0].Revision = 999
	if len(detail.Result.Messages) == 0 {
		t.Fatal("example has no stored message evidence")
	}
	detail.Result.Messages[0].Name = "changed"
	if acknowledgement := detail.Result.Messages[0].Acknowledgement; acknowledgement != nil {
		acknowledgement.Revision = 999
	}
	again, err := db.DeliveryRun(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(again, first) {
		t.Fatalf("read mutated snapshot: %#v %v", again, err)
	}
	if _, err := db.DeliveryRun(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing evidence: %v", err)
	}
	if records, err := db.DeliveryModels(ctx); err != nil || len(records) != 0 {
		t.Fatalf("run installed a definition: %#v %v", records, err)
	}
}
