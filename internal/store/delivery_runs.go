package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"software-studio/internal/delivery"
	"software-studio/internal/model"
)

type DeliveryRunRecord struct {
	ID        string                  `json:"id"`
	CreatedAt string                  `json:"createdAt"`
	Model     model.DeliveryModelLock `json:"model"`
	Scenario  model.DeliveryScenario  `json:"scenario"`
	Result    delivery.Result         `json:"result"`
}

type DeliveryRunSummary struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"createdAt"`
	ModelID      string `json:"modelId"`
	ModelVersion string `json:"modelVersion"`
	StepCount    int    `json:"stepCount"`
	MessageCount int    `json:"messageCount"`
	Verified     bool   `json:"verified"`
	Passed       bool   `json:"passed"`
}

// SaveDeliveryRun stores complete experiment evidence in one INSERT. Simulated
// transport and recipient commits have no separate PostgreSQL runtime effects.
func (s *Store) SaveDeliveryRun(ctx context.Context, record DeliveryRunRecord) error {
	created, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid delivery run creation time: %w", err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	summary := DeliveryRunSummary{ID: record.ID, CreatedAt: record.CreatedAt, ModelID: record.Model.Model.ID, ModelVersion: record.Model.Model.Version, StepCount: len(record.Result.Steps), MessageCount: len(record.Result.Messages), Verified: record.Result.Verified, Passed: record.Result.Passed}
	brief, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO delivery_runs(id,created_at,body,summary) VALUES($1,$2,$3,$4)`, record.ID, created, string(body), string(brief))
	return err
}

func (s *Store) DeliveryRun(ctx context.Context, id string) (DeliveryRunRecord, error) {
	var record DeliveryRunRecord
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM delivery_runs WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	if err != nil {
		return record, err
	}
	err = json.Unmarshal(body, &record)
	return record, err
}

func (s *Store) DeliveryRuns(ctx context.Context) ([]DeliveryRunSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT summary FROM delivery_runs ORDER BY created_at DESC,id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]DeliveryRunSummary, 0)
	for rows.Next() {
		var body []byte
		var record DeliveryRunSummary
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
