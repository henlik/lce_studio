// Package assembly shares design resolution and execution planning between the catalog and portable source projects.
package assembly

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"software-studio/internal/systemrun"
	"strings"
)

var ErrMissing = errors.New("missing or non-executable artifact")

func hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type DefinitionRecord struct{ Definition system.ComponentDefinition }
type Source interface {
	ResolveSystem(context.Context, system.SystemDesign) (system.Resolution, error)
	Definitions(context.Context) ([]DefinitionRecord, error)
	ComponentBundle(context.Context, model.ComponentRef) (model.ComponentBundle, error)
	ComponentPackage(context.Context, string, string) (model.ComponentPackage, error)
	StateMachine(context.Context, string, string, string) (model.StateMachineLock, error)
	DeliveryModel(context.Context, string, string, string) (model.DeliveryModelLock, error)
	WorkflowPackage(context.Context, string, string, string) (model.WorkflowLock, error)
}
type ExecutionPreparation struct {
	ProviderChecks []system.ProviderCheck `json:"providerChecks,omitempty"`
	Plan           systemrun.Plan         `json:"plan"`
	Resolution     system.Resolution      `json:"resolution"`
	DesignHash     string                 `json:"designHash"`
}

func Prepare(ctx context.Context, s Source, d system.SystemDesign) (ExecutionPreparation, error) {
	prepared := ExecutionPreparation{DesignHash: hash(d)}
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
	channels := []systemrun.MessageChannel{}
	workflows := []systemrun.WorkflowTask{}
	planBytes := 0
	links := []systemrun.Link{}
	addTask := func(component, slot, name, id, version, digest, path string) error {
		if strings.HasPrefix(id, "builtin.") {
			add(path, "Built-in conformance adapters are not pure behavior tasks")
			return nil
		}
		b, err := s.ComponentBundle(ctx, model.ComponentRef{PackageID: id, Version: version, Digest: digest})
		if errors.Is(err, ErrMissing) {
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
		usedOwners := map[int]bool{}
		usedWorkflowOwners := map[int]bool{}
		usedWorkflowRoutes := map[int]bool{}
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
					if a.Kind == "workflow" {
						requirement := "workflow/" + a.Slot
						provider := system.ExecutionProvider{}
						for _, b := range c.Bindings {
							if b.Requirement == requirement {
								provider = providers[b.ResourceID]
								usedBindings[requirement] = true
							}
						}
						if provider.ID == "" {
							add(path+".bindings", "Bind "+requirement+" to modeled-workflow@1.0.0")
							continue
						}
						usedProviders[provider.ID] = true
						record, err := s.WorkflowPackage(ctx, a.ID, a.Version, a.Digest)
						if err != nil {
							return prepared, err
						}
						f := systemrun.WorkflowTask{ID: systemrun.Key(c.ID, a.Slot), Name: c.Name + " / " + a.Slot, Lock: model.WorkflowLock{Workflow: record.Workflow, Digest: record.Digest}, Provider: provider}
						for k, o := range c.WorkflowOwners {
							if o.Slot == a.Slot {
								usedWorkflowOwners[k] = true
								f.Owners = append(f.Owners, systemrun.WorkflowOwner{OwnerID: o.OwnerID, TaskID: systemrun.Key(o.ComponentID, o.StateSlot)})
							}
						}
						for k, r := range c.WorkflowMessages {
							if r.Slot == a.Slot {
								usedWorkflowRoutes[k] = true
								f.Routes = append(f.Routes, systemrun.WorkflowRoute{NodeID: r.NodeID, ChannelID: systemrun.Key(r.ChannelComponentID, r.ChannelSlot), RouteID: r.RouteID, SourceNodeID: r.SourceNodeID, Response: r.Response})
							}
						}
						workflows = append(workflows, f)
						count++
						continue
					}
					if a.Kind == "messages" {
						requirement := "messages/" + a.Slot
						provider := system.ExecutionProvider{}
						for _, b := range c.Bindings {
							if b.Requirement == requirement {
								provider = providers[b.ResourceID]
								usedBindings[requirement] = true
							}
						}
						if provider.ID == "" {
							add(path+".bindings", "Bind "+requirement+" to modeled-broker@1.0.0")
							continue
						}
						usedProviders[provider.ID] = true
						record, err := s.DeliveryModel(ctx, a.ID, a.Version, a.Digest)
						if err != nil {
							return prepared, err
						}
						channel := systemrun.MessageChannel{ID: systemrun.Key(c.ID, a.Slot), Model: model.DeliveryModelLock{Model: record.Model, Digest: record.Digest}, Provider: provider}
						for k, o := range c.MessageOwners {
							if o.Slot == a.Slot {
								usedOwners[k] = true
								channel.Owners = append(channel.Owners, systemrun.MessageOwner{InstanceID: o.InstanceID, TaskID: systemrun.Key(o.ComponentID, o.StateSlot)})
							}
						}
						channels = append(channels, channel)
						count++
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
						add(fmt.Sprintf("%s.definition.aspects[%d]", path, j), "Workflow aspects need execution adapters; they are not ignored")
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
		for j := range c.WorkflowOwners {
			if !usedWorkflowOwners[j] {
				add(fmt.Sprintf("%s.workflowOwners[%d]", path, j), "Mapping must name an owned workflow aspect")
			}
		}
		for j := range c.WorkflowMessages {
			if !usedWorkflowRoutes[j] {
				add(fmt.Sprintf("%s.workflowMessages[%d]", path, j), "Route must name an owned workflow aspect")
			}
		}
		for j := range c.MessageOwners {
			if !usedOwners[j] {
				add(fmt.Sprintf("%s.messageOwners[%d]", path, j), "Mapping must name an owned message aspect")
			}
		}
		for j, binding := range c.Bindings {
			if usedBindings[binding.Requirement] {
				provider := providers[binding.ResourceID]
				required := append([]string{}, system.RequiredCapabilities(binding.Requirement)...)
				required = append(required, binding.RequiredCapabilities...)
				provided := system.ProviderCapabilities(provider)
				supported := map[string]bool{}
				for _, cap := range provided {
					supported[cap] = true
				}
				ok := true
				for _, cap := range required {
					if !supported[cap] {
						ok = false
						add(fmt.Sprintf("%s.bindings[%d]", path, j), "Provider lacks required capability: "+cap)
					}
				}
				prepared.ProviderChecks = append(prepared.ProviderChecks, system.ProviderCheck{ComponentID: c.ID, Requirement: binding.Requirement, ProviderID: binding.ResourceID, Required: required, Provided: provided, Satisfied: ok})
			}
			if !usedBindings[binding.Requirement] {
				add(fmt.Sprintf("%s.bindings[%d]", path, j), "Binding does not identify an executable state/<slot>, messages/<slot> or workflow/<slot> requirement")
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
			add(fmt.Sprintf("executionProviders[%d]", i), "Provider is unused by executable state, message or workflow aspects")
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
	prepared.Plan, err = systemrun.PrepareUnified(tasks, links, channels, workflows)
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
