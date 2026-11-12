package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/testutil"
)

func catalogDataContract() model.DataContract {
	return model.DataContract{
		SchemaVersion: 1, ID: "test.people", Version: "1.0.0", Name: "People", Description: "Closed person records", Author: "Test engineer", License: "MIT",
		Records:  []model.DataRecord{{Name: "person", Fields: []model.DataField{{Name: "id", Type: "integer"}, {Name: "display", Type: "string"}}}},
		Entities: []model.DataEntity{{Name: "people", Record: "person", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}}}, Relationships: []model.DataRelationship{},
	}
}

func TestDataCatalogVersioningAndReopen(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.DataContracts(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty catalog: %#v %v", records, err)
	}
	c := catalogDataContract()
	if issues := model.ValidateDataContract(c); len(issues) != 0 {
		t.Fatal(issues)
	}
	want, created, err := db.InstallDataContract(ctx, c)
	if err != nil || !created || want.Digest == "" || !reflect.DeepEqual(want.Contract, c) {
		t.Fatalf("install: %#v %v %v", want, created, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, want.InstalledAt); err != nil {
		t.Fatal(err)
	}
	again, created, err := db.InstallDataContract(ctx, c)
	if err != nil || created || !reflect.DeepEqual(again, want) {
		t.Fatalf("reinstall: %#v %v %v", again, created, err)
	}
	c.Description = "Different content"
	if _, _, err := db.InstallDataContract(ctx, c); !errors.Is(err, ErrDataContractConflict) {
		t.Fatalf("version conflict: %v", err)
	}
	c.Version = "2.0.0"
	if _, created, err := db.InstallDataContract(ctx, c); err != nil || !created {
		t.Fatalf("new version: %v %v", created, err)
	}
	c.ID = "test.alpha"
	if _, _, err := db.InstallDataContract(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	records, err := db.DataContracts(ctx)
	if err != nil || len(records) != 3 || records[0].Contract.ID != "test.alpha" || records[1].Contract.Version != "1.0.0" || !reflect.DeepEqual(records[1], want) {
		t.Fatalf("reopen: %#v %v", records, err)
	}
	exact, err := db.DataContract(ctx, want.Contract.ID, want.Contract.Version, want.Digest)
	if err != nil || !reflect.DeepEqual(exact, want) {
		t.Fatalf("exact retrieval: %#v %v", exact, err)
	}
	if _, err := db.DataContract(ctx, want.Contract.ID, want.Contract.Version, "wrong"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong digest: %v", err)
	}
	if records, err := db.Components(ctx); err != nil || len(records) != 0 {
		t.Fatalf("data installed components: %#v %v", records, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := db.InstallDataContract(cancelled, c); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled install: %v", err)
	}
}

func TestConcurrentDataInstallationIsImmutable(t *testing.T) {
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
	type result struct {
		record  DataContractRecord
		created bool
		err     error
	}
	const workers = 12
	for _, mixed := range []bool{false, true} {
		results := make(chan result, workers)
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			go func(i int) {
				c := catalogDataContract()
				if mixed {
					c.ID = "test.competing"
					c.Description = fmt.Sprintf("Variant %d", i%2)
				}
				<-start
				r, created, err := stores[i%len(stores)].InstallDataContract(context.Background(), c)
				results <- result{r, created, err}
			}(i)
		}
		close(start)
		created, conflicts := 0, 0
		var first *DataContractRecord
		for i := 0; i < workers; i++ {
			r := <-results
			if errors.Is(r.err, ErrDataContractConflict) {
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
				t.Fatalf("concurrent records disagree: %#v %#v", *first, r.record)
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

func TestDataCatalogCapacityIsIndependentAndAtomic(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	for i := 0; i < MaxDataContractVersions-1; i++ {
		c := catalogDataContract()
		c.ID = fmt.Sprintf("test.people-%03d", i)
		if _, _, err := db.InstallDataContract(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	const workers = 8
	results := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func(i int) {
			c := catalogDataContract()
			c.ID = fmt.Sprintf("test.race-%d", i)
			<-start
			_, _, err := db.InstallDataContract(ctx, c)
			results <- err
		}(i)
	}
	close(start)
	created, full := 0, 0
	for i := 0; i < workers; i++ {
		err := <-results
		if err == nil {
			created++
		} else if errors.Is(err, ErrDataCatalogFull) {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if created != 1 || full != workers-1 {
		t.Fatalf("capacity race: %d created %d full", created, full)
	}
	c := catalogDataContract()
	c.ID = "test.people-000"
	if _, created, err := db.InstallDataContract(ctx, c); err != nil || created {
		t.Fatalf("reinstall at capacity: %v %v", created, err)
	}
	c.Description = "Different"
	if _, _, err := db.InstallDataContract(ctx, c); !errors.Is(err, ErrDataContractConflict) {
		t.Fatalf("conflict at capacity: %v", err)
	}
	if _, created, err := db.InstallComponent(ctx, catalogPackage()); err != nil || !created {
		t.Fatalf("data capacity affected component catalog: %v %v", created, err)
	}
}

func TestDataValidationSnapshotsAndHistory(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.DataValidations(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty history: %#v %v", records, err)
	}
	c := catalogDataContract()
	digest, err := model.DataContractDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	first := DataValidationRecord{ID: "validation-000", CreatedAt: "2026-01-01T02:00:00+02:00", Contract: model.DataContractLock{Contract: c, Digest: digest}, Fixture: model.DataFixture{Name: "Snapshot", Rows: map[string][]map[string]model.TypedValue{"people": {nil}}}, Result: model.DataCheckResult{EngineVersion: "0.5.0", Valid: false, Issues: []model.ValidationIssue{{Path: "rows.people[0]", Message: "Invalid row"}}, EntityCount: 1, RowCount: 1}}
	if err := db.SaveDataValidation(ctx, first); err != nil {
		t.Fatal(err)
	}
	changed := first
	changed.Fixture.Name = "Replacement"
	if err := db.SaveDataValidation(ctx, changed); err == nil {
		t.Fatal("existing validation was replaced")
	}
	for i := 1; i <= 55; i++ {
		r := first
		r.ID = fmt.Sprintf("validation-%03d", i)
		r.CreatedAt = time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		r.Result.Valid = i%2 == 0
		if err := db.SaveDataValidation(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	bad := first
	bad.ID = "bad-time"
	bad.CreatedAt = "yesterday"
	if err := db.SaveDataValidation(ctx, bad); err == nil {
		t.Fatal("invalid timestamp accepted")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	history, err := db.DataValidations(ctx)
	if err != nil || len(history) != 50 || history[0].ID != "validation-055" || history[49].ID != "validation-006" || history[0].ContractID != c.ID || history[0].ContractVersion != c.Version || history[0].RowCount != 1 || history[0].Valid {
		t.Fatalf("history: %#v %v", history, err)
	}
	detail, err := db.DataValidation(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(detail, first) {
		t.Fatalf("original snapshot was changed or trimmed: %#v %v", detail, err)
	}
	detail.Contract.Contract.Records[0].Fields[0].Name = "changed"
	detail.Result.Issues[0].Message = "changed"
	again, err := db.DataValidation(ctx, first.ID)
	if err != nil || !reflect.DeepEqual(again, first) {
		t.Fatalf("read mutated snapshot: %#v %v", again, err)
	}
	if _, err := db.DataValidation(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing validation: %v", err)
	}
	if records, err := db.DataContracts(ctx); err != nil || len(records) != 0 {
		t.Fatalf("validation installed contract: %#v %v", records, err)
	}
}
