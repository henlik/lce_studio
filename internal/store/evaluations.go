package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
)

type EvaluationRecord struct {
	ID        string                      `json:"id"`
	CreatedAt string                      `json:"createdAt"`
	Bundle    model.ComponentBundle       `json:"bundle"`
	Inputs    map[string]model.TypedValue `json:"inputs"`
	Result    behavior.Result             `json:"result"`
}

type EvaluationSummary struct {
	ID               string `json:"id"`
	CreatedAt        string `json:"createdAt"`
	ComponentID      string `json:"componentId"`
	ComponentVersion string `json:"componentVersion"`
	Operations       int    `json:"operations"`
	Invocations      int    `json:"invocations"`
}

func (s *Store) SaveEvaluation(ctx context.Context, record EvaluationRecord) error {
	created, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid evaluation creation time: %w", err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	summary := EvaluationSummary{ID: record.ID, CreatedAt: record.CreatedAt,
		ComponentID: record.Bundle.Root.PackageID, ComponentVersion: record.Bundle.Root.Version,
		Operations: record.Result.Operations, Invocations: record.Result.Invocations}
	brief, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO component_evaluations(id,created_at,body,summary) VALUES($1,$2,$3,$4)`, record.ID, created, string(body), string(brief))
	return err
}

func (s *Store) Evaluation(ctx context.Context, id string) (EvaluationRecord, error) {
	var record EvaluationRecord
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM component_evaluations WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	if err != nil {
		return record, err
	}
	err = json.Unmarshal(body, &record)
	return record, err
}

// Evaluations returns the newest 50 summaries without trimming older evidence.
func (s *Store) Evaluations(ctx context.Context) ([]EvaluationSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT summary FROM component_evaluations ORDER BY created_at DESC,id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]EvaluationSummary, 0)
	for rows.Next() {
		var body []byte
		var record EvaluationSummary
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
