package store

import (
	"context"
	"fmt"
	"software-studio/internal/model"
	"software-studio/internal/system"
	"strings"
)

func (s *Store) resolveSystemCompatibility(ctx context.Context, d system.SystemDesign, catalog []system.CatalogEntry) (system.Resolution, error) {
	result := system.Resolve(d, catalog)
	result.Compatibility = "invalid"
	if !result.Valid {
		return result, nil
	}
	definitions, err := s.Definitions(ctx)
	if err != nil {
		return result, err
	}
	find := func(kind, id, version string) system.CatalogEntry {
		for _, e := range catalog {
			if e.Kind == kind && e.ID == id && e.Version == version {
				return e
			}
		}
		return system.CatalogEntry{}
	}
	identities := map[string]system.Dependency{}
	appendReport := func(report system.DefinitionCompatibility, path, component string, root bool, links []system.AspectLink) {
		rewrite := func(p string) string {
			if root {
				return path + strings.TrimPrefix(p, "aspects[0]")
			}
			return path + "." + p
		}
		for _, i := range report.Issues {
			result.Issues = append(result.Issues, model.ValidationIssue{Path: rewrite(i.Path), Message: i.Message})
		}
		for _, dep := range report.Dependencies {
			dep.Path = rewrite(dep.Path)
			key := dep.Kind + "\x00" + dep.ID + "\x00" + dep.Version
			if old, ok := identities[key]; ok && old.Digest != dep.Digest {
				result.Issues = append(result.Issues, model.ValidationIssue{Path: dep.Path, Message: "Conflicting dependency content; also used at " + old.Path})
			} else {
				identities[key] = dep
			}
			result.Dependencies = append(result.Dependencies, dep)
		}
		if component != "" {
			for _, p := range report.Ports {
				bound := false
				for _, l := range links {
					if l.Target == p.AspectEndpoint {
						bound = true
					}
				}
				result.Ports = append(result.Ports, system.InstancePort{ComponentID: component, AspectPort: p, Bound: bound})
			}
		}
	}
	for i, c := range d.Components {
		path := fmt.Sprintf("components[%d].definition", i)
		if c.Definition.Kind == "definition" {
			for _, record := range definitions {
				if record.Definition.ID == c.Definition.ID && record.Definition.Version == c.Definition.Version {
					report, err := s.checkDefinition(ctx, record.Definition, catalog)
					if err != nil {
						return result, err
					}
					appendReport(report, path, c.ID, false, record.Definition.Links)
					break
				}
			}
		} else if !strings.HasPrefix(c.Definition.ID, "builtin.") {
			e := find("component", c.Definition.ID, c.Definition.Version)
			synthetic := system.ComponentDefinition{SchemaVersion: 1, ID: "system.inspection", Version: "1.0.0", Name: "Instance inspection", Aspects: []system.AspectRef{{Slot: "root", Kind: "component", ID: e.ID, Version: e.Version, Digest: e.Digest}}}
			report, err := s.checkDefinition(ctx, synthetic, catalog)
			if err != nil {
				return result, err
			}
			appendReport(report, path, c.ID, true, nil)
		}
		for j, a := range c.Artifacts {
			if strings.HasPrefix(a.ID, "builtin.") {
				continue
			}
			e := find(a.Kind, a.ID, a.Version)
			synthetic := system.ComponentDefinition{SchemaVersion: 1, ID: "system.inspection", Version: "1.0.0", Name: "Attachment inspection", Aspects: []system.AspectRef{{Slot: "root", Kind: a.Kind, ID: e.ID, Version: e.Version, Digest: e.Digest}}}
			report, err := s.checkDefinition(ctx, synthetic, catalog)
			if err != nil {
				return result, err
			}
			appendReport(report, fmt.Sprintf("components[%d].artifacts[%d]", i, j), "", true, nil)
		}
		if len(result.Dependencies) > 8192 {
			result.Dependencies = result.Dependencies[:8192]
			result.Issues = append(result.Issues, model.ValidationIssue{Path: "components", Message: "System closure exceeds 8192 dependency occurrences"})
			result.Valid = false
			return result, nil
		}
	}
	checks, issues := system.CheckConnections(d.Connections, result.Ports)
	result.Connections = checks
	result.Issues = append(result.Issues, issues...)
	result.Valid = len(result.Issues) == 0
	if result.Valid {
		result.Compatibility = "checked"
		for _, c := range checks {
			if c.Status == "untyped" {
				result.Compatibility = "incomplete"
			}
		}
	}
	return result, nil
}
