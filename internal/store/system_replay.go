package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"software-studio/internal/systemrun"
	"sort"
	"strings"
	"time"
)

const ReplayVerifierVersion = "system-replay-1"

type ReplayCheck struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
type ReplayDifference struct {
	Path     string `json:"path"`
	Recorded string `json:"recorded"`
	Replayed string `json:"replayed"`
}
type SystemReplay struct {
	ID                   string             `json:"id"`
	ExecutionID          string             `json:"executionId"`
	CreatedAt            string             `json:"createdAt"`
	VerifierVersion      string             `json:"verifierVersion"`
	SnapshotHash         string             `json:"snapshotHash"`
	Status               string             `json:"status"`
	Message              string             `json:"message"`
	Checks               []ReplayCheck      `json:"checks"`
	RecordedReportHash   string             `json:"recordedReportHash"`
	ReplayedReportHash   string             `json:"replayedReportHash,omitempty"`
	Report               *systemrun.Report  `json:"report,omitempty"`
	Differences          []ReplayDifference `json:"differences"`
	DifferencesTruncated bool               `json:"differencesTruncated"`
}
type ReplaySummary struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// verifyExecution uses only the frozen snapshot. It must never resolve against a
// mutable catalog or silently replace a recorded engine version with another one.
// Hashes detect inconsistency, not authenticity: these records are not signed.
func verifyExecution(ctx context.Context, saved SystemExecution, executionID string) (SystemReplay, error) {
	r := SystemReplay{ExecutionID: executionID, VerifierVersion: ReplayVerifierVersion, SnapshotHash: executionHash(saved), Status: "unverified", Checks: []ReplayCheck{}, Differences: []ReplayDifference{}, RecordedReportHash: executionHash(saved.Report)}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if saved.SnapshotVersion != 0 && saved.SnapshotVersion != 1 {
		r.Message = "Snapshot version is unsupported; no replay was attempted"
		return r, nil
	}
	b, err := json.Marshal(saved)
	if err != nil {
		return r, err
	}
	if len(b) > 8<<20 {
		r.Message = "Snapshot exceeds the 8 MiB replay limit"
		return r, nil
	}
	check := func(path string, equal bool, message string) {
		status := "matched"
		if !equal {
			status = "mismatch"
			r.Status = "mismatch"
		}
		r.Checks = append(r.Checks, ReplayCheck{Path: path, Status: status, Message: message})
	}
	check("/id", saved.ID == executionID, "Source execution identity consistency")
	check("/designHash", saved.DesignHash == executionHash(saved.Design), "Design hash consistency")
	inputHash := executionHash(saved.Inputs)
	if saved.Scenario != nil {
		inputHash = executionHash(saved.Scenario)
	}
	check("/inputHash", saved.InputHash == inputHash, "Input or scenario hash consistency")
	check("/inputs", saved.Scenario == nil || len(saved.Inputs) == 0, "Unambiguous scenario or single-run inputs")
	if saved.SnapshotVersion == 1 {
		check("/planHash", saved.PlanHash == executionHash(saved.Plan), "Pinned plan hash consistency")
		check("/reportHash", saved.ReportHash == r.RecordedReportHash, "Recorded report hash consistency")
	} else {
		r.Checks = append(r.Checks, ReplayCheck{Path: "/snapshotVersion", Status: "unverified", Message: "Legacy record has no stored plan/report hashes; reproduction can still be compared"})
	}
	if r.Status == "mismatch" {
		r.Message = "Saved snapshot integrity checks failed; no replay was attempted"
		return r, nil
	}
	plan, err := systemrun.Prepare(saved.Plan.Tasks, saved.Plan.Links)
	if err != nil {
		r.Message = "Pinned plan cannot be replayed: " + err.Error()
		return r, nil
	}
	engine, mode := plan.EngineVersion, "pure-dataflow"
	if plan.EngineVersion == systemrun.Version {
		mode = "stateful-dataflow"
	}
	if saved.Scenario != nil {
		engine, mode = "system-scenario-1", "shared-scenario"
	}
	if saved.Plan.EngineVersion != plan.EngineVersion || saved.Report.EngineVersion != engine || saved.Report.Mode != mode || saved.Report.ClaimsEvaluated {
		r.Message = "Recorded engine version, mode or claim semantics are unsupported; no replay was attempted"
		return r, nil
	}
	check("/plan", executionHash(plan) == executionHash(saved.Plan), "Schedule and external inputs derived from the pinned graph")
	if r.Status == "mismatch" {
		r.Message = "Saved schedule or input requirements differ from the pinned graph; no replay was attempted"
		return r, nil
	}
	var report systemrun.Report
	if saved.Scenario != nil {
		report, err = systemrun.RunScenario(ctx, saved.Plan, *saved.Scenario)
	} else {
		report, err = systemrun.Run(ctx, saved.Plan, saved.Inputs)
	}
	if err != nil {
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		r.Message = "Replay could not complete: " + err.Error()
		return r, nil
	}
	if err = ctx.Err(); err != nil {
		return r, err
	}
	r.Report = &report
	r.ReplayedReportHash = executionHash(report)
	check("/report", r.RecordedReportHash == r.ReplayedReportHash, "Complete report reproduction")
	if r.Status == "mismatch" {
		r.Message = "Replay completed but differs from the recorded report"
		r.Differences, r.DifferencesTruncated = reportDifferences(saved.Report, report)
	} else {
		r.Status = "matched"
		r.Message = "Saved execution reproduced exactly; this does not establish that scenario expectations passed or system claims hold"
	}
	return r, nil
}

