package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"software-studio/internal/model"
)

const MaxWorkflowPackageVersions = 200

var (
	ErrWorkflowPackageConflict = errors.New("workflow package version already contains different content")
	ErrWorkflowCatalogFull     = errors.New("workflow package catalog contains the maximum number of installed versions")
)

type WorkflowPackageRecord struct {
	Workflow    model.WorkflowDefinition `json:"workflow"`
	Digest      string                   `json:"digest"`
	InstalledAt string                   `json:"installedAt"`
}

func scanWorkflowPackage(row rowScanner) (WorkflowPackageRecord, error) {
	var record WorkflowPackageRecord
	var body []byte
	var installed time.Time
	if err := row.Scan(&body, &record.Digest, &installed); err != nil {
		return record, err
	}
	if err := json.Unmarshal(body, &record.Workflow); err != nil {
		return record, err
	}
	record.InstalledAt = installed.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func (s *Store) WorkflowPackages(ctx context.Context) ([]WorkflowPackageRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body,digest,installed_at FROM workflow_packages ORDER BY id COLLATE "C",version COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]WorkflowPackageRecord, 0)
	for rows.Next() {
		record, err := scanWorkflowPackage(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// InstallWorkflowPackage retains one exact portable definition. It neither creates
// live instances nor installs its embedded dependencies in other catalogs.
func (s *Store) InstallWorkflowPackage(ctx context.Context, definition model.WorkflowDefinition) (WorkflowPackageRecord, bool, error) {
	digest, err := model.WorkflowDigest(definition)
	if err != nil {
		return WorkflowPackageRecord{}, false, err
	}
	body, err := json.Marshal(definition)
	if err != nil {
		return WorkflowPackageRecord{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkflowPackageRecord{}, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075,6)`); err != nil {
		return WorkflowPackageRecord{}, false, err
	}
	existing, err := scanWorkflowPackage(tx.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM workflow_packages WHERE id=$1 AND version=$2`, definition.ID, definition.Version))
	if err == nil {
		if existing.Digest != digest {
			return WorkflowPackageRecord{}, false, ErrWorkflowPackageConflict
		}
		if err = tx.Commit(); err != nil {
			return WorkflowPackageRecord{}, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return WorkflowPackageRecord{}, false, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM workflow_packages`).Scan(&count); err != nil {
		return WorkflowPackageRecord{}, false, err
	}
	if count >= MaxWorkflowPackageVersions {
		return WorkflowPackageRecord{}, false, ErrWorkflowCatalogFull
	}
	installed := time.Now().UTC().Truncate(time.Microsecond)
	if _, err = tx.ExecContext(ctx, `INSERT INTO workflow_packages(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)`, definition.ID, definition.Version, digest, string(body), installed); err != nil {
		return WorkflowPackageRecord{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return WorkflowPackageRecord{}, false, err
	}
	return WorkflowPackageRecord{Workflow: definition, Digest: digest, InstalledAt: installed.Format(time.RFC3339Nano)}, true, nil
}

func (s *Store) WorkflowPackage(ctx context.Context, id, version, digest string) (WorkflowPackageRecord, error) {
	record, err := scanWorkflowPackage(s.db.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM workflow_packages WHERE id=$1 AND version=$2 AND digest=$3`, id, version, digest))
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	return record, err
}
