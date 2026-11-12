package model

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func behaviorBool(v bool) TypedValue     { return TypedValue{Type: "boolean", Boolean: &v} }
func behaviorInt(v int64) TypedValue     { return TypedValue{Type: "integer", Integer: &v} }
func behaviorString(v string) TypedValue { return TypedValue{Type: "string", String: &v} }

func TestTypedValuesHaveExactDiscriminantsAndBounds(t *testing.T) {
	valid := []TypedValue{behaviorBool(false), behaviorBool(true), behaviorInt(0), behaviorInt(MaxBehaviorInteger), behaviorInt(-MaxBehaviorInteger), behaviorString(""), behaviorString("Unicode é\n\t\r"), behaviorString(strings.Repeat("x", 4096))}
	for _, value := range valid {
		if issues := ValidateTypedValue(value); len(issues) != 0 {
			t.Fatalf("valid typed value rejected: %+v", issues)
		}
	}
	invalid := []TypedValue{{}, {Type: "boolean"}, {Type: "float"}, {Type: "boolean", Integer: behaviorInt(1).Integer}, {Type: "integer", Integer: behaviorInt(1).Integer, Boolean: behaviorBool(true).Boolean}, behaviorInt(MaxBehaviorInteger + 1), behaviorInt(-MaxBehaviorInteger - 1), behaviorString(strings.Repeat("x", 4097)), behaviorString("x\x00"), behaviorString("x\x01"), behaviorString(string([]byte{255}))}
	for _, value := range invalid {
		if issues := ValidateTypedValue(value); len(issues) == 0 {
			t.Fatalf("invalid typed value accepted: %+v", value)
		}
	}
	ports := []BehaviorPort{{Name: "active", Type: "boolean"}, {Name: "count", Type: "integer"}}
	if issues := ValidateBehaviorValues(map[string]TypedValue{"active": behaviorBool(false), "count": behaviorInt(0)}, ports); len(issues) != 0 {
		t.Fatal(issues)
	}
	issues := ValidateBehaviorValues(map[string]TypedValue{"active": behaviorString("false"), "extra": behaviorInt(1)}, ports)
	if !issueAt(issues, "active.type") || !issueAt(issues, "count") || !issueAt(issues, "extra") {
		t.Fatalf("missing, extra or mistyped inputs ignored: %+v", issues)
	}
	if again := ValidateBehaviorValues(map[string]TypedValue{"extra": behaviorInt(1), "active": behaviorString("false")}, ports); !reflect.DeepEqual(again, issues) {
		t.Fatal("diagnostics depend on input map order")
	}
}

func TestPrimitiveTypeUnificationAndMetadata(t *testing.T) {
	if len(PrimitiveSpecs()) != 11 {
		t.Fatal("primitive discovery omitted a supported operation")
	}
	for _, spec := range PrimitiveSpecs() {
		for _, generic := range []string{"boolean", "integer", "string"} {
			inputs := make(map[string]string)
			for _, port := range spec.Inputs {
				inputs[port.Name] = port.Type
				if port.Type == "T" {
					inputs[port.Name] = generic
				}
			}
			want := spec.Outputs[0].Type
			if want == "T" {
				want = generic
			}
			if output, err := PrimitiveOutputType(spec.Op, inputs); err != nil || output != want {
				t.Fatalf("%s signature: got %q, %v", spec.Op, output, err)
			}
			inputs["unexpected"] = "string"
			if _, err := PrimitiveOutputType(spec.Op, inputs); err == nil {
				t.Fatalf("%s accepted an extra input", spec.Op)
			}
		}
	}
	for _, tt := range []struct {
		op     string
		inputs map[string]string
	}{
		{"execute", map[string]string{}},
		{"identity", map[string]string{"value": "T"}},
		{"equal", map[string]string{"left": "string", "right": "integer"}},
		{"select", map[string]string{"condition": "boolean", "trueValue": "boolean", "falseValue": "string"}},
		{"select", map[string]string{"condition": "integer", "trueValue": "string", "falseValue": "string"}},
		{"add", map[string]string{"left": "integer"}},
		{"and", map[string]string{"left": "boolean", "other": "boolean"}},
	} {
		if _, err := PrimitiveOutputType(tt.op, tt.inputs); err == nil {
			t.Fatalf("invalid %s signature accepted", tt.op)
		}
	}
	specs := PrimitiveSpecs()
	specs[0].Inputs[0].Name = "changed"
	if PrimitiveSpecs()[0].Inputs[0].Name != "value" {
		t.Fatal("discovery exposes shared mutable metadata")
	}
}

