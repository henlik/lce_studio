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

var ErrPureExecution = errors.New("model execution failed")

type ExecutionPreparation struct {
	Plan       systemrun.Plan    `json:"plan"`
	Resolution system.Resolution `json:"resolution"`
	DesignHash string            `json:"designHash"`
}
type SystemExecution struct {
	SnapshotVersion int                 `json:"snapshotVersion,omitempty"`
	PlanHash        string              `json:"planHash,omitempty"`
	ReportHash      string              `json:"reportHash,omitempty"`
	Scenario        *systemrun.Scenario `json:"scenario,omitempty"`
	ID              string              `json:"id"`
	CreatedAt       string              `json:"createdAt"`
	Design          system.SystemDesign `json:"design"`
	Inputs          []systemrun.Inputs  `json:"inputs"`
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
		add("deploymentPlans", "Model execution does not support deployment plans; keep deployment simulation separate")
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
	providers := map[string]system.ExecutionProvider{}
	usedProviders := map[string]bool{}
	for _, p := range d.ExecutionProviders {
		providers[p.ID] = p
	}
	for i, c := range d.Components {
		path := fmt.Sprintf("components[%d]", i)
		usedBindings := map[string]bool{}
		entries := map[string]string{}
		usedEntries := map[string]bool{}
		for _, entry := range c.StateEntries {
			entries[entry.Slot] = entry.Command
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
					if a.Kind == "state" {
						requirement := "state/" + a.Slot
						provider := system.ExecutionProvider{}
						for _, binding := range c.Bindings {
							if binding.Requirement == requirement {
								provider = providers[binding.ResourceID]
								usedBindings[requirement] = true
							}
						}
						if provider.ID == "" {
							add(path+".bindings", "Bind "+requirement+" to a modeled-state@1.0.0 execution provider")
							continue
						}
						usedProviders[provider.ID] = true
						machine, err := s.StateMachine(ctx, a.ID, a.Version, a.Digest)
						if err != nil {
							return prepared, err
						}
						task := systemrun.Task{ID: systemrun.Key(c.ID, a.Slot), ComponentID: c.ID, Slot: a.Slot, Name: c.Name + " / " + a.Slot, State: &systemrun.StateTask{EntryCommand: entries[a.Slot], Machine: model.StateMachineLock{Machine: machine.Machine, Digest: machine.Digest}, Provider: provider}}
						usedEntries[a.Slot] = true
						if _, ok := systemrun.EntryCommand(task); !ok {
							add(path+".stateEntries", "Select an existing graph entry command for state slot "+a.Slot)
						}
						encoded, err := json.Marshal(task)
						if err != nil {
							return prepared, err
						}
						planBytes += len(encoded)
						if planBytes > systemrun.MaxBytes {
							return prepared, &systemrun.Invalid{Issues: []model.ValidationIssue{{Path: path, Message: "Portable task definitions exceed 4 MiB"}}}
						}
						tasks = append(tasks, task)
						count++
						continue
					}
					if a.Kind != "component" {
						add(fmt.Sprintf("%s.definition.aspects[%d]", path, j), "Message and workflow aspects need execution adapters; they are not ignored")
						continue
					}
					count++
					if err := addTask(c.ID, a.Slot, c.Name+" / "+a.Slot, a.ID, a.Version, a.Digest, path+".definition"); err != nil {
						return prepared, err
					}
				}
				if count == 0 {
					add(path+".definition", "At least one behavior or state aspect is required")
				}
				for _, l := range def.Links {
					if !executionInterface(tasks, systemrun.Key(c.ID, l.Source.Slot), l.Source.Interface, false) || !executionInterface(tasks, systemrun.Key(c.ID, l.Target.Slot), l.Target.Interface, true) {
						add(path+".definition.links", "Link must use the selected behavior or state graph-entry interface")
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
		for j, binding := range c.Bindings {
			if !usedBindings[binding.Requirement] {
				add(fmt.Sprintf("%s.bindings[%d]", path, j), "Binding does not identify an executable state/<slot> requirement")
			}
		}
		for j, entry := range c.StateEntries {
			if !usedEntries[entry.Slot] {
				add(fmt.Sprintf("%s.stateEntries[%d]", path, j), "Entry selection must identify an owned state aspect")
			}
		}
	}
	for i, p := range d.ExecutionProviders {
		if !usedProviders[p.ID] {
			add(fmt.Sprintf("executionProviders[%d]", i), "Provider is unused by executable state aspects")
		}
	}
	for i, c := range d.Connections {
		path := fmt.Sprintf("connections[%d]", i)
		if c.Kind != "data" {
			add(path+".kind", "Model execution supports data links only; request/event semantics require adapters")
		}
		if c.SourcePort == nil || c.TargetPort == nil {
			add(path, "Bind both typed endpoints before executing")
			continue
		}
		if !executionInterface(tasks, systemrun.Key(c.Source, c.SourcePort.Slot), c.SourcePort.Interface, false) || !executionInterface(tasks, systemrun.Key(c.Target, c.TargetPort.Slot), c.TargetPort.Interface, true) {
			add(path, "Link must use the selected behavior or state graph-entry interface")
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
	return s.executeSystem(ctx, id, d, inputs, nil)
}
func (s *Store) ExecuteSystemScenario(ctx context.Context, id string, d system.SystemDesign, scenario systemrun.Scenario) (SystemExecution, error) {
	return s.executeSystem(ctx, id, d, nil, &scenario)
}
func (s *Store) executeSystem(ctx context.Context, id string, d system.SystemDesign, inputs []systemrun.Inputs, scenario *systemrun.Scenario) (SystemExecution, error) {
	p, err := s.PrepareSystemExecution(ctx, d)
	if err != nil {
		return SystemExecution{}, err
	}
	var report systemrun.Report
	if scenario != nil {
		report, err = systemrun.RunScenario(ctx, p.Plan, *scenario)
	} else {
		report, err = systemrun.Run(ctx, p.Plan, inputs)
	}
	if err != nil {
		var invalid *systemrun.Invalid
		if errors.As(err, &invalid) || ctx.Err() != nil {
			return SystemExecution{}, err
		}
		return SystemExecution{}, fmt.Errorf("%w: %v", ErrPureExecution, err)
	}
	e := SystemExecution{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Design: d, Inputs: inputs, ExecutionPreparation: p, InputHash: executionHash(inputs), Report: report}
	e.SnapshotVersion, e.PlanHash, e.ReportHash = 1, executionHash(p.Plan), executionHash(report)
	if scenario != nil {
		e.Scenario = scenario
		e.InputHash = executionHash(scenario)
	}
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,body->'design'->>'name',json_array_length(body->'plan'->'tasks') FROM system_executions WHERE system_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50`, systemID)
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

func executionInterface(tasks []systemrun.Task, id, name string, input bool) bool {
	suffix := "outputs"
	if input {
		suffix = "inputs"
	}
	for _, task := range tasks {
		if task.ID != id {
			continue
		}
		if task.State == nil {
			return name == suffix
		}
		command, ok := systemrun.EntryCommand(task)
		if !ok {
			return false
		}
		return name == "command/"+command.Name+"/"+suffix
	}
	return false
}