// Deterministic JSON pointers and bounded previews make mismatches inspectable.
// Use json.Number so adjacent safe integers are never rounded during comparison.
func reportDifferences(recorded, replayed systemrun.Report) ([]ReplayDifference, bool) {
	decode := func(v any) any {
		b, _ := json.Marshal(v)
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.UseNumber()
		var out any
		_ = d.Decode(&out)
		return out
	}
	preview := func(v any, present bool) string {
		if !present {
			return "(missing)"
		}
		b, _ := json.Marshal(v)
		s := []rune(string(b))
		if len(s) > 160 {
			return string(s[:160]) + "…"
		}
		return string(s)
	}
	diffs := []ReplayDifference{}
	truncated := false
	var walk func(string, any, any, bool, bool)
	walk = func(path string, a, b any, ap, bp bool) {
		if ap == bp && executionHash(a) == executionHash(b) {
			return
		}
		if len(diffs) >= 32 {
			truncated = true
			return
		}
		am, aok := a.(map[string]any)
		bm, bok := b.(map[string]any)
		if ap && bp && aok && bok {
			keys := map[string]bool{}
			for k := range am {
				keys[k] = true
			}
			for k := range bm {
				keys[k] = true
			}
			sorted := []string{}
			for k := range keys {
				sorted = append(sorted, k)
			}
			sort.Strings(sorted)
			for _, k := range sorted {
				av, ao := am[k]
				bv, bo := bm[k]
				walk(path+"/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"), av, bv, ao, bo)
				if truncated {
					return
				}
			}
			return
		}
		aa, aok := a.([]any)
		ba, bok := b.([]any)
		if ap && bp && aok && bok {
			n := max(len(aa), len(ba))
			for i := 0; i < n; i++ {
				var av, bv any
				ao, bo := i < len(aa), i < len(ba)
				if ao {
					av = aa[i]
				}
				if bo {
					bv = ba[i]
				}
				walk(fmt.Sprintf("%s/%d", path, i), av, bv, ao, bo)
				if truncated {
					return
				}
			}
			return
		}
		diffs = append(diffs, ReplayDifference{Path: path, Recorded: preview(a, ap), Replayed: preview(b, bp)})
	}
	walk("/report", decode(recorded), decode(replayed), true, true)
	return diffs, truncated
}

func (s *Store) ReplaySystemExecution(ctx context.Context, id, executionID string) (SystemReplay, error) {
	saved, err := s.SystemExecution(ctx, executionID)
	if err != nil {
		return SystemReplay{}, err
	}
	r, err := verifyExecution(ctx, saved, executionID)
	if err != nil {
		return SystemReplay{}, err
	}
	r.ID, r.CreatedAt = id, time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(r)
	if err != nil {
		return SystemReplay{}, err
	}
	if len(b) > 6<<20 {
		return SystemReplay{}, fmt.Errorf("replay evidence exceeds 6 MiB")
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO system_execution_replays(id,execution_id,created_at,body) VALUES($1,$2,$3,$4)`, id, executionID, r.CreatedAt, string(b))
	return r, err
}
func (s *Store) SystemReplay(ctx context.Context, id string) (SystemReplay, error) {
	var r SystemReplay
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM system_execution_replays WHERE id=$1`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	return r, err
}
func (s *Store) SystemReplays(ctx context.Context, executionID string) ([]ReplaySummary, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM system_executions WHERE id=$1)`, executionID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,body->>'status',body->>'message' FROM system_execution_replays WHERE execution_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReplaySummary{}
	for rows.Next() {
		var r ReplaySummary
		var at time.Time
		if err = rows.Scan(&r.ID, &at, &r.Status, &r.Message); err != nil {
			return nil, err
		}
		r.CreatedAt = at.UTC().Format(time.RFC3339Nano)
		out = append(out, r)
	}
	return out, rows.Err()
}
