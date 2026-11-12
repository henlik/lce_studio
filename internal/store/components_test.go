package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/testutil"
)

func catalogPackage() model.ComponentPackage {
	return model.ComponentPackage{
		SchemaVersion: 1, ID: "test.worker", Version: "1.0.0", Name: "Queued worker",
		Description: "Reusable bounded worker", Author: "Test engineer", License: "MIT",
		Kernel: "service", Capabilities: []string{"design", "simulate"},
		Config: model.ComponentConfig{
			ProcessingMs: model.IntegerParameter{Default: 10, Min: 0, Max: 1000},
			Capacity:     model.IntegerParameter{Default: 2, Min: 1, Max: 10},
			MemoryMB:     model.IntegerParameter{Default: 64, Min: 1, Max: 1024},
		},
	}
}

func TestComponentCatalogVersioningAndReopen(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if records, err := db.Components(ctx); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty catalog: %#v, %v", records, err)
	}
	p := catalogPackage()
	want, created, err := db.InstallComponent(ctx, p)
	if err != nil || !created || want.Digest == "" || !reflect.DeepEqual(want.Package, p) {
		t.Fatalf("first installation: %#v, %v, %v", want, created, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, want.InstalledAt); err != nil {
		t.Fatalf("installation timestamp: %v", err)
	}
	again, created, err := db.InstallComponent(ctx, p)
	if err != nil || created || !reflect.DeepEqual(again, want) {
		t.Fatalf("idempotent installation changed record: %#v, %v, %v", again, created, err)
	}
	changed := p
	changed.Description = "Changed metadata is still different package content"
	if _, _, err := db.InstallComponent(ctx, changed); !errors.Is(err, ErrComponentConflict) {
		t.Fatalf("same version conflict: %v", err)
	}
	changed.Version = "2.0.0"
	if _, created, err := db.InstallComponent(ctx, changed); err != nil || !created {
		t.Fatalf("new version: %v, %v", created, err)
	}
	alpha := p
	alpha.ID = "test.alpha"
	if _, _, err := db.InstallComponent(ctx, alpha); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	records, err := db.Components(ctx)
	if err != nil || len(records) != 3 {
		t.Fatalf("reopened catalog: %#v, %v", records, err)
	}
	if records[0].Package.ID != "test.alpha" || records[1].Package.Version != "1.0.0" || records[2].Package.Version != "2.0.0" {
		t.Fatalf("catalog ordering: %#v", records)
	}
	if !reflect.DeepEqual(records[1], want) {
		t.Fatalf("original package replaced: %#v", records[1])
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := db.InstallComponent(cancelled, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled install: %v", err)
	}
}

func TestConcurrentComponentInstallationIsImmutable(t *testing.T) {
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
		record  ComponentRecord
		created bool
		err     error
	}
	const workers = 12
	for _, mixed := range []bool{false, true} {
		results := make(chan result, workers)
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			go func(i int) {
				p := catalogPackage()
				if mixed {
					p.ID = "test.competing"
					p.Description = fmt.Sprintf("Variant %d", i%2)
				}
				<-start
				r, c, e := stores[i%len(stores)].InstallComponent(context.Background(), p)
				results <- result{r, c, e}
			}(i)
		}
		close(start)
		created, conflicts := 0, 0
		var first *ComponentRecord
		for i := 0; i < workers; i++ {
			r := <-results
			if errors.Is(r.err, ErrComponentConflict) {
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
				t.Fatalf("successful concurrent installs disagree: %#v vs %#v", *first, r.record)
			}
		}
		wantConflicts := 0
		if mixed {
			wantConflicts = workers / 2
		}
		if created != 1 || conflicts != wantConflicts {
			t.Fatalf("mixed=%v: %d new, %d conflicts", mixed, created, conflicts)
		}
	}
}

func TestComponentCatalogCapacityIsAtomic(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	for i := 0; i < MaxComponentVersions-1; i++ {
		p := catalogPackage()
		p.ID = fmt.Sprintf("test.worker-%03d", i)
		if _, _, err := db.InstallComponent(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	const workers = 8
	errorsCh := make(chan error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := catalogPackage()
			p.ID = fmt.Sprintf("test.race-%d", i)
			<-start
			_, _, err := db.InstallComponent(ctx, p)
			errorsCh <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errorsCh)
	full, success := 0, 0
	for err := range errorsCh {
		if err == nil {
			success++
		} else if errors.Is(err, ErrCatalogFull) {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || full != workers-1 {
		t.Fatalf("capacity race: %d successes, %d full", success, full)
	}
	records, err := db.Components(ctx)
	if err != nil || len(records) != MaxComponentVersions {
		t.Fatalf("catalog capacity: %d, %v", len(records), err)
	}
	existing := catalogPackage()
	existing.ID = "test.worker-000"
	if _, created, err := db.InstallComponent(ctx, existing); err != nil || created {
		t.Fatalf("identical reinstall at capacity: created=%v, err=%v", created, err)
	}
	existing.Description = "Changed"
	if _, _, err := db.InstallComponent(ctx, existing); !errors.Is(err, ErrComponentConflict) {
		t.Fatalf("conflicting reinstall at capacity: %v", err)
	}
}
