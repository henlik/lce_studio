// Package store persists editable projects and immutable simulation snapshots.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

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

func Open(filename string) (*Store, error) {
	if filename != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("open database file: %w", err)
		}
		err = file.Chmod(0600)
		closeErr := file.Close()
		if err != nil {
			return nil, fmt.Errorf("protect database: %w", err)
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, err
	}
	// One connection keeps :memory: databases consistent and serializes writes.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	_, err = db.Exec(`
PRAGMA busy_timeout = 5000;
PRAGMA journal_mode = WAL;
CREATE TABLE IF NOT EXISTS projects (
 id TEXT PRIMARY KEY,
 body TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS runs (
 id TEXT PRIMARY KEY,
 project_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 body TEXT NOT NULL,
 summary TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS runs_project_created ON runs(project_id, created_at DESC);
`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SaveProject(ctx context.Context, p model.Project) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO projects(id,body,updated_at) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body,updated_at=excluded.updated_at`, p.ID, body, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) Project(ctx context.Context, id string) (model.Project, error) {
	var p model.Project
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM projects WHERE id=?`, id).Scan(&body)
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
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id)
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
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	summary := RunSummary{ID: r.ID, ProjectID: r.ProjectID, CreatedAt: r.CreatedAt, ModelHash: r.ModelHash, EngineVersion: r.EngineVersion, Seed: r.Seed, Metrics: r.Metrics, Verdicts: r.Verdicts, Events: []model.TraceEvent{}}
	brief, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO runs(id,project_id,created_at,body,summary) VALUES(?,?,?,?,?)`, r.ID, r.ProjectID, r.CreatedAt, body, brief)
	return err
}

func (s *Store) Run(ctx context.Context, id string) (model.Run, error) {
	var r model.Run
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM runs WHERE id=?`, id).Scan(&body)
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
		query += ` WHERE project_id=?`
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
