package model

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var nodeColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var behaviorName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

func sortedBehaviorKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func behaviorTextValid(value string, max int, required bool) bool {
	if len(value) > max || !utf8.ValidString(value) || (required && strings.TrimSpace(value) == "") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return false
		}
	}
	return true
}

// ValidateTypedValue rejects null/mixed values, coercion and browser-unsafe numbers.
func ValidateTypedValue(v TypedValue) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if v.RecordRef != nil || v.Record != nil || v.Collection != nil {
		issues = append(issues, ValidationIssue{"", "Scalar values cannot contain structured value members."})
	}
	count := 0
	if v.Boolean != nil {
		count++
	}
	if v.Integer != nil {
		count++
	}
	if v.String != nil {
		count++
	}
	if !scalarType(v.Type) {
		issues = append(issues, ValidationIssue{"type", "Type must be boolean, integer or string."})
	}
	if count != 1 || (v.Type == "boolean" && v.Boolean == nil) || (v.Type == "integer" && v.Integer == nil) || (v.Type == "string" && v.String == nil) {
		issues = append(issues, ValidationIssue{"", "Exactly one value member must match the declared scalar type."})
	}
	if v.Integer != nil && (*v.Integer < -MaxBehaviorInteger || *v.Integer > MaxBehaviorInteger) {
		issues = append(issues, ValidationIssue{"integer", "Integer must be within -9007199254740991..9007199254740991."})
	}
	if v.String != nil && !behaviorTextValid(*v.String, 4096, false) {
		issues = append(issues, ValidationIssue{"string", "String must be valid text of at most 4096 bytes without unsupported control characters."})
	}
	return issues
}

func prefixBehaviorIssues(prefix string, issues []ValidationIssue) []ValidationIssue {
	out := make([]ValidationIssue, 0, len(issues))
	for _, issue := range issues {
		path := prefix
		if issue.Path != "" {
			path += "." + issue.Path
		}
		out = append(out, ValidationIssue{path, issue.Message})
	}
	return out
}

// ValidateBehaviorValues checks an exact public input/output map in sorted order.
func ValidateBehaviorValues(values map[string]TypedValue, ports []BehaviorPort) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if len(values) > 16 || len(ports) > 16 {
		return []ValidationIssue{{"", "At most 16 typed values are supported."}}
	}
	expected := make(map[string]string, len(ports))
	for _, port := range ports {
		expected[port.Name] = port.Type
	}
	for _, name := range sortedBehaviorKeys(expected) {
		value, ok := values[name]
		if !ok {
			issues = append(issues, ValidationIssue{name, "A value is required for this public port."})
			continue
		}
		issues = append(issues, prefixBehaviorIssues(name, ValidateTypedValue(value))...)
		if value.Type != expected[name] {
			issues = append(issues, ValidationIssue{name + ".type", "Value type must match the public port type."})
		}
	}
	for _, name := range sortedBehaviorKeys(values) {
		if _, ok := expected[name]; !ok {
			issues = append(issues, ValidationIssue{name, "Value does not correspond to a public port."})
		}
	}
	return issues
}

func ValidateComponentReference(ref ComponentRef) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if !safeID.MatchString(ref.PackageID) || !strings.Contains(ref.PackageID, ".") {
		issues = append(issues, ValidationIssue{"packageId", "Reference requires a namespaced safe package ID."})
	}
	if len(ref.Version) > 32 || !componentVersion.MatchString(ref.Version) {
		issues = append(issues, ValidationIssue{"version", "Reference requires an exact three-part decimal version."})
	}
	if !componentDigest.MatchString(ref.Digest) {
		issues = append(issues, ValidationIssue{"digest", "Reference requires a lowercase SHA-256 digest."})
	}
	return issues
}

