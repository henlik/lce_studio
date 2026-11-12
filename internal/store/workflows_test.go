package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/testutil"
	"software-studio/internal/workflow"
)

// Portable snapshots preserve canonical JSON. Empty optional Go collections may
// be omitted and decode as nil, which does not change the stored artifact.
func workflowJSONEqual(a, b any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func catalogWorkflowExample(t *testing.T) model.WorkflowExample {
	t.Helper()
	examples := model.ExampleWorkflowScenarios()
	if len(examples) == 0 {
		t.Fatal("no workflow examples")
	}
	if issues := model.ValidateWorkflowLock(examples[0].Workflow); len(issues) > 0 {
		t.Fatal(issues)
	}
	return examples[0]
}

func TestWorkflowCatalogVersioningAndReopen(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.WorkflowPackages(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty catalog: %#v %v", records, err)
	}
	definition := catalogWorkflowExample(t).Workflow.Workflow
	want, created, err := db.InstallWorkflowPackage(ctx, definition)
	if err != nil || !created || want.Digest == "" || !workflowJSONEqual(want.Workflow, definition) {
		t.Fatalf("install: %#v %v %v", want, created, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, want.InstalledAt); err != nil {
		t.Fatal(err)
	}
	again, created, err := db.InstallWorkflowPackage(ctx, definition)
	if err != nil || created || !workflowJSONEqual(again, want) {
		t.Fatalf("reinstall: %#v %v %v", again, created, err)
	}
	definition.Description = "Changed metadata"
	if _, _, err := db.InstallWorkflowPackage(ctx, definition); !errors.Is(err, ErrWorkflowPackageConflict) {
		t.Fatalf("immutable conflict: %v", err)
	}
	definition.Version = "2.0.0"
	if _, created, err := db.InstallWorkflowPackage(ctx, definition); err != nil || !created {
		t.Fatalf("new version: %v %v", created, err)
	}
	definition.ID = "a.first"
	if _, _, err := db.InstallWorkflowPackage(ctx, definition); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	records, err := db.WorkflowPackages(ctx)
	if err != nil || len(records) != 3 || records[0].Workflow.ID != "a.first" || !workflowJSONEqual(records[1], want) || records[2].Workflow.Version != "2.0.0" {
		t.Fatalf("reopen/order: %#v %v", records, err)
	}
	exact, err := db.WorkflowPackage(ctx, want.Workflow.ID, want.Workflow.Version, want.Digest)
	if err != nil || !workflowJSONEqual(exact, want) {
		t.Fatalf("exact read: %#v %v", exact, err)
	}
	if _, err := db.WorkflowPackage(ctx, want.Workflow.ID, want.Workflow.Version, "wrong"); !errors.Is(err, ErrNotFound) {
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
	if records, err := db.DeliveryModels(ctx); err != nil || len(records) != 0 {
		t.Fatalf("definition installed delivery dependencies: %#v %v", records, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := db.InstallWorkflowPackage(cancelled, definition); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled install: %v", err)
	}
}

func TestConcurrentWorkflowInstallationIsImmutable(t *testing.T) {
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
	definition := catalogWorkflowExample(t).Workflow.Workflow
	type result struct {
		record  WorkflowPackageRecord
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
				record, created, err := stores[i%len(stores)].InstallWorkflowPackage(context.Background(), m)
				results <- result{record, created, err}
			}(i)
		}
		close(start)
		created, conflicts := 0, 0
		var first *WorkflowPackageRecord
		for i := 0; i < workers; i++ {
			r := <-results
			if errors.Is(r.err, ErrWorkflowPackageConflict) {
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
			} else if !workflowJSONEqual(*first, r.record) {
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

func TestWorkflowCatalogCapacityIsAtomicAndIndependent(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	definition := catalogWorkflowExample(t).Workflow.Workflow
	for i := 0; i < MaxWorkflowPackageVersions-1; i++ {
		m := definition
		m.ID = fmt.Sprintf("test.definition-%03d", i)
		if _, _, err := db.InstallWorkflowPackage(ctx, m); err != nil {
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
			_, _, err := db.InstallWorkflowPackage(ctx, m)
			results <- err
		}(i)
	}
	close(start)
	created, full := 0, 0
	for i := 0; i < workers; i++ {
		err := <-results
		if err == nil {
			created++
		} else if errors.Is(err, ErrWorkflowCatalogFull) {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if created != 1 || full != workers-1 {
		t.Fatalf("capacity race: %d created %d full", created, full)
	}
	definition.ID = "test.definition-000"
	if _, created, err := db.InstallWorkflowPackage(ctx, definition); err != nil || created {
		t.Fatalf("reinstall at capacity: %v %v", created, err)
	}
	definition.Description = "Changed"
	if _, _, err := db.InstallWorkflowPackage(ctx, definition); !errors.Is(err, ErrWorkflowPackageConflict) {
		t.Fatalf("conflict at capacity: %v", err)
	}
	if _, created, err := db.InstallComponent(ctx, catalogPackage()); err != nil || !created {
		t.Fatalf("workflow capacity affected components: %v %v", created, err)
	}
	if _, created, err := db.InstallDataContract(ctx, catalogDataContract()); err != nil || !created {
		t.Fatalf("workflow capacity affected data: %v %v", created, err)
	}
	if _, created, err := db.InstallStateMachine(ctx, catalogStateExample(t).Machine.Machine); err != nil || !created {
		t.Fatalf("workflow capacity affected state: %v %v", created, err)
	}
	if _, created, err := db.InstallDeliveryModel(ctx, catalogDeliveryExample(t).Model.Model); err != nil || !created {
		t.Fatalf("workflow capacity affected delivery: %v %v", created, err)
	}
}

func TestWorkflowRunSnapshotsAndHistory(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.WorkflowRuns(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty history: %#v %v", records, err)
	}
	example := catalogWorkflowExample(t)
	result, err := workflow.Run(ctx, example.Workflow, example.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	first := WorkflowRunRecord{ID: "workflow-000", CreatedAt: "2026-01-01T02:00:00+02:00", Workflow: example.Workflow, Scenario: example.Scenario, Result: result}
	if err := db.SaveWorkflowRun(ctx, first); err != nil {
		t.Fatal(err)
	}
	changed := first
	changed.Scenario.Name = "Replacement"
	if err := db.SaveWorkflowRun(ctx, changed); err == nil {
		t.Fatal("existing workflow evidence was replaced")
	}
	for i := 1; i <= 55; i++ {
		r := first
		r.ID = fmt.Sprintf("workflow-%03d", i)
		r.CreatedAt = time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		r.Result.Verified = true
		r.Result.Passed = i%2 == 0
		if err := db.SaveWorkflowRun(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	bad := first
	bad.ID = "bad-time"
	bad.CreatedAt = "yesterday"
	if err := db.SaveWorkflowRun(ctx, bad); err == nil {
		t.Fatal("invalid creation time accepted")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	history, err := db.WorkflowRuns(ctx)
	if err != nil || len(history) != 50 || history[0].ID != "workflow-055" || history[49].ID != "workflow-006" || history[0].WorkflowID != example.Workflow.Workflow.ID || history[0].WorkflowVersion != example.Workflow.Workflow.Version || history[0].Status != result.Status || history[0].Outcome != result.Outcome || history[0].EventCount != len(result.Events) || history[0].TaskCount != len(result.Tasks) || !history[0].Verified || history[0].Passed {
		t.Fatalf("history: %#v %v", history, err)
	}
	detail, err := db.WorkflowRun(ctx, first.ID)
	if err != nil || !workflowJSONEqual(detail, first) {
		t.Fatalf("original evidence changed or trimmed: %#v %v", detail, err)
	}
	detail.Workflow.Workflow.Nodes[0].Name = "changed"
	if len(detail.Result.Tasks) == 0 {
		t.Fatal("example has no task evidence")
	}
	detail.Result.Tasks[0].Title = "changed"
	again, err := db.WorkflowRun(ctx, first.ID)
	if err != nil || !workflowJSONEqual(again, first) {
		t.Fatalf("read mutated snapshot: %#v %v", again, err)
	}
	if _, err := db.WorkflowRun(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing evidence: %v", err)
	}
	if records, err := db.WorkflowPackages(ctx); err != nil || len(records) != 0 {
		t.Fatalf("run installed a workflow package: %#v %v", records, err)
	}
}
