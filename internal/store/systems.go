package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"software-studio/internal/conformance"
	"software-studio/internal/system"
)

// SystemEvidence owns its design snapshot; later edits cannot rewrite a result.
type SystemEvidence struct {
	ID           string              `json:"id"`
	CreatedAt    string              `json:"createdAt"`
	DesignHash   string              `json:"designHash"`
	ContractHash string              `json:"contractHash"`
	Design       system.SystemDesign `json:"design"`
	Report       conformance.Report  `json:"report"`
	Resolution   *system.Resolution  `json:"resolution,omitempty"`
}

func initializeSystems(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
 CREATE TABLE IF NOT EXISTS system_designs (
 id TEXT PRIMARY KEY, body JSON NOT NULL, updated_at TIMESTAMPTZ NOT NULL
 );
 CREATE TABLE IF NOT EXISTS component_definitions (
 id TEXT NOT NULL,version TEXT NOT NULL,digest TEXT NOT NULL,body JSON NOT NULL,
 PRIMARY KEY(id,version)
 );
 CREATE TABLE IF NOT EXISTS system_executions (id TEXT PRIMARY KEY,system_id TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL,body JSON NOT NULL);
 CREATE INDEX IF NOT EXISTS system_executions_created ON system_executions(system_id,created_at DESC,id DESC);
 CREATE TABLE IF NOT EXISTS system_evidence (
 id TEXT PRIMARY KEY, system_id TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, body JSON NOT NULL
 );
 CREATE INDEX IF NOT EXISTS system_evidence_created ON system_evidence(system_id,created_at DESC,id DESC);
 `)
	return err
}

func (s *Store) SaveSystem(ctx context.Context, design system.SystemDesign) error {
	body, err := json.Marshal(design)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO system_designs(id,body,updated_at) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET body=excluded.body,updated_at=excluded.updated_at`, design.ID, string(body), time.Now().UTC())
	return err
}

func (s *Store) System(ctx context.Context, id string) (system.SystemDesign, error) {
	var design system.SystemDesign
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM system_designs WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return design, ErrNotFound
	}
	if err != nil {
		return design, err
	}
	err = json.Unmarshal(body, &design)
	return design, err
}

func (s *Store) Systems(ctx context.Context) ([]system.SystemDesign, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body FROM system_designs ORDER BY updated_at DESC,id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]system.SystemDesign, 0)
	for rows.Next() {
		var body []byte
		var design system.SystemDesign
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &design); err != nil {
			return nil, err
		}
		result = append(result, design)
	}
	return result, rows.Err()
}

// RunSystemConformance uses only the configured server pool, never a client DSN.
func (s *Store) RunSystemConformance(ctx context.Context, design system.SystemDesign) (conformance.Report, error) {
	return conformance.Run(ctx, s.db, design)
}

func NewSystemEvidence(id string, design system.SystemDesign, report conformance.Report) (SystemEvidence, error) {
	e := SystemEvidence{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Design: design, Report: report}
	body, err := json.Marshal(design)
	if err != nil {
		return e, err
	}
	hash := sha256.Sum256(body)
	e.DesignHash = hex.EncodeToString(hash[:])
	body, err = json.Marshal(design.Contract)
	if err != nil {
		return e, err
	}
	hash = sha256.Sum256(body)
	e.ContractHash = hex.EncodeToString(hash[:])
	// Detach every nested slice so caller edits cannot change the snapshot.
	body, err = json.Marshal(e)
	if err != nil {
		return e, err
	}
	var snapshot SystemEvidence
	err = json.Unmarshal(body, &snapshot)
	return snapshot, err
}
func (s *Store) SaveSystemEvidence(ctx context.Context, e SystemEvidence) error {
	at, err := time.Parse(time.RFC3339Nano, e.CreatedAt)
	if err != nil {
		return err
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO system_evidence(id,system_id,created_at,body) VALUES($1,$2,$3,$4)`, e.ID, e.Design.ID, at, string(body))
	return err
}
func (s *Store) SystemEvidence(ctx context.Context, id string) (SystemEvidence, error) {
	var e SystemEvidence
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM system_evidence WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	if err != nil {
		return e, err
	}
	err = json.Unmarshal(body, &e)
	return e, err
}
func (s *Store) SystemEvidenceList(ctx context.Context, systemID string) ([]SystemEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body FROM system_evidence WHERE ($1='' OR system_id=$1) ORDER BY created_at DESC,id DESC LIMIT 50`, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]SystemEvidence, 0)
	for rows.Next() {
		var body []byte
		var e SystemEvidence
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &e); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
