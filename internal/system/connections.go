package system

import (
	"fmt"
	"software-studio/internal/model"
)

type InstancePort struct {
	ComponentID string `json:"componentId"`
	AspectPort
	Bound bool `json:"bound,omitempty"`
}
type ConnectionCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
type instanceEndpoint struct {
	component string
	endpoint  AspectEndpoint
}

// CheckConnections checks declared data types only, not transport or scheduling.
// Legacy untyped edges remain readable but never receive a checked verdict.
func CheckConnections(connections []Connection, ports []InstancePort) ([]ConnectionCheck, []model.ValidationIssue) {
	index := map[instanceEndpoint]InstancePort{}
	targets := map[instanceEndpoint]bool{}
	issues := []model.ValidationIssue{}
	checks := []ConnectionCheck{}
	for _, p := range ports {
		key := instanceEndpoint{p.ComponentID, p.AspectEndpoint}
		if _, ok := index[key]; ok {
			issues = append(issues, model.ValidationIssue{Path: "components", Message: "Ambiguous port identity"})
		}
		index[key] = p
		if p.Bound {
			targets[key] = true
		}
	}
	for i, c := range connections {
		checked := ConnectionCheck{ID: c.ID, Status: "checked", Message: "Declared output and input types match exactly; execution semantics are not checked"}
		if c.SourcePort == nil && c.TargetPort == nil {
			checked.Status = "untyped"
			checked.Message = "Choose source and target ports to check this legacy connection"
			checks = append(checks, checked)
			continue
		}
		add := func(field, message string) {
			issues = append(issues, model.ValidationIssue{Path: fmt.Sprintf("connections[%d]", i) + field, Message: message})
			checked.Status = "invalid"
			checked.Message = message
		}
		if c.SourcePort == nil || c.TargetPort == nil {
			add("", "Both endpoint ports are required")
			checks = append(checks, checked)
			continue
		}
		source, sok := index[instanceEndpoint{c.Source, *c.SourcePort}]
		target, tok := index[instanceEndpoint{c.Target, *c.TargetPort}]
		if !sok || source.Direction != "output" {
			add(".sourcePort", "Choose an existing output port on the source instance")
		}
		if !tok || target.Direction != "input" {
			add(".targetPort", "Choose an existing input port on the target instance")
		}
		key := instanceEndpoint{c.Target, *c.TargetPort}
		if targets[key] {
			add(".targetPort", "Input already has a producer in this system or inside its component; explicit merging is not supported")
		}
		targets[key] = true
		if sok && tok && !model.SameBehaviorType(model.BehaviorValueType{Type: source.Type, RecordRef: source.RecordRef}, model.BehaviorValueType{Type: target.Type, RecordRef: target.RecordRef}) {
			add(".targetPort", "Incompatible port types: match the scalar type or exact record contract ID, version, digest and record name")
		}
		checks = append(checks, checked)
	}
	return checks, issues
}
