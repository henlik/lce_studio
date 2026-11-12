package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/workflow"
)

type WorkflowRunRecord struct {
	ID        string                 `json:"id"`
	CreatedAt string                 `json:"createdAt"`
	Workflow  model.WorkflowLock     `json:"workflow"`
	Scenario  model.WorkflowScenario `json:"scenario"`
	Result    workflow.Result        `json:"result"`
}

type WorkflowRunSummary struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"createdAt"`
	WorkflowID      string `json:"workflowId"`
	WorkflowVersion string `json:"workflowVersion"`
	Status          string `json:"status"`
	Outcome         string `json:"outcome"`
	EventCount      int    `json:"eventCount"`
	TaskCount       int    `json:"taskCount"`
	Verified        bool   `json:"verified"`
	Passed          bool   `json:"passed"`
}

// SaveWorkflowRun stores complete experiment evidence in one INSERT. Simulated
// tasks and owner commits are experiment evidence, not live application records.
func (s *Store) SaveWorkflowRun(ctx context.Context, record WorkflowRunRecord) error {
	created, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid workflow run creation time: %w", err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	summary := WorkflowRunSummary{ID: record.ID, CreatedAt: record.CreatedAt, WorkflowID: record.Workflow.Workflow.ID, WorkflowVersion: record.Workflow.Workflow.Version, Status: record.Result.Status, Outcome: record.Result.Outcome, EventCount: len(record.Result.Events), TaskCount: len(record.Result.Tasks), Verified: record.Result.Verified, Passed: record.Result.Passed}
	brief, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO workflow_runs(id,created_at,body,summary) VALUES($1,$2,$3,$4)`, record.ID, created, string(body), string(brief))
	return err
}

func (s *Store) WorkflowRun(ctx context.Context, id string) (WorkflowRunRecord, error) {
	var record WorkflowRunRecord
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM workflow_runs WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	if err != nil {
		return record, err
	}
	err = json.Unmarshal(body, &record)
	return record, err
}

func (s *Store) WorkflowRuns(ctx context.Context) ([]WorkflowRunSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT summary FROM workflow_runs ORDER BY created_at DESC,id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]WorkflowRunSummary, 0)
	for rows.Next() {
		var body []byte
		var record WorkflowRunSummary
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
