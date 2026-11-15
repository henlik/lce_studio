package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"software-studio/internal/assembly"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"time"
)

var ErrPureExecution = errors.New("model execution failed")

type ExecutionPreparation = assembly.ExecutionPreparation
type SystemExecution struct {
	SnapshotVersion int                 `json:"snapshotVersion,omitempty"`
	PlanHash        string              `json:"planHash,omitempty"`
	ReportHash      string              `json:"reportHash,omitempty"`
	Scenario        *systemrun.Scenario `json:"scenario,omitempty"`
	ID              string              `json:"id"`
	CreatedAt       string              `json:"createdAt"`
	Design          system.SystemDesign `json:"design"`
	Inputs          []systemrun.Inputs  `json:"inputs"`
	ExecutionPreparation
	InputHash string           `json:"inputHash"`
	Report    systemrun.Report `json:"report"`
}
type ExecutionSummary struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Name      string `json:"name"`
	TaskCount int    `json:"taskCount"`
}

func executionHash(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func (s *Store) PrepareSystemExecution(ctx context.Context, d system.SystemDesign) (ExecutionPreparation, error) {
	return assembly.Prepare(ctx, assemblySource{s}, d)
}
func (s *Store) ExecuteSystem(ctx context.Context, id string, d system.SystemDesign, inputs []systemrun.Inputs) (SystemExecution, error) {
	return s.executeSystem(ctx, id, d, inputs, nil)
}
func (s *Store) ExecuteSystemScenario(ctx context.Context, id string, d system.SystemDesign, scenario systemrun.Scenario) (SystemExecution, error) {
	return s.executeSystem(ctx, id, d, nil, &scenario)
}
func (s *Store) executeSystem(ctx context.Context, id string, d system.SystemDesign, inputs []systemrun.Inputs, scenario *systemrun.Scenario) (SystemExecution, error) {
	p, err := s.PrepareSystemExecution(ctx, d)
	if err != nil {
		return SystemExecution{}, err
	}
	var report systemrun.Report
	if scenario != nil {
		report, err = systemrun.RunScenario(ctx, p.Plan, *scenario)
	} else {
		report, err = systemrun.Run(ctx, p.Plan, inputs)
	}
	if err != nil {
		var invalid *systemrun.Invalid
		if errors.As(err, &invalid) || ctx.Err() != nil {
			return SystemExecution{}, err
		}
		return SystemExecution{}, fmt.Errorf("%w: %v", ErrPureExecution, err)
	}
	e := SystemExecution{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Design: d, Inputs: inputs, ExecutionPreparation: p, InputHash: executionHash(inputs), Report: report}
	e.SnapshotVersion, e.PlanHash, e.ReportHash = 1, executionHash(p.Plan), executionHash(report)
	if scenario != nil {
		e.Scenario = scenario
		e.InputHash = executionHash(scenario)
	}
	b, err := json.Marshal(e)
	if err != nil {
		return SystemExecution{}, err
	}
	if len(b) > 8<<20 {
		return SystemExecution{}, fmt.Errorf("%w: execution snapshot exceeds 8 MiB", ErrPureExecution)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO system_executions(id,system_id,created_at,body) VALUES($1,$2,$3,$4)`, id, d.ID, e.CreatedAt, string(b))
	if err != nil {
		return SystemExecution{}, err
	}
	var frozen SystemExecution
	err = json.Unmarshal(b, &frozen)
	return frozen, err
}
func (s *Store) SystemExecution(ctx context.Context, id string) (SystemExecution, error) {
	var e SystemExecution
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM system_executions WHERE id=$1`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	if err != nil {
		return e, err
	}
	err = json.Unmarshal(b, &e)
	return e, err
}
func (s *Store) SystemExecutions(ctx context.Context, systemID string) ([]ExecutionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,body->'design'->>'name',json_array_length(body->'plan'->'tasks') FROM system_executions WHERE system_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50`, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExecutionSummary{}
	for rows.Next() {
		var e ExecutionSummary
		var at time.Time
		if err := rows.Scan(&e.ID, &at, &e.Name, &e.TaskCount); err != nil {
			return nil, err
		}
		e.CreatedAt = at.UTC().Format(time.RFC3339Nano)
		out = append(out, e)
	}
	return out, rows.Err()
}
