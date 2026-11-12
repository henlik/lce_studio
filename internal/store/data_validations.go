package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"software-studio/internal/model"
)

type DataValidationRecord struct {
	ID        string                 `json:"id"`
	CreatedAt string                 `json:"createdAt"`
	Contract  model.DataContractLock `json:"contract"`
	Fixture   model.DataFixture      `json:"fixture"`
	Result    model.DataCheckResult  `json:"result"`
}

type DataValidationSummary struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"createdAt"`
	ContractID      string `json:"contractId"`
	ContractVersion string `json:"contractVersion"`
	Valid           bool   `json:"valid"`
	RowCount        int    `json:"rowCount"`
}

func (s *Store) SaveDataValidation(ctx context.Context, record DataValidationRecord) error {
	created, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid data validation creation time: %w", err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	summary := DataValidationSummary{ID: record.ID, CreatedAt: record.CreatedAt, ContractID: record.Contract.Contract.ID, ContractVersion: record.Contract.Contract.Version, Valid: record.Result.Valid, RowCount: record.Result.RowCount}
	brief, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO data_validations(id,created_at,body,summary) VALUES($1,$2,$3,$4)`, record.ID, created, string(body), string(brief))
	return err
}

func (s *Store) DataValidation(ctx context.Context, id string) (DataValidationRecord, error) {
	var record DataValidationRecord
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM data_validations WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	if err != nil {
		return record, err
	}
	err = json.Unmarshal(body, &record)
	return record, err
}

// DataValidations lists recent summaries without deleting historical snapshots.
func (s *Store) DataValidations(ctx context.Context) ([]DataValidationSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT summary FROM data_validations ORDER BY created_at DESC,id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]DataValidationSummary, 0)
	for rows.Next() {
		var body []byte
		var record DataValidationSummary
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
