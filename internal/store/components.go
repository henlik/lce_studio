package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"software-studio/internal/model"
)

const MaxComponentVersions = 200

var (
	ErrComponentConflict = errors.New("component package version already contains different content")
	ErrCatalogFull       = errors.New("component catalog contains the maximum number of installed versions")
)

// ComponentRecord is an immutable package version together with its installation
// evidence. The timestamp is not part of the package content digest.
type ComponentRecord struct {
	Package     model.ComponentPackage `json:"package"`
	Digest      string                 `json:"digest"`
	InstalledAt string                 `json:"installedAt"`
}

type rowScanner interface{ Scan(...any) error }

func scanComponent(row rowScanner) (ComponentRecord, error) {
	var record ComponentRecord
	var body []byte
	var installed time.Time
	if err := row.Scan(&body, &record.Digest, &installed); err != nil {
		return record, err
	}
	if err := json.Unmarshal(body, &record.Package); err != nil {
		return record, err
	}
	record.InstalledAt = installed.UTC().Format(time.RFC3339Nano)
	return record, nil
}

// Components returns the complete bounded catalog, including every installed
// version. Bytewise ordering is stable across PostgreSQL locale configurations.
func (s *Store) Components(ctx context.Context) ([]ComponentRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body,digest,installed_at FROM component_packages ORDER BY id COLLATE "C", version COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]ComponentRecord, 0)
	for rows.Next() {
		record, err := scanComponent(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// InstallComponent persists a previously validated package version. A matching
// digest is idempotent, while different content at the same ID/version is never
// substituted. The returned bool reports a new installation.
func (s *Store) InstallComponent(ctx context.Context, p model.ComponentPackage) (ComponentRecord, bool, error) {
	digest, err := model.ComponentDigest(p)
	if err != nil {
		return ComponentRecord{}, false, err
	}
	body, err := json.Marshal(p)
	if err != nil {
		return ComponentRecord{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ComponentRecord{}, false, err
	}
	defer tx.Rollback()
	// Serialize installations across processes, covering both immutability and
	// the global capacity check. The lock is automatically released on rollback.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075, 2)`); err != nil {
		return ComponentRecord{}, false, err
	}
	existing, err := scanComponent(tx.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM component_packages WHERE id=$1 AND version=$2`, p.ID, p.Version))
	if err == nil {
		if existing.Digest != digest {
			return ComponentRecord{}, false, ErrComponentConflict
		}
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ComponentRecord{}, false, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM component_packages`).Scan(&count); err != nil {
		return ComponentRecord{}, false, err
	}
	if count >= MaxComponentVersions {
		return ComponentRecord{}, false, ErrCatalogFull
	}
	// PostgreSQL stores microsecond precision. Use it in the response as well so
	// subsequent reads and identical reinstalls reproduce the same record.
	installed := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := tx.ExecContext(ctx, `INSERT INTO component_packages(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)`, p.ID, p.Version, digest, string(body), installed); err != nil {
		return ComponentRecord{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ComponentRecord{}, false, err
	}
	return ComponentRecord{Package: p, Digest: digest, InstalledAt: installed.Format(time.RFC3339Nano)}, true, nil
}
