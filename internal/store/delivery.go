package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"software-studio/internal/model"
)

const MaxDeliveryModelVersions = 200

var (
	ErrDeliveryModelConflict = errors.New("delivery model version already contains different content")
	ErrDeliveryCatalogFull   = errors.New("delivery model catalog contains the maximum number of installed versions")
)

type DeliveryModelRecord struct {
	Model       model.DeliveryModel `json:"model"`
	Digest      string              `json:"digest"`
	InstalledAt string              `json:"installedAt"`
}

func scanDeliveryModel(row rowScanner) (DeliveryModelRecord, error) {
	var record DeliveryModelRecord
	var body []byte
	var installed time.Time
	if err := row.Scan(&body, &record.Digest, &installed); err != nil {
		return record, err
	}
	if err := json.Unmarshal(body, &record.Model); err != nil {
		return record, err
	}
	record.InstalledAt = installed.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func (s *Store) DeliveryModels(ctx context.Context) ([]DeliveryModelRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body,digest,installed_at FROM delivery_models ORDER BY id COLLATE "C",version COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]DeliveryModelRecord, 0)
	for rows.Next() {
		record, err := scanDeliveryModel(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// InstallDeliveryModel retains one exact portable definition. It neither creates
// live instances nor installs its embedded dependencies in other catalogs.
func (s *Store) InstallDeliveryModel(ctx context.Context, definition model.DeliveryModel) (DeliveryModelRecord, bool, error) {
	digest, err := model.DeliveryModelDigest(definition)
	if err != nil {
		return DeliveryModelRecord{}, false, err
	}
	body, err := json.Marshal(definition)
	if err != nil {
		return DeliveryModelRecord{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DeliveryModelRecord{}, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075,5)`); err != nil {
		return DeliveryModelRecord{}, false, err
	}
	existing, err := scanDeliveryModel(tx.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM delivery_models WHERE id=$1 AND version=$2`, definition.ID, definition.Version))
	if err == nil {
		if existing.Digest != digest {
			return DeliveryModelRecord{}, false, ErrDeliveryModelConflict
		}
		if err = tx.Commit(); err != nil {
			return DeliveryModelRecord{}, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return DeliveryModelRecord{}, false, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM delivery_models`).Scan(&count); err != nil {
		return DeliveryModelRecord{}, false, err
	}
	if count >= MaxDeliveryModelVersions {
		return DeliveryModelRecord{}, false, ErrDeliveryCatalogFull
	}
	installed := time.Now().UTC().Truncate(time.Microsecond)
	if _, err = tx.ExecContext(ctx, `INSERT INTO delivery_models(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)`, definition.ID, definition.Version, digest, string(body), installed); err != nil {
		return DeliveryModelRecord{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return DeliveryModelRecord{}, false, err
	}
	return DeliveryModelRecord{Model: definition, Digest: digest, InstalledAt: installed.Format(time.RFC3339Nano)}, true, nil
}

func (s *Store) DeliveryModel(ctx context.Context, id, version, digest string) (DeliveryModelRecord, error) {
	record, err := scanDeliveryModel(s.db.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM delivery_models WHERE id=$1 AND version=$2 AND digest=$3`, id, version, digest))
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	return record, err
}
