package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/testutil"
)

func TestProjectsAndRunsSurviveReopen(t *testing.T) {
	ctx := context.Background()
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	p := model.ExampleProject()
	p.Description = "Unicode: café 中文 🌍; escaped control: \x00"
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
	db, err = Open(databaseURL)
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

func TestEmptyStoreAndNotFound(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
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

func TestRunHistoryChronologyAndLimit(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	base := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 55; i++ {
		// Offsets and fractional seconds must sort by time, not text.
		created := base.Add(time.Duration(i) * time.Millisecond)
		if i%2 == 0 {
			created = created.In(time.FixedZone("offset", 2*60*60))
		}
		r := model.Run{ID: fmt.Sprintf("run-%02d", i), ProjectID: "history", CreatedAt: created.Format(time.RFC3339Nano)}
		if err := db.SaveRun(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	for _, filter := range []string{"", "history"} {
		runs, err := db.Runs(ctx, filter)
		if err != nil || len(runs) != 50 {
			t.Fatalf("history limit: %d runs, %v", len(runs), err)
		}
		for i, r := range runs {
			if r.ID != fmt.Sprintf("run-%02d", 54-i) {
				t.Fatalf("history order at %d: %s", i, r.ID)
			}
		}
	}
	if _, err := db.Run(ctx, "run-00"); err != nil {
		t.Fatalf("history limit removed full snapshot: %v", err)
	}
	if err := db.SaveRun(ctx, model.Run{ID: "bad-time", CreatedAt: "not-a-time"}); err == nil {
		t.Fatal("invalid creation time accepted")
	}
}

func TestConcurrentInitializationAndWrites(t *testing.T) {
	databaseURL := testutil.DatabaseURL(t)
	const workers = 6
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			db, err := Open(databaseURL)
			if err != nil {
				results <- err
				return
			}
			defer db.Close()
			p := model.ExampleProject()
			p.ID = fmt.Sprintf("parallel-%d", i)
			results <- db.SaveProject(context.Background(), p)
		}(i)
	}
	for i := 0; i < workers; i++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	projects, err := db.Projects(context.Background())
	if err != nil || len(projects) != workers {
		t.Fatalf("concurrent writes: %d projects, %v", len(projects), err)
	}
}

func TestOpenRejectsMissingOrMalformedConfiguration(t *testing.T) {
	for _, input := range []string{"", "   ", "postgres://user:private-password@host:invalid-port/studio"} {
		db, err := Open(input)
		if err == nil {
			db.Close()
			t.Fatal("invalid database configuration accepted")
		}
		if strings.Contains(err.Error(), "private-password") {
			t.Fatal("connection error leaked password")
		}
	}
}
