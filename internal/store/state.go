package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"software-studio/internal/model"
)

const MaxStateMachineVersions = 200

var (
	ErrStateMachineConflict = errors.New("state machine version already contains different content")
	ErrStateCatalogFull     = errors.New("state machine catalog contains the maximum number of installed versions")
)

type StateMachineRecord struct {
	Machine     model.StateMachine `json:"machine"`
	Digest      string             `json:"digest"`
	InstalledAt string             `json:"installedAt"`
}

func scanStateMachine(row rowScanner) (StateMachineRecord, error) {
	var record StateMachineRecord
	var body []byte
	var installed time.Time
	if err := row.Scan(&body, &record.Digest, &installed); err != nil {
		return record, err
	}
	if err := json.Unmarshal(body, &record.Machine); err != nil {
		return record, err
	}
	record.InstalledAt = installed.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func (s *Store) StateMachines(ctx context.Context) ([]StateMachineRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body,digest,installed_at FROM state_machines ORDER BY id COLLATE "C",version COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]StateMachineRecord, 0)
	for rows.Next() {
		record, err := scanStateMachine(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// InstallStateMachine retains one exact portable definition. It neither creates
// live instances nor installs its embedded dependencies in other catalogs.
func (s *Store) InstallStateMachine(ctx context.Context, machine model.StateMachine) (StateMachineRecord, bool, error) {
	digest, err := model.StateMachineDigest(machine)
	if err != nil {
		return StateMachineRecord{}, false, err
	}
	body, err := json.Marshal(machine)
	if err != nil {
		return StateMachineRecord{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return StateMachineRecord{}, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075,4)`); err != nil {
		return StateMachineRecord{}, false, err
	}
	existing, err := scanStateMachine(tx.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM state_machines WHERE id=$1 AND version=$2`, machine.ID, machine.Version))
	if err == nil {
		if existing.Digest != digest {
			return StateMachineRecord{}, false, ErrStateMachineConflict
		}
		if err = tx.Commit(); err != nil {
			return StateMachineRecord{}, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return StateMachineRecord{}, false, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM state_machines`).Scan(&count); err != nil {
		return StateMachineRecord{}, false, err
	}
	if count >= MaxStateMachineVersions {
		return StateMachineRecord{}, false, ErrStateCatalogFull
	}
	installed := time.Now().UTC().Truncate(time.Microsecond)
	if _, err = tx.ExecContext(ctx, `INSERT INTO state_machines(id,version,digest,body,installed_at) VALUES($1,$2,$3,$4,$5)`, machine.ID, machine.Version, digest, string(body), installed); err != nil {
		return StateMachineRecord{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return StateMachineRecord{}, false, err
	}
	return StateMachineRecord{Machine: machine, Digest: digest, InstalledAt: installed.Format(time.RFC3339Nano)}, true, nil
}

func (s *Store) StateMachine(ctx context.Context, id, version, digest string) (StateMachineRecord, error) {
	record, err := scanStateMachine(s.db.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM state_machines WHERE id=$1 AND version=$2 AND digest=$3`, id, version, digest))
	if errors.Is(err, sql.ErrNoRows) {
		return record, ErrNotFound
	}
	return record, err
}