func validateBehaviorLocal(p ComponentPackage) []ValidationIssue {
	b := p.Behavior
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	if b.Inputs == nil {
		add("behavior.inputs", "Inputs must be an array; use [] when empty.")
	}
	if b.Outputs == nil {
		add("behavior.outputs", "Outputs must be an array.")
	}
	if b.Nodes == nil {
		add("behavior.nodes", "Nodes must be an array; use [] when empty.")
	}
	if b.Bindings == nil {
		add("behavior.bindings", "Output bindings must be an object.")
	}
	if len(b.Inputs) > 16 {
		add("behavior.inputs", "At most 16 input ports are supported.")
	}
	if len(b.Outputs) < 1 || len(b.Outputs) > 16 {
		add("behavior.outputs", "Between 1 and 16 output ports are required.")
	}
	if len(b.Nodes) > 32 {
		add("behavior.nodes", "At most 32 internal nodes are supported.")
	}
	if len(b.Bindings) > 16 {
		add("behavior.bindings", "At most 16 public output bindings are supported.")
	}
	if len(p.Tests) > 32 {
		add("tests", "At most 32 test cases are supported.")
	}
	if len(issues) != 0 {
		return issues
	}
	ports := func(path string, list []BehaviorPort) map[string]string {
		result := make(map[string]string)
		for i, port := range list {
			at := fmt.Sprintf("%s[%d]", path, i)
			if !behaviorName.MatchString(port.Name) {
				add(at+".name", "Port names require 1–64 letters, digits or underscores, starting with a letter.")
			}
			if _, exists := result[port.Name]; exists {
				add(at+".name", "Port names must be unique within their direction.")
			}
			if p.SchemaVersion == 3 {
				issues = append(issues, prefixBehaviorIssues(at, validateBehaviorValueType(PortBehaviorType(port)))...)
			} else {
				if !scalarType(port.Type) {
					add(at+".type", "Public port types must be boolean, integer or string.")
				}
				if port.RecordRef != nil {
					add(at+".recordRef", "Record references require component schema version 3.")
				}
			}
			result[port.Name] = port.Type
		}
		return result
	}
	inputs, outputs := ports("behavior.inputs", b.Inputs), ports("behavior.outputs", b.Outputs)
	nodes := make(map[string]bool)
	knownOps := make(map[string]bool)
	specs := PrimitiveSpecs()
	if p.SchemaVersion == 3 {
		specs = StructuredPrimitiveSpecs()
	}
	for _, spec := range specs {
		knownOps[spec.Op] = true
	}
	for i, node := range b.Nodes {
		at := fmt.Sprintf("behavior.nodes[%d]", i)
		if node.Inputs == nil {
			add(at+".inputs", "Node inputs must be an object; use {} when empty.")
		}
		if !behaviorName.MatchString(node.ID) {
			add(at+".id", "Local IDs require 1–64 letters, digits or underscores, starting with a letter.")
		}
		if nodes[node.ID] {
			add(at+".id", "Local node IDs must be unique.")
		}
		nodes[node.ID] = true
		if (node.Op == "") == (node.Component == nil) {
			add(at, "Specify exactly one primitive op or component reference.")
		}
		if node.Op != "" && !knownOps[node.Op] {
			add(at+".op", "Unsupported pure primitive.")
		}
		if p.SchemaVersion == 3 {
			issues = append(issues, prefixBehaviorIssues(at, validateStructuredNodeMetadata(node))...)
		} else if node.RecordRef != nil || node.Field != "" {
			add(at, "Structured node metadata requires component schema version 3.")
		}
		if node.Component != nil {
			issues = append(issues, prefixBehaviorIssues(at+".component", ValidateComponentReference(*node.Component))...)
		}
		if node.Color != "" && !nodeColor.MatchString(node.Color) {
			add(at+".color", "Node color must be a six-digit hexadecimal color.")
		}
		if math.IsNaN(node.Position.X) || math.IsInf(node.Position.X, 0) || math.Abs(node.Position.X) > 1e6 {
			add(at+".position.x", "Position must be finite within -1000000..1000000.")
		}
		if math.IsNaN(node.Position.Y) || math.IsInf(node.Position.Y, 0) || math.Abs(node.Position.Y) > 1e6 {
			add(at+".position.y", "Position must be finite within -1000000..1000000.")
		}
		if len(node.Inputs) > 16 {
			add(at+".inputs", "At most 16 input bindings are supported per node.")
		}
	}
	binding := func(path string, value BehaviorBinding) {
		forms := 0
		if value.Input != "" {
			forms++
		}
		if value.Node != "" || value.Port != "" {
			forms++
		}
		if value.Literal != nil {
			forms++
		}
		if forms != 1 {
			add(path, "Specify exactly one outer input, node+port, or typed literal source.")
		}
		if value.Input != "" {
			if _, ok := inputs[value.Input]; !ok {
				add(path+".input", "Binding must reference a declared public input.")
			}
		}
		if value.Node != "" || value.Port != "" {
			if !nodes[value.Node] {
				add(path+".node", "Binding must reference an existing local node.")
			}
			if !behaviorName.MatchString(value.Port) {
				add(path+".port", "Node sources require a valid output port name.")
			}
		}
		if value.Literal != nil {
			if p.SchemaVersion == 3 {
				issues = append(issues, prefixBehaviorIssues(path+".literal", validateStructuredShape(*value.Literal))...)
			} else {
				issues = append(issues, prefixBehaviorIssues(path+".literal", ValidateTypedValue(*value.Literal))...)
			}
		}
	}
	for i, node := range b.Nodes {
		if len(node.Inputs) > 16 {
			continue
		}
		for _, name := range sortedBehaviorKeys(node.Inputs) {
			at := fmt.Sprintf("behavior.nodes[%d].inputs.%s", i, name)
			if !behaviorName.MatchString(name) {
				add(at, "Input binding names must be valid local identifiers.")
			}
			binding(at, node.Inputs[name])
		}
	}
	for _, name := range sortedBehaviorKeys(outputs) {
		if _, ok := b.Bindings[name]; !ok {
			add("behavior.bindings."+name, "Every public output requires one binding.")
		}
	}
	for _, name := range sortedBehaviorKeys(b.Bindings) {
		if _, ok := outputs[name]; !ok {
			add("behavior.bindings."+name, "Binding does not correspond to a public output.")
		}
		binding("behavior.bindings."+name, b.Bindings[name])
	}
	names := make(map[string]bool)
	for i, test := range p.Tests {
		at := fmt.Sprintf("tests[%d]", i)
		if !behaviorTextValid(test.Name, 100, true) {
			add(at+".name", "Test names require nonblank valid text of at most 100 bytes.")
		}
		if names[test.Name] {
			add(at+".name", "Test names must be unique.")
		}
		names[test.Name] = true
		if p.SchemaVersion == 3 {
			issues = append(issues, prefixBehaviorIssues(at+".inputs", validateStructuredBehaviorValues(test.Inputs, b.Inputs, nil, false))...)
			issues = append(issues, prefixBehaviorIssues(at+".expected", validateStructuredBehaviorValues(test.Expected, b.Outputs, nil, false))...)
		} else {
			issues = append(issues, prefixBehaviorIssues(at+".inputs", ValidateBehaviorValues(test.Inputs, b.Inputs))...)
			issues = append(issues, prefixBehaviorIssues(at+".expected", ValidateBehaviorValues(test.Expected, b.Outputs))...)
		}
	}
	if p.SchemaVersion == 3 && len(issues) == 0 && len(p.DataContracts) <= 8 {
		issues = append(issues, validateLocalStructuredReferences(p)...)
	}
	return issues
}