func TestBehaviorOrderIsStableAndDetectsAllCycles(t *testing.T) {
	b := Behavior{Nodes: []BehaviorNode{
		{ID: "third", Inputs: map[string]BehaviorBinding{"a": {Node: "first", Port: "result"}, "b": {Node: "first", Port: "result"}}},
		{ID: "second", Inputs: map[string]BehaviorBinding{}},
		{ID: "first", Inputs: map[string]BehaviorBinding{}},
	}}
	order, err := BehaviorNodeOrder(b)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{order[0].ID, order[1].ID, order[2].ID}
	if !reflect.DeepEqual(got, []string{"first", "second", "third"}) {
		t.Fatalf("unstable order: %v", got)
	}
	if b.Nodes[0].ID != "third" {
		t.Fatal("topological sorting mutated the definition")
	}
	b.Nodes[2].Inputs["cycle"] = BehaviorBinding{Node: "third", Port: "result"}
	if _, err := BehaviorNodeOrder(b); err == nil {
		t.Fatal("cycle accepted")
	}
	delete(b.Nodes[2].Inputs, "cycle")
	b.Nodes[1].Inputs["missing"] = BehaviorBinding{Node: "absent", Port: "result"}
	if _, err := BehaviorNodeOrder(b); err == nil {
		t.Fatal("dangling node accepted")
	}
}

