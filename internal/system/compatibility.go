package system

import (
	"fmt"
	"software-studio/internal/model"
)

// Interface names identify declared surfaces, not delivery or scheduling semantics.
type AspectPort struct {
	AspectEndpoint
	Direction string             `json:"direction"`
	Type      string             `json:"type"`
	RecordRef *model.DataTypeRef `json:"recordRef,omitempty"`
}
type Dependency struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}
type DefinitionCompatibility struct {
	model.Validation
	Ports        []AspectPort `json:"ports"`
	Dependencies []Dependency `json:"dependencies"`
}

// CheckAspectLinks is a static type check. It deliberately does not claim
// completeness of required inputs, trigger ordering or joint execution.
func CheckAspectLinks(links []AspectLink, ports []AspectPort) []model.ValidationIssue {
	issues := []model.ValidationIssue{}
	index := map[AspectEndpoint]AspectPort{}
	for _, p := range ports {
		if _, exists := index[p.AspectEndpoint]; exists {
			issues = append(issues, model.ValidationIssue{Path: "aspects", Message: "Ambiguous exported endpoint for aspect " + p.Slot})
		}
		index[p.AspectEndpoint] = p
	}
	targets := map[AspectEndpoint]bool{}
	for i, l := range links {
		path := fmt.Sprintf("links[%d]", i)
		source, okSource := index[l.Source]
		target, okTarget := index[l.Target]
		add := func(suffix, message string) {
			issues = append(issues, model.ValidationIssue{Path: path + suffix, Message: message})
		}
		if !okSource || source.Direction != "output" {
			add(".source", "Choose an existing output port")
		}
		if !okTarget || target.Direction != "input" {
			add(".target", "Choose an existing input port")
		}
		if targets[l.Target] {
			add(".target", "This input already has a producer; explicit merge semantics are not supported")
		}
		targets[l.Target] = true
		if okSource && okTarget && !model.SameBehaviorType(model.BehaviorValueType{Type: source.Type, RecordRef: source.RecordRef}, model.BehaviorValueType{Type: target.Type, RecordRef: target.RecordRef}) {
			add(".target", "Port types must match exactly, including record contract ID, version, digest and record name")
		}
	}
	return issues
}
