package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"strings"
	"time"
)

var ErrPureExecution = errors.New("pure execution failed")

type ExecutionPreparation struct {
	Plan       systemrun.Plan    `json:"plan"`
	Resolution system.Resolution `json:"resolution"`
	DesignHash string            `json:"designHash"`
}
type SystemExecution struct {
	ID        string              `json:"id"`
	CreatedAt string              `json:"createdAt"`
	Design    system.SystemDesign `json:"design"`
	Inputs    []systemrun.Inputs  `json:"inputs"`
	ExecutionPreparation
	InputHash string           `json:"inputHash"`
	Report    systemrun.Report `json:"report"`
}
type ExecutionSummary struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Name      string `json:"name"`
	TaskCount int    `json:"taskCount"`
}

func executionHash(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func (s *Store) PrepareSystemExecution(ctx context.Context, d system.SystemDesign) (ExecutionPreparation, error) {
	prepared := ExecutionPreparation{DesignHash: executionHash(d)}
	resolution, err := s.ResolveSystem(ctx, d)
	if err != nil {
		return prepared, err
	}
	prepared.Resolution = resolution
	if !resolution.Valid {
		return prepared, &systemrun.Invalid{Issues: resolution.Issues}
	}
	issues := []model.ValidationIssue{}
	add := func(path, message string) {
		issues = append(issues, model.ValidationIssue{Path: path, Message: message})
	}
	if len(d.DeploymentPlans) > 0 {
		add("deploymentPlans", "Pure execution does not support deployment plans; use a separate behavior-only design")
	}
	records, err := s.Definitions(ctx)
	if err != nil {
		return prepared, err
	}
	tasks := []systemrun.Task{}
	planBytes := 0
	links := []systemrun.Link{}
	addTask := func(component, slot, name, id, version, digest, path string) error {
		if strings.HasPrefix(id, "builtin.") {
			add(path, "Built-in conformance adapters are not pure behavior tasks")
			return nil
		}
		b, err := s.ComponentBundle(ctx, model.ComponentRef{PackageID: id, Version: version, Digest: digest})
		if errors.Is(err, ErrNotComposite) || errors.Is(err, ErrNotFound) {
			add(path, "Install a complete pure behavior bundle")
			return nil
		}
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(b)
		if err != nil {
			return err
		}
		planBytes += len(encoded)
		if planBytes > systemrun.MaxBytes {
			return &systemrun.Invalid{Issues: []model.ValidationIssue{{Path: path, Message: "Portable task bundles exceed 4 MiB"}}}
		}
		tasks = append(tasks, systemrun.Task{ID: systemrun.Key(component, slot), ComponentID: component, Slot: slot, Name: name, Bundle: b})
		return nil
	}
	for i, c := range d.Components {
		path := fmt.Sprintf("components[%d]", i)
		if len(c.Bindings) > 0 {
			add(path+".bindings", "Provider bindings are not supported by pure execution")
		}
		for j, a := range c.Artifacts {
			if a.Kind != "data" {
				add(fmt.Sprintf("%s.artifacts[%d]", path, j), "Only data attachments are allowed; compose executable behavior using owned aspect slots")
			}
		}
		if c.Definition.Kind == "definition" {
			for _, record := range records {
				def := record.Definition
				if def.ID != c.Definition.ID || def.Version != c.Definition.Version {
					continue
				}
				count := 0
				for j, a := range def.Aspects {
					if a.Kind == "data" {
						continue
					}
					if a.Kind != "component" {
						add(fmt.Sprintf("%s.definition.aspects[%d]", path, j), "State, message and workflow aspects need execution adapters; they are not ignored")
						continue
					}
					count++
					if err := addTask(c.ID, a.Slot, c.Name+" / "+a.Slot, a.ID, a.Version, a.Digest, path+".definition"); err != nil {
						return prepared, err
					}
				}
				if count == 0 {
					add(path+".definition", "At least one pure behavior aspect is required")
				}
				for _, l := range def.Links {
					if l.Source.Interface != "outputs" || l.Target.Interface != "inputs" {
						add(path+".definition.links", "Only behavior output-to-input links are executable")
					}
					links = append(links, systemrun.Link{ID: "internal/" + systemrun.Key(c.ID, l.ID), Source: systemrun.Key(c.ID, l.Source.Slot), Target: systemrun.Key(c.ID, l.Target.Slot), Output: l.Source.Port, Input: l.Target.Port})
				}
			}
		} else {
			digest := ""
			for _, r := range resolution.References {
				if r.Path == path+".definition" {
					digest = r.Digest
				}
			}
			if err := addTask(c.ID, "root", c.Name, c.Definition.ID, c.Definition.Version, digest, path+".definition"); err != nil {
				return prepared, err
			}
		}
	}
	for i, c := range d.Connections {
		path := fmt.Sprintf("connections[%d]", i)
		if c.Kind != "data" {
			add(path+".kind", "Pure execution supports data links only; request/event semantics require adapters")
		}
		if c.SourcePort == nil || c.TargetPort == nil {
			add(path, "Bind both typed endpoints before executing")
			continue
		}
		if c.SourcePort.Interface != "outputs" || c.TargetPort.Interface != "inputs" {
			add(path, "Only behavior output-to-input interfaces are executable")
		}
		links = append(links, systemrun.Link{ID: "system/" + c.ID, ConnectionID: c.ID, Source: systemrun.Key(c.Source, c.SourcePort.Slot), Target: systemrun.Key(c.Target, c.TargetPort.Slot), Output: c.SourcePort.Port, Input: c.TargetPort.Port})
	}
	if len(issues) > 0 {
		return prepared, &systemrun.Invalid{Issues: issues}
	}
	prepared.Plan, err = systemrun.Prepare(tasks, links)
	if err != nil {
		return prepared, err
	}
	b, err := json.Marshal(prepared)
	if err != nil {
		return prepared, err
	}
	if len(b) > systemrun.MaxBytes {
		return prepared, &systemrun.Invalid{Issues: []model.ValidationIssue{{Path: "components", Message: "Portable execution plan exceeds 4 MiB"}}}
	}
	return prepared, nil
}
func (s *Store) ExecuteSystem(ctx context.Context, id string, d system.SystemDesign, inputs []systemrun.Inputs) (SystemExecution, error) {
	p, err := s.PrepareSystemExecution(ctx, d)
	if err != nil {
		return SystemExecution{}, err
	}
	report, err := systemrun.Run(ctx, p.Plan, inputs)
	if err != nil {
		var invalid *systemrun.Invalid
		if errors.As(err, &invalid) || ctx.Err() != nil {
			return SystemExecution{}, err
		}
		return SystemExecution{}, fmt.Errorf("%w: %v", ErrPureExecution, err)
	}
	e := SystemExecution{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Design: d, Inputs: inputs, ExecutionPreparation: p, InputHash: executionHash(inputs), Report: report}
	b, err := json.Marshal(e)
	if err != nil {
		return SystemExecution{}, err
	}
	if len(b) > 8<<20 {
		return SystemExecution{}, fmt.Errorf("%w: execution snapshot exceeds 8 MiB", ErrPureExecution)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO system_executions(id,system_id,created_at,body) VALUES($1,$2,$3,$4)`, id, d.ID, e.CreatedAt, string(b))
	if err != nil {
		return SystemExecution{}, err
	}
	var frozen SystemExecution
	err = json.Unmarshal(b, &frozen)
	return frozen, err
}
func (s *Store) SystemExecution(ctx context.Context, id string) (SystemExecution, error) {
	var e SystemExecution
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM system_executions WHERE id=$1`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	if err != nil {
		return e, err
	}
	err = json.Unmarshal(b, &e)
	return e, err
}
func (s *Store) SystemExecutions(ctx context.Context, systemID string) ([]ExecutionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,body->'design'->>'name',json_array_length(body->'report'->'results') FROM system_executions WHERE system_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50`, systemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExecutionSummary{}
	for rows.Next() {
		var e ExecutionSummary
		var at time.Time
		if err := rows.Scan(&e.ID, &at, &e.Name, &e.TaskCount); err != nil {
			return nil, err
		}
		e.CreatedAt = at.UTC().Format(time.RFC3339Nano)
		out = append(out, e)
	}
	return out, rows.Err()
}
