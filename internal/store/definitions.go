package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/system"
)

var ErrDefinitionConflict = errors.New("definition version already contains different content; install a new version")
var ErrDefinitionCapacity = errors.New("definition catalog is full (200 versions)")

type DefinitionValidationError struct{ Issues []model.ValidationIssue }

func (e *DefinitionValidationError) Error() string {
	return "definition has unresolved or invalid aspects"
}

type DefinitionRecord struct {
	Definition system.ComponentDefinition `json:"definition"`
	Digest     string                     `json:"digest"`
}

func (s *Store) Definitions(ctx context.Context) ([]DefinitionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,version,body,digest FROM component_definitions ORDER BY id COLLATE "C",version COLLATE "C"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DefinitionRecord{}
	for rows.Next() {
		var id, version string
		var body []byte
		var record DefinitionRecord
		if err = rows.Scan(&id, &version, &body, &record.Digest); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(body, &record.Definition); err != nil {
			return nil, err
		}
		if id != record.Definition.ID || version != record.Definition.Version {
			return nil, errors.New("definition identity mismatch")
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
func (s *Store) InstallDefinition(ctx context.Context, d system.ComponentDefinition) (DefinitionRecord, error) {
	catalog, err := s.SystemCatalog(ctx)
	if err != nil {
		return DefinitionRecord{}, err
	}
	checked, err := s.checkDefinition(ctx, d, catalog)
	if err != nil {
		return DefinitionRecord{}, err
	}
	if !checked.Valid {
		return DefinitionRecord{}, &DefinitionValidationError{checked.Issues}
	}
	digest, err := system.DefinitionDigest(d)
	if err != nil {
		return DefinitionRecord{}, err
	}
	body, err := json.Marshal(d)
	if err != nil {
		return DefinitionRecord{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DefinitionRecord{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1279477075, 20)`); err != nil {
		return DefinitionRecord{}, err
	}
	var previous string
	var previousBody []byte
	err = tx.QueryRowContext(ctx, `SELECT digest,body FROM component_definitions WHERE id=$1 AND version=$2`, d.ID, d.Version).Scan(&previous, &previousBody)
	if err == nil {
		var stored system.ComponentDefinition
		if err := json.Unmarshal(previousBody, &stored); err != nil {
			return DefinitionRecord{}, err
		}
		actual, err := system.DefinitionDigest(stored)
		if err != nil {
			return DefinitionRecord{}, err
		}
		if previous != digest || actual != digest {
			return DefinitionRecord{}, ErrDefinitionConflict
		}
		return DefinitionRecord{d, digest}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return DefinitionRecord{}, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM component_definitions`).Scan(&count); err != nil {
		return DefinitionRecord{}, err
	}
	if count >= 200 {
		return DefinitionRecord{}, ErrDefinitionCapacity
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO component_definitions(id,version,digest,body) VALUES($1,$2,$3,$4)`, d.ID, d.Version, digest, string(body)); err != nil {
		return DefinitionRecord{}, err
	}
	if err = tx.Commit(); err != nil {
		return DefinitionRecord{}, err
	}
	return DefinitionRecord{d, digest}, nil
}

// AspectEditor returns an exact installed artifact in the existing editor's
// import format. Editing it creates a draft; it never rewrites an installed pin.
func (s *Store) AspectEditor(ctx context.Context, a system.AspectRef) (map[string]any, error) {
	d := system.ComponentDefinition{SchemaVersion: 1, ID: "editor.request", Version: "1.0.0", Name: "Editor request", Aspects: []system.AspectRef{a}}
	catalog, err := s.SystemCatalog(ctx)
	if err != nil {
		return nil, err
	}
	checked := system.ResolveDefinition(d, catalog)
	if !checked.Valid {
		return nil, &DefinitionValidationError{checked.Issues}
	}
	switch a.Kind {
	case "component":
		bundle, err := s.ComponentBundle(ctx, model.ComponentRef{PackageID: a.ID, Version: a.Version, Digest: a.Digest})
		if err != nil {
			return nil, err
		}
		return map[string]any{"workspace": "components", "body": map[string]any{"bundle": bundle}}, nil
	case "data":
		r, err := s.DataContract(ctx, a.ID, a.Version, a.Digest)
		if err != nil {
			return nil, err
		}
		return map[string]any{"workspace": "data", "body": map[string]any{"contract": model.DataContractLock{Contract: r.Contract, Digest: r.Digest}}}, nil
	case "state":
		r, err := s.StateMachine(ctx, a.ID, a.Version, a.Digest)
		if err != nil {
			return nil, err
		}
		return map[string]any{"workspace": "state", "body": map[string]any{"machine": model.StateMachineLock{Machine: r.Machine, Digest: r.Digest}}}, nil
	case "messages":
		r, err := s.DeliveryModel(ctx, a.ID, a.Version, a.Digest)
		if err != nil {
			return nil, err
		}
		return map[string]any{"workspace": "messages", "body": map[string]any{"model": model.DeliveryModelLock{Model: r.Model, Digest: r.Digest}}}, nil
	case "workflow":
		r, err := s.WorkflowPackage(ctx, a.ID, a.Version, a.Digest)
		if err != nil {
			return nil, err
		}
		return map[string]any{"workspace": "workflows", "body": map[string]any{"workflow": model.WorkflowLock{Workflow: r.Workflow, Digest: r.Digest}}}, nil
	}
	return nil, fmt.Errorf("unsupported aspect kind")
}
