package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"software-studio/internal/systemrun"
	"software-studio/internal/testutil"
	"strings"
	"testing"
)

func TestSystemReplayPersistenceIntegrityAndVersions(t *testing.T) {
	ctx := context.Background()
	s, err := Open(testutil.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	example, err := s.InstallConnectionExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PrepareSystemExecution(ctx, example.Design)
	if err != nil {
		t.Fatal(err)
	}
	e, err := s.ExecuteSystem(ctx, "replay-source", example.Design, encounterInputs(p.Plan))
	if err != nil {
		t.Fatal(err)
	}
	if e.SnapshotVersion != 1 || e.PlanHash == "" || e.ReportHash == "" {
		t.Fatal("new execution lacks integrity hashes")
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM component_packages`); err != nil {
		t.Fatal(err)
	}
	r, err := s.ReplaySystemExecution(ctx, "replay-proof", e.ID)
	if err != nil || r.Status != "matched" || r.Report == nil || !reflect.DeepEqual(*r.Report, e.Report) || r.RecordedReportHash != r.ReplayedReportHash || r.SnapshotHash == "" {
		t.Fatalf("portable replay: %+v %v", r, err)
	}
	saved, err := s.SystemReplay(ctx, r.ID)
	if err != nil || !reflect.DeepEqual(saved, r) {
		t.Fatal("verification not preserved", err)
	}
	original, err := s.SystemExecution(ctx, e.ID)
	if err != nil || !reflect.DeepEqual(original, e) {
		t.Fatal("source changed", err)
	}
	rows, err := s.SystemReplays(ctx, e.ID)
	if err != nil || len(rows) != 1 || rows[0].ID != r.ID {
		t.Fatal("history", rows, err)
	}
	if _, err = s.ReplaySystemExecution(ctx, r.ID, e.ID); err == nil {
		t.Fatal("verification overwritten")
	}
	if _, err = s.ReplaySystemExecution(ctx, "missing-replay", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.SystemReplays(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.ReplaySystemExecution(cancelled, "cancelled-replay", e.ID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
	if _, err = s.SystemReplay(ctx, "cancelled-replay"); !errors.Is(err, ErrNotFound) {
		t.Fatal("cancelled verification saved", err)
	}
	clone := func() SystemExecution {
		b, _ := json.Marshal(e)
		var copy SystemExecution
		_ = json.Unmarshal(b, &copy)
		return copy
	}
	cases := []struct {
		name, status string
		edit         func(*SystemExecution)
		replayed     bool
	}{
		{"design corruption", "mismatch", func(e *SystemExecution) { e.Design.Name = "changed" }, false},
		{"source identity corruption", "mismatch", func(e *SystemExecution) { e.ID = "other-execution" }, false},
		{"input corruption", "mismatch", func(e *SystemExecution) { e.Inputs = nil }, false},
		{"incomplete inputs with consistent hash", "unverified", func(e *SystemExecution) { e.Inputs = nil; e.InputHash = executionHash(e.Inputs) }, false},
		{"plan corruption", "mismatch", func(e *SystemExecution) { e.Plan.Order = nil }, false},
		{"report corruption", "mismatch", func(e *SystemExecution) { e.Report.Events[0].Kind = "forged" }, false},
		{"missing new hash", "mismatch", func(e *SystemExecution) { e.PlanHash = "" }, false},
		{"schedule corruption with consistent hash", "mismatch", func(e *SystemExecution) { e.Plan.Order = nil; e.PlanHash = executionHash(e.Plan) }, false},
		{"unsupported snapshot", "unverified", func(e *SystemExecution) { e.SnapshotVersion = 99 }, false},
		{"unsupported plan engine", "unverified", func(e *SystemExecution) { e.Plan.EngineVersion = "future-engine"; e.PlanHash = executionHash(e.Plan) }, false},
		{"unsupported report engine", "unverified", func(e *SystemExecution) {
			e.Report.EngineVersion = "future-engine"
			e.ReportHash = executionHash(e.Report)
		}, false},
		{"unsupported claim semantics", "unverified", func(e *SystemExecution) { e.Report.ClaimsEvaluated = true; e.ReportHash = executionHash(e.Report) }, false},
		{"legacy record", "matched", func(e *SystemExecution) { e.SnapshotVersion = 0; e.PlanHash = ""; e.ReportHash = "" }, true},
		{"changed recorded output", "mismatch", func(e *SystemExecution) { e.Report.Events[0].Kind = "forged"; e.ReportHash = executionHash(e.Report) }, true},
		{"invalid pinned definition", "unverified", func(e *SystemExecution) {
			e.Plan.Tasks[0].Bundle.Root.Digest = strings.Repeat("0", 64)
			e.PlanHash = executionHash(e.Plan)
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := clone()
			tc.edit(&copy)
			result, err := verifyExecution(ctx, copy, e.ID)
			if err != nil || result.Status != tc.status || (result.Report != nil) != tc.replayed {
				t.Fatalf("%+v %v", result, err)
			}
			if tc.name == "changed recorded output" && (len(result.Differences) != 1 || result.Differences[0].Path != "/report/events/0/kind") {
				t.Fatal(result.Differences)
			}
			if tc.name == "legacy record" && !strings.Contains(result.Checks[4].Message, "Legacy") {
				t.Fatal("legacy limitation hidden")
			}
		})
	}
	// Mismatches and unsupported records must also remain inspectable evidence.
	for _, status := range []string{"mismatch", "unverified"} {
		copy := clone()
		copy.ID = "source-" + status
		if status == "mismatch" {
			copy.Report.Events[0].Kind = "forged"
			copy.ReportHash = executionHash(copy.Report)
		} else {
			copy.Report.EngineVersion = "future"
			copy.ReportHash = executionHash(copy.Report)
		}
		b, _ := json.Marshal(copy)
		_, err = s.db.ExecContext(ctx, `INSERT INTO system_executions(id,system_id,created_at,body) VALUES($1,$2,$3,$4)`, copy.ID, copy.Design.ID, copy.CreatedAt, string(b))
		if err != nil {
			t.Fatal(err)
		}
		proof, err := s.ReplaySystemExecution(ctx, "proof-"+status, copy.ID)
		if err != nil || proof.Status != status {
			t.Fatal(proof, err)
		}
		stored, err := s.SystemReplay(ctx, proof.ID)
		if err != nil || !reflect.DeepEqual(proof, stored) {
			t.Fatal("negative proof not retained", err)
		}
	}
}

func TestReplayDifferenceBoundsAndJSONPointers(t *testing.T) {
	a := systemrun.Report{Events: []systemrun.Event{}}
	b := systemrun.Report{Events: []systemrun.Event{}}
	for i := 0; i < 40; i++ {
		a.Events = append(a.Events, systemrun.Event{Sequence: i, Kind: strings.Repeat("a", 300)})
		b.Events = append(b.Events, systemrun.Event{Sequence: i, Kind: "b"})
	}
	diff, truncated := reportDifferences(a, b)
	if !truncated || len(diff) != 32 || len([]rune(diff[0].Recorded)) > 161 {
		t.Fatal("unbounded differences", len(diff), truncated)
	}
	first, _ := json.Marshal(diff)
	again, _ := reportDifferences(a, b)
	second, _ := json.Marshal(again)
	if string(first) != string(second) {
		t.Fatal("nondeterministic differences")
	}
	a.Results = []systemrun.TaskResult{{TaskID: "before"}}
	b = a
	b.Results = nil
	diff, _ = reportDifferences(a, b)
	if len(diff) != 1 || diff[0].Path != "/report/results" {
		t.Fatal("missing result not located", diff)
	}
}