// BehaviorNodeOrder is stable regardless of definition array order and does not mutate it.
func BehaviorNodeOrder(b Behavior) ([]BehaviorNode, error) {
	if len(b.Nodes) > 32 {
		return nil, fmt.Errorf("at most 32 internal nodes are supported")
	}
	nodes := make(map[string]BehaviorNode, len(b.Nodes))
	for _, node := range b.Nodes {
		if _, exists := nodes[node.ID]; exists {
			return nil, fmt.Errorf("duplicate local node %q", node.ID)
		}
		if len(node.Inputs) > 16 {
			return nil, fmt.Errorf("node %s has too many inputs", node.ID)
		}
		nodes[node.ID] = node
	}
	remaining := make(map[string]map[string]bool, len(nodes))
	for _, id := range sortedBehaviorKeys(nodes) {
		node := nodes[id]
		deps := make(map[string]bool)
		for _, name := range sortedBehaviorKeys(node.Inputs) {
			input := node.Inputs[name]
			if input.Node != "" {
				if _, exists := nodes[input.Node]; !exists {
					return nil, fmt.Errorf("node %s references missing node %s", id, input.Node)
				}
				deps[input.Node] = true
			}
		}
		remaining[id] = deps
	}
	order := make([]BehaviorNode, 0, len(nodes))
	for len(remaining) > 0 {
		ready := ""
		for _, id := range sortedBehaviorKeys(remaining) {
			if len(remaining[id]) == 0 {
				ready = id
				break
			}
		}
		if ready == "" {
			return nil, fmt.Errorf("local dataflow contains a cycle")
		}
		order = append(order, nodes[ready])
		delete(remaining, ready)
		for _, deps := range remaining {
			delete(deps, ready)
		}
	}
	return order, nil
}
