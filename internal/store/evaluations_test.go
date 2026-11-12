package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
	"software-studio/internal/testutil"
)

func TestEvaluationEvidenceSurvivesReopenAndCannotBeReplaced(t *testing.T) {
	databaseURL := testutil.DatabaseURL(t)
	db, err := Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	ctx := context.Background()
	bundle := compositeFixture(t, "test.evaluation-root", compositeLeaf("test.evaluation-child"))
	value := true
	inputs := map[string]model.TypedValue{"value": {Type: "boolean", Boolean: &value}}
	result, err := behavior.Evaluate(ctx, bundle, inputs)
	if err != nil {
		t.Fatal(err)
	}
	record := EvaluationRecord{ID: "evaluation-1", CreatedAt: "2026-11-12T00:00:00.123456789Z", Bundle: bundle, Inputs: inputs, Result: result}
	if err := db.SaveEvaluation(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveEvaluation(ctx, record); err == nil {
		t.Fatal("duplicate evaluation ID overwrote evidence")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.Evaluation(ctx, record.ID)
	if err != nil || !reflect.DeepEqual(got, record) {
		t.Fatalf("evaluation roundtrip: %#v, %v", got, err)
	}
	value = false
	bundle.Packages[0].Package.Description = "Changed after evaluation"
	again, err := db.Evaluation(ctx, record.ID)
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("caller mutation changed stored evidence: %#v, %v", again, err)
	}
	if _, err := db.Evaluation(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing evidence: %v", err)
	}
	if records, err := db.Components(ctx); err != nil || len(records) != 0 {
		t.Fatalf("portable evaluation installed catalog versions: %#v, %v", records, err)
	}
}

func TestEvaluationHistoryLimitsSummariesWithoutDeletingEvidence(t *testing.T) {
	db, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if summaries, err := db.Evaluations(ctx); err != nil || summaries == nil || len(summaries) != 0 {
		t.Fatalf("empty evaluation history: %#v, %v", summaries, err)
	}
	base := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 55; i++ {
		created := base.Add(time.Duration(i) * time.Millisecond)
		if i%2 == 0 {
			created = created.In(time.FixedZone("offset", 7200))
		}
		record := EvaluationRecord{ID: fmt.Sprintf("evaluation-%02d", i), CreatedAt: created.Format(time.RFC3339Nano),
			Bundle: model.ComponentBundle{Root: model.ComponentRef{PackageID: "test.history", Version: "1.0.0"}},
			Result: behavior.Result{Operations: i, Invocations: 1}}
		if err := db.SaveEvaluation(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	summaries, err := db.Evaluations(ctx)
	if err != nil || len(summaries) != 50 {
		t.Fatalf("summary limit: %d, %v", len(summaries), err)
	}
	for i, summary := range summaries {
		if summary.ID != fmt.Sprintf("evaluation-%02d", 54-i) || summary.ComponentID != "test.history" || summary.Operations != 54-i {
			t.Fatalf("summary order/contents: %#v", summary)
		}
	}
	if _, err := db.Evaluation(ctx, "evaluation-00"); err != nil {
		t.Fatalf("old evidence trimmed: %v", err)
	}
	if err := db.SaveEvaluation(ctx, EvaluationRecord{ID: "bad-time", CreatedAt: "invalid"}); err == nil {
		t.Fatal("invalid evidence timestamp accepted")
	}
}
