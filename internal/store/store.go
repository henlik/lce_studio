// Package store persists editable projects and immutable simulation snapshots.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"software-studio/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store struct{ db *sql.DB }

// RunSummary deliberately omits the potentially large trace and project. The
// detail endpoint returns the exact snapshot used by the engine.
type RunSummary struct {
	ID            string             `json:"id"`
	ProjectID     string             `json:"projectId"`
	CreatedAt     string             `json:"createdAt"`
	ModelHash     string             `json:"modelHash"`
	EngineVersion string             `json:"engineVersion"`
	Seed          int64              `json:"seed"`
	Metrics       model.Metrics      `json:"metrics"`
	Verdicts      []model.Verdict    `json:"verdicts"`
	Events        []model.TraceEvent `json:"events"`
}

// Open connects to PostgreSQL and initializes the schema before serving requests.
// DATABASE_URL is explicit: there is no embedded database or credential fallback.
func Open(databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("DATABASE_URL is required (PostgreSQL connection string)")
	}
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		// Parser errors can echo the connection string, including its password.
		return nil, errors.New("invalid PostgreSQL DATABASE_URL")
	}
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := initialize(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize PostgreSQL schema: %w", err)
	}
	return &Store{db: db}, nil
}

func initialize(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize bootstrap across processes; all schema changes commit together.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075, 1)`); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS projects (
 id TEXT PRIMARY KEY,
 body JSON NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS runs (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 body JSON NOT NULL,
 summary JSON NOT NULL
);
CREATE INDEX IF NOT EXISTS runs_project_created ON runs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS runs_created ON runs(created_at DESC, id DESC);
CREATE TABLE IF NOT EXISTS component_packages (
 id TEXT NOT NULL,
 version TEXT NOT NULL,
 digest TEXT NOT NULL,
 body JSON NOT NULL,
 installed_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(id, version)
);
`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) SaveProject(ctx context.Context, p model.Project) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO projects(id,body,updated_at) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET body=excluded.body,updated_at=excluded.updated_at`, p.ID, string(body), time.Now().UTC())
	return err
}

func (s *Store) Project(ctx context.Context, id string) (model.Project, error) {
	var p model.Project
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM projects WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(body, &p)
	return p, err
}

func (s *Store) Projects(ctx context.Context) ([]model.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body FROM projects ORDER BY updated_at DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Project, 0)
	for rows.Next() {
		var body []byte
		var p model.Project
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	// Run history remains available because every run owns a full model snapshot.
	return nil
}

func (s *Store) SaveRun(ctx context.Context, r model.Run) error {
	createdAt, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid run creation time: %w", err)
	}
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	summary := RunSummary{ID: r.ID, ProjectID: r.ProjectID, CreatedAt: r.CreatedAt, ModelHash: r.ModelHash, EngineVersion: r.EngineVersion, Seed: r.Seed, Metrics: r.Metrics, Verdicts: r.Verdicts, Events: []model.TraceEvent{}}
	brief, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO runs(id,project_id,created_at,body,summary) VALUES($1,$2,$3,$4,$5)`, r.ID, r.ProjectID, createdAt, string(body), string(brief))
	return err
}

func (s *Store) Run(ctx context.Context, id string) (model.Run, error) {
	var r model.Run
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM runs WHERE id=$1`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(body, &r)
	return r, err
}

// Runs returns the most recent 50 summaries. Detail snapshots are never trimmed.
func (s *Store) Runs(ctx context.Context, projectID string) ([]RunSummary, error) {
	query := `SELECT summary FROM runs`
	args := []any{}
	if projectID != "" {
		query += ` WHERE project_id=$1`
		args = append(args, projectID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT 50`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]RunSummary, 0)
	for rows.Next() {
		var body []byte
		var r RunSummary
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
