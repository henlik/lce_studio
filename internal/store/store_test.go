package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"software-studio/internal/model"
)

func TestProjectsAndRunsSurviveReopen(t *testing.T) {
	ctx := context.Background()
	filename := filepath.Join(t.TempDir(), "data", "studio.db")
	db, err := Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	p := model.ExampleProject()
	if err := db.SaveProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	r := model.Run{ID: "run-1", ProjectID: p.ID, CreatedAt: "2026-11-12T00:00:00Z", Project: p, ModelHash: "hash", Seed: 42, Events: []model.TraceEvent{{Sequence: 1, Kind: "sent"}}, Verdicts: []model.Verdict{}}
	if err := db.SaveRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := db.Project(ctx, p.ID)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("project roundtrip: %#v, %v", got, err)
	}
	p.Name = "Changed after run"
	if err := db.SaveProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	stored, err := db.Run(ctx, r.ID)
	if err != nil || !reflect.DeepEqual(stored, r) {
		t.Fatalf("immutable run changed: %#v, %v", stored, err)
	}
	if err := db.SaveRun(ctx, r); err == nil {
		t.Fatal("duplicate run ID must not overwrite an existing snapshot")
	}
	if err := db.DeleteProject(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Project(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := db.Run(ctx, r.ID); err != nil {
		t.Fatalf("deleting editable project lost run snapshot: %v", err)
	}
	list, err := db.Runs(ctx, p.ID)
	if err != nil || len(list) != 1 || len(list[0].Events) != 0 {
		t.Fatalf("run summaries: %#v, %v", list, err)
	}
	other, err := db.Runs(ctx, "unrelated")
	if err != nil || other == nil || len(other) != 0 {
		t.Fatalf("filter/empty slice: %#v, %v", other, err)
	}
}

func TestMemoryStoreAndNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	projects, err := db.Projects(ctx)
	if err != nil || projects == nil || len(projects) != 0 {
		t.Fatalf("empty projects: %#v, %v", projects, err)
	}
	if _, err := db.Run(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("run: %v", err)
	}
	if err := db.DeleteProject(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete: %v", err)
	}
}
