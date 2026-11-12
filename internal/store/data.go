package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"software-studio/internal/model"
)

const MaxDataContractVersions = 200

var (
	ErrDataContractConflict = errors.New("data contract version already contains different content")
	ErrDataCatalogFull      = errors.New("data catalog contains the maximum number of installed versions")
)

type DataContractRecord struct {
	Contract    model.DataContract `json:"contract"`
	Digest      string             `json:"digest"`
	InstalledAt string             `json:"installedAt"`
}

func scanDataContract(row rowScanner) (DataContractRecord, error) {
	var record DataContractRecord
	var body []byte
	var installed time.Time
	if err := row.Scan(&body, &record.Digest, &installed); err != nil {
		return record, err
	}
	if err := json.Unmarshal(body, &record.Contract); err != nil {
		return record, err
	}
	record.InstalledAt = installed.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func (s *Store) DataContracts(ctx context.Context) ([]DataContractRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body,digest,installed_at FROM data_contracts ORDER BY id COLLATE "C",version COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]DataContractRecord, 0)
	for rows.Next() {
		record, err := scanDataContract(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// InstallDataContract accepts a previously validated contract. Immutable content
// and capacity are serialized across app processes independently of components.
func (s *Store) InstallDataContract(ctx context.Context, contract model.DataContract) (DataContractRecord, bool, error) {
	digest, err := model.DataContractDigest(contract)
	if err != nil {
		return DataContractRecord{}, false, err
	}
	body, err := json.Marshal(contract)
	if err != nil {
		return DataContractRecord{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DataContractRecord{}, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075,3)`); err != nil {
		return DataContractRecord{}, false, err
	}
	existing, err := scanDataContract(tx.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM data_contracts WHERE id=$1 AND version=$2`, contract.ID, contract.Version))
	if err == nil {
		if existing.Digest != digest {
			return DataContractRecord{}, false, ErrDataContractConflict
		}
		if err = tx.Commit(); err != nil {
			return DataContractRecord{}, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return DataContractRecord{}, false, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM data_contracts`).Scan(&count); err != nil {
		return DataContractRecord{}, false, err
	}
	if count >= MaxDataContractVersions {
		return DataContractRecord{}, false, ErrDataCatalogFull
	}
	installed := time.Now().UTC().Truncate(time.Microsecond)
	if _, err = tx.ExecContext(ctx, `INSERT INTO data_contracts(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)`, contract.ID, contract.Version, digest, string(body), installed); err != nil {
		return DataContractRecord{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return DataContractRecord{}, false, err
	}
	return DataContractRecord{Contract: contract, Digest: digest, InstalledAt: installed.Format(time.RFC3339Nano)}, true, nil
}

func (s *Store) DataContract(ctx context.Context, id, version, digest string) (DataContractRecord, error) {
	record, err := scanDataContract(s.db.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM data_contracts WHERE id=$1 AND version=$2 AND digest=$3`, id, version, digest))
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	return record, err
}