func TestCompositeLocalValidation(t *testing.T) {
	tests := []struct {
		name, path string
		mutate     func(*ComponentPackage)
	}{
		{"kernel", "kernel", func(p *ComponentPackage) { p.Kernel = "service" }},
		{"capabilities", "capabilities", func(p *ComponentPackage) { p.Capabilities = []string{"design", "simulate"} }},
		{"legacy config", "config", func(p *ComponentPackage) { p.Config.Capacity.Default = 1 }},
		{"missing behavior", "behavior", func(p *ComponentPackage) { p.Behavior = nil }},
		{"null inputs", "behavior.inputs", func(p *ComponentPackage) { p.Behavior.Inputs = nil }},
		{"null nodes", "behavior.nodes", func(p *ComponentPackage) { p.Behavior.Nodes = nil }},
		{"null output bindings", "behavior.bindings", func(p *ComponentPackage) { p.Behavior.Bindings = nil }},
		{"null node inputs", "behavior.nodes[0].inputs", func(p *ComponentPackage) { p.Behavior.Nodes[0].Inputs = nil }},
		{"generic public port", "behavior.inputs[0].type", func(p *ComponentPackage) { p.Behavior.Inputs[0].Type = "T" }},
		{"unsafe port", "behavior.inputs[0].name", func(p *ComponentPackage) { p.Behavior.Inputs[0].Name = "tenant.id" }},
		{"duplicate input", "behavior.inputs[1].name", func(p *ComponentPackage) { p.Behavior.Inputs[1].Name = p.Behavior.Inputs[0].Name }},
		{"unsafe local ID", "behavior.nodes[0].id", func(p *ComponentPackage) { p.Behavior.Nodes[0].ID = "nested/path" }},
		{"missing operation", "behavior.nodes[0]", func(p *ComponentPackage) { p.Behavior.Nodes[0].Component = nil }},
		{"two operations", "behavior.nodes[0]", func(p *ComponentPackage) { p.Behavior.Nodes[0].Op = "identity" }},
		{"unsupported operation", "behavior.nodes[0].op", func(p *ComponentPackage) { p.Behavior.Nodes[0].Op, p.Behavior.Nodes[0].Component = "network", nil }},
		{"bad nested ref", "behavior.nodes[0].component.digest", func(p *ComponentPackage) { p.Behavior.Nodes[0].Component.Digest = "bad" }},
		{"NaN position", "behavior.nodes[0].position.x", func(p *ComponentPackage) { p.Behavior.Nodes[0].Position.X = math.NaN() }},
		{"infinite position", "behavior.nodes[0].position.y", func(p *ComponentPackage) { p.Behavior.Nodes[0].Position.Y = math.Inf(1) }},
		{"position bound", "behavior.nodes[0].position.x", func(p *ComponentPackage) { p.Behavior.Nodes[0].Position.X = 1000001 }},
		{"mixed binding", "behavior.nodes[0].inputs.active", func(p *ComponentPackage) {
			v := behaviorBool(true)
			p.Behavior.Nodes[0].Inputs["active"] = BehaviorBinding{Input: "active", Literal: &v}
		}},
		{"dangling input", "behavior.nodes[0].inputs.active.input", func(p *ComponentPackage) { p.Behavior.Nodes[0].Inputs["active"] = BehaviorBinding{Input: "missing"} }},
		{"port without node", "behavior.nodes[0].inputs.active.node", func(p *ComponentPackage) { p.Behavior.Nodes[0].Inputs["active"] = BehaviorBinding{Port: "result"} }},
		{"node without port", "behavior.bindings.permitted.port", func(p *ComponentPackage) { p.Behavior.Bindings["permitted"] = BehaviorBinding{Node: "policy"} }},
		{"missing output", "behavior.bindings.permitted", func(p *ComponentPackage) { delete(p.Behavior.Bindings, "permitted") }},
		{"extra output", "behavior.bindings.extra", func(p *ComponentPackage) { p.Behavior.Bindings["extra"] = BehaviorBinding{Input: "active"} }},
		{"duplicate test name", "tests[1].name", func(p *ComponentPackage) { p.Tests[1].Name = p.Tests[0].Name }},
		{"empty test name", "tests[0].name", func(p *ComponentPackage) { p.Tests[0].Name = "  " }},
		{"missing test input", "tests[0].inputs.active", func(p *ComponentPackage) { delete(p.Tests[0].Inputs, "active") }},
		{"mistyped expected", "tests[0].expected.permitted.type", func(p *ComponentPackage) { p.Tests[0].Expected["permitted"] = behaviorString("true") }},
		{"input bound", "behavior.inputs", func(p *ComponentPackage) { p.Behavior.Inputs = make([]BehaviorPort, 17) }},
		{"output bound", "behavior.outputs", func(p *ComponentPackage) { p.Behavior.Outputs = nil }},
		{"node bound", "behavior.nodes", func(p *ComponentPackage) { p.Behavior.Nodes = make([]BehaviorNode, 33) }},
		{"test bound", "tests", func(p *ComponentPackage) { p.Tests = make([]BehaviorCase, 33) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ExampleComponentBundles()[0].Packages[0].Package
			tt.mutate(&p)
			if issues := ValidateComponentPackage(p); !issueAt(issues, tt.path) {
				t.Fatalf("expected %s; got %+v", tt.path, issues)
			}
		})
	}
	p := ExampleComponentPackages()[0]
	p.Behavior, p.Tests = &Behavior{}, []BehaviorCase{{Name: "Unsupported"}}
	if issues := ValidateComponentPackage(p); !issueAt(issues, "behavior") || !issueAt(issues, "tests") {
		t.Fatalf("profile silently ignored executable declarations: %+v", issues)
	}
}
