package store

import (
	"context"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/system"
)

func (s *Store) CheckDefinition(ctx context.Context, d system.ComponentDefinition) (system.DefinitionCompatibility, error) {
	catalog, err := s.SystemCatalog(ctx)
	if err != nil {
		return system.DefinitionCompatibility{}, err
	}
	return s.checkDefinition(ctx, d, catalog)
}

// catalog must contain only previously checked artifacts. This method never
// calls SystemCatalog, avoiding recursion when checking installed manifests.
func (s *Store) checkDefinition(ctx context.Context, d system.ComponentDefinition, catalog []system.CatalogEntry) (system.DefinitionCompatibility, error) {
	r := system.DefinitionCompatibility{Validation: system.ResolveDefinition(d, catalog), Ports: []system.AspectPort{}, Dependencies: []system.Dependency{}}
	if !r.Valid {
		return r, nil
	}
	add := func(path, message string) {
		r.Issues = append(r.Issues, model.ValidationIssue{Path: path, Message: message})
	}
	identities := map[string]system.Dependency{}
	truncated := false
	appendDependency := func(path, kind, id, version, digest string) {
		dep := system.Dependency{Path: path, Kind: kind, ID: id, Version: version, Digest: digest}
		key := kind + "\x00" + id + "\x00" + version
		if prior, exists := identities[key]; exists && prior.Digest != digest {
			add(path, "Conflicting content for "+kind+" "+id+"@"+version+"; also referenced at "+prior.Path)
		} else {
			identities[key] = dep
		}
		if len(r.Dependencies) < 4096 {
			r.Dependencies = append(r.Dependencies, dep)
		} else if !truncated {
			add(path, "Definition closure exceeds 4096 dependency occurrences")
			truncated = true
		}
	}
	data := func(path string, locks []model.DataContractLock) {
		for i, l := range locks {
			appendDependency(fmt.Sprintf("%s[%d]", path, i), "data", l.Contract.ID, l.Contract.Version, l.Digest)
		}
	}
	bundle := func(path string, b model.ComponentBundle) {
		for i, l := range b.Packages {
			p := fmt.Sprintf("%s.packages[%d]", path, i)
			appendDependency(p, "component", l.Package.ID, l.Package.Version, l.Digest)
			data(p+".dataContracts", l.Package.DataContracts)
		}
	}
	machine := func(path string, m model.StateMachineLock) {
		appendDependency(path, "state", m.Machine.ID, m.Machine.Version, m.Digest)
		data(path+".dataContracts", m.Machine.DataContracts)
		for i, c := range m.Machine.Commands {
			bundle(fmt.Sprintf("%s.commands[%d].decision", path, i), c.Decision)
		}
	}
	ports := func(slot, iface, direction string, ps []model.BehaviorPort) {
		for _, p := range ps {
			r.Ports = append(r.Ports, system.AspectPort{AspectEndpoint: system.AspectEndpoint{Slot: slot, Interface: iface, Port: p.Name}, Direction: direction, Type: p.Type, RecordRef: p.RecordRef})
		}
	}
	machinePorts := func(slot, prefix string, m model.StateMachine) {
		for _, c := range m.Commands {
			ports(slot, prefix+"command/"+c.Name+"/inputs", "input", c.Inputs)
			ports(slot, prefix+"command/"+c.Name+"/outputs", "output", c.Outputs)
		}
		for _, e := range m.Effects {
			ports(slot, prefix+"effect/"+e.Name, "output", e.Payload)
		}
	}
	for i, a := range d.Aspects {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		path := fmt.Sprintf("aspects[%d]", i)
		appendDependency(path, a.Kind, a.ID, a.Version, a.Digest)
		switch a.Kind {
		case "component":
			root, err := scanComponent(s.db.QueryRowContext(ctx, `SELECT body,digest,installed_at FROM component_packages WHERE id=$1 AND version=$2`, a.ID, a.Version))
			if err != nil {
				return r, err
			}
			if root.Package.SchemaVersion == 1 {
				continue
			} // Only a timing-profile root has no interface.
			b, err := s.ComponentBundle(ctx, model.ComponentRef{PackageID: a.ID, Version: a.Version, Digest: a.Digest})
			if err != nil {
				if ctx.Err() != nil {
					return r, ctx.Err()
				}
				add(path, "Cannot resolve behavior dependency closure: "+err.Error())
				continue
			}
			bundle(path, b)
			for _, l := range b.Packages {
				if l.Package.ID == a.ID && l.Package.Version == a.Version {
					ports(a.Slot, "inputs", "input", l.Package.Behavior.Inputs)
					ports(a.Slot, "outputs", "output", l.Package.Behavior.Outputs)
				}
			}
		case "data": // Data contracts have types, not executable ports.
		case "state":
			m, err := s.StateMachine(ctx, a.ID, a.Version, a.Digest)
			if err != nil {
				return r, err
			}
			machine(path, model.StateMachineLock{Machine: m.Machine, Digest: m.Digest})
			machinePorts(a.Slot, "", m.Machine)
		case "messages":
			m, err := s.DeliveryModel(ctx, a.ID, a.Version, a.Digest)
			if err != nil {
				return r, err
			}
			for j, l := range m.Model.Machines {
				machine(fmt.Sprintf("%s.machines[%d]", path, j), l)
			}
			for _, instance := range m.Model.Instances {
				for _, l := range m.Model.Machines {
					if l.Machine.ID == instance.Machine.MachineID && l.Machine.Version == instance.Machine.Version && l.Digest == instance.Machine.Digest {
						machinePorts(a.Slot, "instance/"+instance.ID+"/", l.Machine)
					}
				}
			}
		case "workflow":
			w, err := s.WorkflowPackage(ctx, a.ID, a.Version, a.Digest)
			if err != nil {
				return r, err
			}
			data(path+".dataContracts", w.Workflow.DataContracts)
			for j, b := range w.Workflow.Decisions {
				bundle(fmt.Sprintf("%s.decisions[%d]", path, j), b)
			}
			for j, m := range w.Workflow.Machines {
				machine(fmt.Sprintf("%s.machines[%d]", path, j), m)
			}
			ports(a.Slot, "inputs", "input", w.Workflow.Inputs)
			ports(a.Slot, "outputs", "output", w.Workflow.Outputs)
		}
	}
	r.Issues = append(r.Issues, system.CheckAspectLinks(d.Links, r.Ports)...)
	r.Valid = len(r.Issues) == 0
	return r, nil
}
