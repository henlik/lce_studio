package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/testutil"
)

func compositeLeaf(id string) model.ComponentPackage {
	return model.ComponentPackage{SchemaVersion: 2, ID: id, Version: "1.0.0", Name: "Boolean pass-through", Author: "Test engineer", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"},
		Behavior: &model.Behavior{
			Inputs: []model.BehaviorPort{{Name: "value", Type: "boolean"}}, Outputs: []model.BehaviorPort{{Name: "result", Type: "boolean"}},
			Nodes:    []model.BehaviorNode{{ID: "copy", Op: "identity", Inputs: map[string]model.BehaviorBinding{"value": {Input: "value"}}}},
			Bindings: map[string]model.BehaviorBinding{"result": {Node: "copy", Port: "result"}},
		}}
}

func compositeLock(t *testing.T, p model.ComponentPackage) model.PackageLock {
	t.Helper()
	digest, err := model.ComponentDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	return model.PackageLock{Package: p, Digest: digest}
}

func compositeFixture(t *testing.T, id string, children ...model.ComponentPackage) model.ComponentBundle {
	t.Helper()
	root := compositeLeaf(id)
	root.Behavior.Nodes = []model.BehaviorNode{}
	locks := make([]model.PackageLock, len(children))
	for i, child := range children {
		locks[i] = compositeLock(t, child)
		root.Behavior.Nodes = append(root.Behavior.Nodes, model.BehaviorNode{ID: fmt.Sprintf("call%d", i),
			Component: &model.ComponentRef{PackageID: child.ID, Version: child.Version, Digest: locks[i].Digest},
			Inputs:    map[string]model.BehaviorBinding{"value": {Input: "value"}}})
	}
	root.Behavior.Bindings = map[string]model.BehaviorBinding{"result": {Node: "call0", Port: "result"}}
	if len(children) == 2 {
		root.Behavior.Nodes = append(root.Behavior.Nodes, model.BehaviorNode{ID: "both", Op: "and", Inputs: map[string]model.BehaviorBinding{"left": {Node: "call0", Port: "result"}, "right": {Node: "call1", Port: "result"}}})
		root.Behavior.Bindings["result"] = model.BehaviorBinding{Node: "both", Port: "result"}
	}
	bundle, issues := model.PrepareComponentBundle(root, locks)
	if len(issues) != 0 {
		t.Fatalf("fixture is invalid: %#v", issues)
	}
	return bundle
}

func TestCompositeCatalogAtomicityAndExport(t *testing.T) {
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	ctx := context.Background()
	shared := compositeLeaf("test.shared")
	first := compositeFixture(t, "test.first", shared)
	records, created, err := db.InstallComponentBundle(ctx, first)
	if err != nil || !created || len(records) != 2 {
		t.Fatalf("first bundle: %d records, created %v, %v", len(records), created, err)
	}
	again, created, err := db.InstallComponentBundle(ctx, first)
	if err != nil || created || !reflect.DeepEqual(records, again) {
		t.Fatalf("idempotent bundle: %#v, %v, %v", again, created, err)
	}
	conflicting := shared
	conflicting.Description = "Different content at the existing exact version"
	fresh := compositeLeaf("test.fresh")
	bad := compositeFixture(t, "test.rejected", conflicting, fresh)
	if _, _, err := db.InstallComponentBundle(ctx, bad); !errors.Is(err, ErrComponentConflict) {
		t.Fatalf("expected dependency conflict: %v", err)
	}
	remaining, err := db.Components(ctx)
	if err != nil || !reflect.DeepEqual(remaining, records) {
		t.Fatalf("conflicting closure partially installed: %#v, %v", remaining, err)
	}
	second := compositeFixture(t, "test.second", shared)
	if _, created, err := db.InstallComponentBundle(ctx, second); err != nil || !created {
		t.Fatalf("shared dependency reuse: %v, %v", created, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := db.ComponentBundle(ctx, first.Root)
	if err != nil || len(model.ValidateComponentBundle(exported)) != 0 || exported.Root != first.Root || len(exported.Packages) != 2 {
		t.Fatalf("export after reopen: %#v, %v", exported, err)
	}
	for _, lock := range exported.Packages {
		if lock.Package.ID == "test.second" {
			t.Fatal("export included an unrelated installed root")
		}
	}
	missing := first.Root
	missing.Digest = "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := db.ComponentBundle(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong exact digest: %v", err)
	}
	profile := catalogPackage()
	installed, _, err := db.InstallComponent(ctx, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ComponentBundle(ctx, model.ComponentRef{PackageID: profile.ID, Version: profile.Version, Digest: installed.Digest}); !errors.Is(err, ErrNotComposite) {
		t.Fatalf("profile exported as behavior: %v", err)
	}
}

func TestCompositeCapacityFailureInstallsNothing(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	for i := 0; i < MaxComponentVersions-1; i++ {
		p := catalogPackage()
		p.ID = fmt.Sprintf("test.existing-%03d", i)
		if _, _, err := db.InstallComponent(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	bundle := compositeFixture(t, "test.full-root", compositeLeaf("test.full-child"))
	if _, _, err := db.InstallComponentBundle(ctx, bundle); !errors.Is(err, ErrCatalogFull) {
		t.Fatalf("bundle exceeding available slot: %v", err)
	}
	records, err := db.Components(ctx)
	if err != nil || len(records) != MaxComponentVersions-1 {
		t.Fatalf("capacity failure changed catalog: %d records, %v", len(records), err)
	}
	for _, record := range records {
		if record.Package.SchemaVersion != 1 {
			t.Fatal("capacity failure left a composite dependency installed")
		}
	}
}

func TestConcurrentBundlesDoNotPartiallyInstallConflicts(t *testing.T) {
	databaseURL := testutil.DatabaseURL(t)
	firstStore, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer firstStore.Close()
	secondStore, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer secondStore.Close()
	child := compositeLeaf("test.contested")
	a := compositeFixture(t, "test.root-a", child)
	child.Description = "Second immutable content candidate"
	b := compositeFixture(t, "test.root-b", child)
	results := make(chan error, 2)
	start := make(chan struct{})
	for i, bundle := range []model.ComponentBundle{a, b} {
		db := []*Store{firstStore, secondStore}[i]
		go func(db *Store, bundle model.ComponentBundle) {
			<-start
			_, _, err := db.InstallComponentBundle(context.Background(), bundle)
			results <- err
		}(db, bundle)
	}
	close(start)
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, ErrComponentConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	records, err := firstStore.Components(context.Background())
	if err != nil || success != 1 || conflict != 1 || len(records) != 2 {
		t.Fatalf("concurrent bundles: %d success %d conflict %d records, %v", success, conflict, len(records), err)
	}
}
