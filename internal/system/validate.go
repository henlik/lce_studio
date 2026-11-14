package system

import (
	"fmt"
	"regexp"
	"software-studio/internal/model"
	"strings"
)

var exactVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func supportedClaim(kind string) bool {
	switch kind {
	case "exactly_one_record", "response_matches_record", "no_false_success", "required_evidence":
		return true
	}
	return false
}

func ValidateContract(c Contract) []model.ValidationIssue {
	issues := []model.ValidationIssue{}
	add := func(path, msg string) { issues = append(issues, model.ValidationIssue{Path: path, Message: msg}) }
	if strings.TrimSpace(c.ID) == "" {
		add("id", "Contract ID is required")
	}
	if strings.TrimSpace(c.Revision) == "" {
		add("revision", "Contract revision is required")
	}
	if len(c.Claims) == 0 {
		add("claims", "At least one executable claim is required")
	}
	ids := map[string]bool{}
	for i, c := range c.Claims {
		p := fmt.Sprintf("claims[%d]", i)
		if strings.TrimSpace(c.ID) == "" || ids[c.ID] {
			add(p+".id", "Claim IDs must be nonempty and unique")
		}
		ids[c.ID] = true
		if !supportedClaim(c.Kind) {
			add(p+".kind", "Unsupported executable claim")
		}
	}
	ids = map[string]bool{}
	for i, a := range c.Assumptions {
		p := fmt.Sprintf("assumptions[%d]", i)
		if strings.TrimSpace(a.ID) == "" || ids[a.ID] {
			add(p+".id", "Assumption IDs must be nonempty and unique")
		}
		ids[a.ID] = true
		if strings.TrimSpace(a.Description) == "" {
			add(p+".description", "Describe the assumption explicitly")
		}
	}
	return issues
}

// Validate checks local structure and references. Referenced artifact existence
// and content integrity must additionally be resolved by the catalog adapter.
func Validate(d SystemDesign) model.Validation {
	issues := []model.ValidationIssue{}
	add := func(p, m string) { issues = append(issues, model.ValidationIssue{Path: p, Message: m}) }
	if d.SchemaVersion != 1 {
		add("schemaVersion", "Only schema version 1 is supported")
	}
	for _, field := range []struct{ path, value string }{{"id", d.ID}, {"name", d.Name}, {"revision", d.Revision}} {
		if strings.TrimSpace(field.value) == "" {
			add(field.path, "Value is required")
		}
	}
	components := map[string]bool{}
	for i, c := range d.Components {
		p := fmt.Sprintf("components[%d]", i)
		if strings.TrimSpace(c.ID) == "" || components[c.ID] {
			add(p+".id", "Component instance IDs must be nonempty and unique")
		}
		components[c.ID] = true
		if c.Definition.Kind != "" && c.Definition.Kind != "component" && c.Definition.Kind != "definition" {
			add(p+".definition.kind", "Choose a component package or a unified definition")
		}
		if strings.TrimSpace(c.Definition.ID) == "" {
			add(p+".definition.id", "Definition ID is required")
		}
		if !exactVersion.MatchString(c.Definition.Version) {
			add(p+".definition.version", "An exact major.minor.patch version is required")
		}
		for j, a := range c.Artifacts {
			q := fmt.Sprintf("%s.artifacts[%d]", p, j)
			switch a.Kind {
			case "component", "data", "state", "messages", "workflow":
			default:
				add(q+".kind", "Unsupported artifact kind")
			}
			if strings.TrimSpace(a.ID) == "" {
				add(q+".id", "Artifact ID is required")
			}
			if !exactVersion.MatchString(a.Version) {
				add(q+".version", "An exact major.minor.patch version is required")
			}
		}
	}
	resources := map[string]bool{}
	plans := map[string]bool{}
	for i, p := range d.DeploymentPlans {
		path := fmt.Sprintf("deploymentPlans[%d]", i)
		if strings.TrimSpace(p.ID) == "" || plans[p.ID] {
			add(path+".id", "Deployment plan IDs must be nonempty and unique")
		}
		plans[p.ID] = true
		local := map[string]bool{}
		kinds := map[string]string{}
		for j, r := range p.Resources {
			q := fmt.Sprintf("%s.resources[%d]", path, j)
			if strings.TrimSpace(r.ID) == "" || local[r.ID] {
				add(q+".id", "Resource IDs must be nonempty and unique within a plan")
			}
			local[r.ID] = true
			kinds[r.ID] = r.Kind
			resources[r.ID] = true
			switch r.Kind {
			case "compute", "postgresql", "redis", "broker", "network":
			default:
				add(q+".kind", "Unsupported resource kind")
			}
		}
		placed := map[string]bool{}
		for j, x := range p.Placements {
			q := fmt.Sprintf("%s.placements[%d]", path, j)
			if !components[x.ComponentID] {
				add(q+".componentId", "Unknown component instance")
			}
			if !local[x.ResourceID] {
				add(q+".resourceId", "Unknown resource in this deployment plan")
			} else if kinds[x.ResourceID] != "compute" {
				add(q+".resourceId", "Components must be placed on compute resources")
			}
			if placed[x.ComponentID] {
				add(q+".componentId", "A component can have one placement per plan in schema version 1")
			}
			placed[x.ComponentID] = true
		}
		for ci, c := range d.Components {
			for bi, b := range c.Bindings {
				if !local[b.ResourceID] {
					add(fmt.Sprintf("components[%d].bindings[%d].resourceId", ci, bi), fmt.Sprintf("Resource is missing from deployment plan %q", p.ID))
				}
			}
		}
	}
	for i, c := range d.Components {
		bound := map[string]bool{}
		for j, b := range c.Bindings {
			p := fmt.Sprintf("components[%d].bindings[%d]", i, j)
			if strings.TrimSpace(b.Requirement) == "" || bound[b.Requirement] {
				add(p+".requirement", "Binding requirements must be nonempty and unique")
			}
			bound[b.Requirement] = true
			if !resources[b.ResourceID] {
				add(p+".resourceId", "Unknown deployment resource")
			}
		}
	}
	connections := map[string]bool{}
	for i, c := range d.Connections {
		p := fmt.Sprintf("connections[%d]", i)
		if strings.TrimSpace(c.ID) == "" || connections[c.ID] {
			add(p+".id", "Connection IDs must be nonempty and unique")
		}
		connections[c.ID] = true
		if !components[c.Source] {
			add(p+".source", "Unknown source component")
		}
		if !components[c.Target] {
			add(p+".target", "Unknown target component")
		}
		switch c.Kind {
		case "request", "event", "data":
		default:
			add(p+".kind", "Unsupported connection kind")
		}
	}
	for _, e := range ValidateContract(d.Contract) {
		add("contract."+e.Path, e.Message)
	}
	return model.Validation{Valid: len(issues) == 0, Issues: issues}
}
