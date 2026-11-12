package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/state"
)

type StateRunRecord struct {
	ID        string                 `json:"id"`
	CreatedAt string                 `json:"createdAt"`
	Machine   model.StateMachineLock `json:"machine"`
	Scenario  model.StateScenario    `json:"scenario"`
	Result    state.Result           `json:"result"`
}

type StateRunSummary struct {
	ID             string `json:"id"`
	CreatedAt      string `json:"createdAt"`
	MachineID      string `json:"machineId"`
	MachineVersion string `json:"machineVersion"`
	StepCount      int    `json:"stepCount"`
	Verified       bool   `json:"verified"`
	Passed         bool   `json:"passed"`
}

// SaveStateRun stores complete experiment evidence in one INSERT. Simulated
// commits and restarts have no separate PostgreSQL application-state effects.
func (s *Store) SaveStateRun(ctx context.Context, record StateRunRecord) error {
	created, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid state run creation time: %w", err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	summary := StateRunSummary{ID: record.ID, CreatedAt: record.CreatedAt, MachineID: record.Machine.Machine.ID, MachineVersion: record.Machine.Machine.Version, StepCount: len(record.Result.Steps), Verified: record.Result.Verified, Passed: record.Result.Passed}
	brief, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO state_runs(id,created_at,body,summary) VALUES($1,$2,$3,$4)`, record.ID, created, string(body), string(brief))
	return err
}

func (s *Store) StateRun(ctx context.Context, id string) (StateRunRecord, error) {
	var record StateRunRecord
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM state_runs WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	if err != nil {
		return record, err
	}
	err = json.Unmarshal(body, &record)
	return record, err
}

func (s *Store) StateRuns(ctx context.Context) ([]StateRunSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT summary FROM state_runs ORDER BY created_at DESC,id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]StateRunSummary, 0)
	for rows.Next() {
		var body []byte
		var record StateRunSummary
		if err = rows.Scan(&body); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(body, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
