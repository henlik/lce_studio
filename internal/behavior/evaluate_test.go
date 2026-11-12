package behavior

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"software-studio/internal/model"
)

func definition(id string, body model.Behavior) model.ComponentPackage {
	return model.ComponentPackage{SchemaVersion: 2, ID: id, Version: "1.0.0", Name: id, Author: "Test author", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, Behavior: &body}
}

func lockDefinition(t testing.TB, p model.ComponentPackage) model.PackageLock {
	t.Helper()
	digest, err := model.ComponentDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	return model.PackageLock{Package: p, Digest: digest}
}

func reference(lock model.PackageLock) model.ComponentRef {
	return model.ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}
}

func bundleDefinition(t testing.TB, p model.ComponentPackage, dependencies ...model.PackageLock) model.ComponentBundle {
	t.Helper()
	lock := lockDefinition(t, p)
	return model.ComponentBundle{SchemaVersion: 1, Root: reference(lock), Packages: append([]model.PackageLock{lock}, dependencies...)}
}

func input(name string) model.BehaviorBinding { return model.BehaviorBinding{Input: name} }
func output(node string) model.BehaviorBinding {
	return model.BehaviorBinding{Node: node, Port: "result"}
}
func literal(value model.TypedValue) model.BehaviorBinding {
	return model.BehaviorBinding{Literal: &value}
}

func primitiveDefinition(t testing.TB, op string, inputs map[string]model.TypedValue) model.ComponentPackage {
	t.Helper()
	ports := []model.BehaviorPort{}
	bindings := map[string]model.BehaviorBinding{}
	types := map[string]string{}
	for _, name := range sortedKeys(inputs) {
		ports = append(ports, model.BehaviorPort{Name: name, Type: inputs[name].Type})
		bindings[name], types[name] = input(name), inputs[name].Type
	}
	typeName, err := model.PrimitiveOutputType(op, types)
	if err != nil {
		t.Fatal(err)
	}
	return definition("test.primitive", model.Behavior{Inputs: ports, Outputs: []model.BehaviorPort{{Name: "result", Type: typeName}},
		Nodes: []model.BehaviorNode{{ID: "operation", Op: op, Inputs: bindings}}, Bindings: map[string]model.BehaviorBinding{"result": output("operation")}})
}

func evaluateOK(t testing.TB, bundle model.ComponentBundle, inputs map[string]model.TypedValue) Result {
	t.Helper()
	r, err := Evaluate(context.Background(), bundle, inputs)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > MaxResultBytes {
		t.Fatalf("invalid result evidence: %v, %d bytes", err, len(encoded))
	}
	if len(r.Events) != r.Operations+2*r.Invocations {
		t.Fatalf("event accounting: %+v", r)
	}
	for i, event := range r.Events {
		if event.Sequence != i+1 {
			t.Fatalf("event sequence: %+v", event)
		}
	}
	return r
}

func TestAllPrimitiveSignaturesAndValues(t *testing.T) {
	for _, tc := range []struct {
		name, op string
		inputs   map[string]model.TypedValue
		want     model.TypedValue
	}{
		{"identity_false", "identity", map[string]model.TypedValue{"value": boolean(false)}, boolean(false)},
		{"identity_zero", "identity", map[string]model.TypedValue{"value": integer(0)}, integer(0)},
		{"identity_empty", "identity", map[string]model.TypedValue{"value": textValue("")}, textValue("")},
		{"equal_boolean", "equal", map[string]model.TypedValue{"left": boolean(false), "right": boolean(false)}, boolean(true)},
		{"equal_integer", "equal", map[string]model.TypedValue{"left": integer(3), "right": integer(4)}, boolean(false)},
		{"equal_string", "equal", map[string]model.TypedValue{"left": textValue("tenant"), "right": textValue("Tenant")}, boolean(false)},
		{"less_than", "less_than", map[string]model.TypedValue{"left": integer(-1), "right": integer(0)}, boolean(true)},
		{"less_or_equal", "less_or_equal", map[string]model.TypedValue{"left": integer(7), "right": integer(7)}, boolean(true)},
		{"and", "and", map[string]model.TypedValue{"left": boolean(true), "right": boolean(false)}, boolean(false)},
		{"or", "or", map[string]model.TypedValue{"left": boolean(false), "right": boolean(true)}, boolean(true)},
		{"not", "not", map[string]model.TypedValue{"value": boolean(false)}, boolean(true)},
		{"select_integer", "select", map[string]model.TypedValue{"condition": boolean(true), "trueValue": integer(7), "falseValue": integer(9)}, integer(7)},
		{"select_boolean", "select", map[string]model.TypedValue{"condition": boolean(false), "trueValue": boolean(true), "falseValue": boolean(false)}, boolean(false)},
		{"select_string", "select", map[string]model.TypedValue{"condition": boolean(false), "trueValue": textValue("yes"), "falseValue": textValue("no")}, textValue("no")},
		{"add", "add", map[string]model.TypedValue{"left": integer(19), "right": integer(23)}, integer(42)},
		{"subtract", "subtract", map[string]model.TypedValue{"left": integer(-19), "right": integer(23)}, integer(-42)},
		{"concat", "concat", map[string]model.TypedValue{"left": textValue("α"), "right": textValue("β")}, textValue("αβ")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle := bundleDefinition(t, primitiveDefinition(t, tc.op, tc.inputs))
			r := evaluateOK(t, bundle, tc.inputs)
			if !equalValue(r.Outputs["result"], tc.want) || r.Operations != 1 || r.Invocations != 1 {
				t.Fatalf("primitive %s: %+v", tc.op, r)
			}
			if r.Events[1].Path != "$/operation" || r.Events[1].Kind != "primitive" || r.Events[1].Op != tc.op {
				t.Fatalf("primitive evidence: %+v", r.Events)
			}
		})
	}
}

func TestStrictInputsAndSafeValueBoundaries(t *testing.T) {
	base := map[string]model.TypedValue{"value": integer(1)}
	bundle := bundleDefinition(t, primitiveDefinition(t, "identity", base))
	for _, values := range []map[string]model.TypedValue{
		{}, {"value": boolean(false)}, {"value": integer(1), "extra": integer(2)},
		{"value": {Type: "integer"}}, {"value": {Type: "integer", Integer: integer(1).Integer, Boolean: boolean(true).Boolean}},
		{"value": integer(model.MaxBehaviorInteger + 1)}, {"value": integer(-model.MaxBehaviorInteger - 1)},
	} {
		if result, err := Evaluate(context.Background(), bundle, values); err == nil || result.EngineVersion != "" || !strings.Contains(err.Error(), "$") {
			t.Fatalf("invalid inputs became a decision: %+v, %v", result, err)
		}
	}
	for _, value := range []int64{-model.MaxBehaviorInteger, model.MaxBehaviorInteger} {
		r := evaluateOK(t, bundle, map[string]model.TypedValue{"value": integer(value)})
		if *r.Outputs["result"].Integer != value {
			t.Fatal("safe integer boundary changed")
		}
	}
	textBundle := bundleDefinition(t, primitiveDefinition(t, "identity", map[string]model.TypedValue{"value": textValue("")}))
	for _, value := range []string{strings.Repeat("x", 4097), "a\x00b", "\xff", "\x01"} {
		if _, err := Evaluate(context.Background(), textBundle, map[string]model.TypedValue{"value": textValue(value)}); err == nil {
			t.Fatal("invalid text accepted")
		}
	}
	for _, value := range []string{"", "\n\t\r", strings.Repeat("界", 1365) + "x"} {
		evaluateOK(t, textBundle, map[string]model.TypedValue{"value": textValue(value)})
	}
}

func TestArithmeticOverflowAndStringLimits(t *testing.T) {
	for _, tc := range []struct {
		op          string
		left, right int64
	}{
		{"add", model.MaxBehaviorInteger, 1}, {"add", -model.MaxBehaviorInteger, -1},
		{"subtract", model.MaxBehaviorInteger, -1}, {"subtract", -model.MaxBehaviorInteger, 1},
	} {
		inputs := map[string]model.TypedValue{"left": integer(tc.left), "right": integer(tc.right)}
		bundle := bundleDefinition(t, primitiveDefinition(t, tc.op, inputs))
		if r, err := Evaluate(context.Background(), bundle, inputs); err == nil || r.EngineVersion != "" || !strings.Contains(err.Error(), "$/operation") {
			t.Fatalf("overflow returned partial evidence: %+v, %v", r, err)
		}
	}
	inputs := map[string]model.TypedValue{"left": textValue(strings.Repeat("界", 1365)), "right": textValue("x")}
	bundle := bundleDefinition(t, primitiveDefinition(t, "concat", inputs))
	if r := evaluateOK(t, bundle, inputs); len(*r.Outputs["result"].String) != 4096 {
		t.Fatal("concat exact byte bound failed")
	}
	inputs["right"] = textValue("xx")
	if _, err := Evaluate(context.Background(), bundle, inputs); err == nil {
		t.Fatal("concat byte overflow accepted")
	}
}

func TestSelectIsEagerAndDoesNotHideFailure(t *testing.T) {
	p := definition("test.eager", model.Behavior{Inputs: []model.BehaviorPort{}, Outputs: []model.BehaviorPort{{Name: "result", Type: "integer"}},
		Nodes: []model.BehaviorNode{
			{ID: "overflow", Op: "add", Inputs: map[string]model.BehaviorBinding{"left": literal(integer(model.MaxBehaviorInteger)), "right": literal(integer(1))}},
			{ID: "select", Op: "select", Inputs: map[string]model.BehaviorBinding{"condition": literal(boolean(false)), "trueValue": output("overflow"), "falseValue": literal(integer(0))}},
		}, Bindings: map[string]model.BehaviorBinding{"result": output("select")}})
	if _, err := Evaluate(context.Background(), bundleDefinition(t, p), nil); err == nil || !strings.Contains(err.Error(), "$/overflow") {
		t.Fatalf("eager selection hid overflow: %v", err)
	}
}

func TestStableTopologicalOrderAndImmutableInputs(t *testing.T) {
	p := definition("test.topology", model.Behavior{Inputs: []model.BehaviorPort{{Name: "value", Type: "integer"}}, Outputs: []model.BehaviorPort{{Name: "first", Type: "integer"}, {Name: "second", Type: "integer"}},
		Nodes: []model.BehaviorNode{
			{ID: "z", Op: "identity", Inputs: map[string]model.BehaviorBinding{"value": input("value")}},
			{ID: "b", Op: "add", Inputs: map[string]model.BehaviorBinding{"left": output("a"), "right": literal(integer(1))}},
			{ID: "a", Op: "identity", Inputs: map[string]model.BehaviorBinding{"value": input("value")}},
		}, Bindings: map[string]model.BehaviorBinding{"first": output("b"), "second": output("z")}})
	bundle := bundleDefinition(t, p)
	inputs := map[string]model.TypedValue{"value": integer(41)}
	beforeBundle, _ := json.Marshal(bundle)
	beforeInputs, _ := json.Marshal(inputs)
	r := evaluateOK(t, bundle, inputs)
	paths := []string{}
	for _, event := range r.Events {
		if event.Kind == "primitive" {
			paths = append(paths, event.Path)
		}
	}
	if !reflect.DeepEqual(paths, []string{"$/a", "$/b", "$/z"}) {
		t.Fatalf("ready nodes were not selected lexicographically: %v", paths)
	}
	if !reflect.DeepEqual(r, evaluateOK(t, bundle, inputs)) {
		t.Fatal("identical evaluation was not deterministic")
	}
	afterBundle, _ := json.Marshal(bundle)
	afterInputs, _ := json.Marshal(inputs)
	if string(beforeBundle) != string(afterBundle) || string(beforeInputs) != string(afterInputs) {
		t.Fatal("evaluation mutated input arguments")
	}
	*r.Outputs["second"].Integer = 9
	*r.Events[0].Inputs["value"].Integer = 8
	if *inputs["value"].Integer != 41 || *r.Events[len(r.Events)-1].Outputs["second"].Integer != 41 {
		t.Fatal("result exposed mutable aliases to inputs or other evidence")
	}
	p.Behavior.Nodes[0], p.Behavior.Nodes[2] = p.Behavior.Nodes[2], p.Behavior.Nodes[0]
	reordered := evaluateOK(t, bundleDefinition(t, p), inputs)
	if r.BundleHash == reordered.BundleHash {
		t.Fatal("exact-content hash omitted array order")
	}
	got := []string{}
	for _, event := range reordered.Events {
		if event.Kind == "primitive" {
			got = append(got, event.Path)
		}
	}
	if !reflect.DeepEqual(paths, got) {
		t.Fatal("node array order affected topological execution")
	}
}

func callChain(id, valueType string, ref model.ComponentRef, count int) model.ComponentPackage {
	b := model.Behavior{Inputs: []model.BehaviorPort{{Name: "value", Type: valueType}}, Outputs: []model.BehaviorPort{{Name: "result", Type: valueType}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{}}
	previous := input("value")
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("call%02d", i)
		b.Nodes = append(b.Nodes, model.BehaviorNode{ID: name, Component: refCopy(ref), Inputs: map[string]model.BehaviorBinding{"value": previous}})
		previous = output(name)
	}
	b.Bindings["result"] = previous
	return definition(id, b)
}

func primitiveChain(id, valueType string, count int) model.ComponentPackage {
	b := model.Behavior{Inputs: []model.BehaviorPort{{Name: "value", Type: valueType}}, Outputs: []model.BehaviorPort{{Name: "result", Type: valueType}}, Nodes: []model.BehaviorNode{}, Bindings: map[string]model.BehaviorBinding{}}
	previous := input("value")
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("node%02d", i)
		b.Nodes = append(b.Nodes, model.BehaviorNode{ID: name, Op: "identity", Inputs: map[string]model.BehaviorBinding{"value": previous}})
		previous = output(name)
	}
	b.Bindings["result"] = previous
	return definition(id, b)
}

func TestExpandedPrimitiveAndInvocationBounds(t *testing.T) {
	leaf := lockDefinition(t, primitiveChain("test.leaf", "integer", 32))
	root := callChain("test.root", "integer", reference(leaf), 8)
	r := evaluateOK(t, bundleDefinition(t, root, leaf), map[string]model.TypedValue{"value": integer(1)})
	if r.Operations != 256 || r.Invocations != 9 {
		t.Fatalf("repeated call expansion count: %+v", r)
	}
	root = callChain("test.root", "integer", reference(leaf), 9)
	if _, err := Evaluate(context.Background(), bundleDefinition(t, root, leaf), map[string]model.TypedValue{"value": integer(1)}); err == nil {
		t.Fatal("expanded primitive bound ignored repeated call occurrences")
	}
	// A zero-primitive leaf still consumes an invocation. Seven branches with
	// 32 invocations each plus one with31, and the root, total exactly256.
	leaf = lockDefinition(t, primitiveChain("test.leaf", "integer", 0))
	large := lockDefinition(t, callChain("test.large", "integer", reference(leaf), 31))
	small := lockDefinition(t, callChain("test.small", "integer", reference(leaf), 30))
	root = callChain("test.root", "integer", reference(large), 8)
	root.Behavior.Nodes[7].Component = refCopy(reference(small))
	r = evaluateOK(t, bundleDefinition(t, root, large, small, leaf), map[string]model.TypedValue{"value": integer(1)})
	if r.Operations != 0 || r.Invocations != 256 {
		t.Fatalf("zero-primitive invocation accounting: ops%d calls%d", r.Operations, r.Invocations)
	}
	root.Behavior.Nodes[7].Component = refCopy(reference(large))
	if _, err := Evaluate(context.Background(), bundleDefinition(t, root, large, leaf), map[string]model.TypedValue{"value": integer(1)}); err == nil {
		t.Fatal("expanded invocation bound ignored repeated calls")
	}
}

func TestNestingBoundAndRuntimeDefenses(t *testing.T) {
	leaf := lockDefinition(t, primitiveChain("test.level0", "integer", 1))
	locks := []model.PackageLock{leaf}
	for depth := 2; depth <= 9; depth++ {
		p := callChain(fmt.Sprintf("test.level%d", depth-1), "integer", reference(locks[0]), 1)
		bundle := bundleDefinition(t, p, locks...)
		if depth <= 8 {
			r := evaluateOK(t, bundle, map[string]model.TypedValue{"value": integer(7)})
			if r.Invocations != depth || r.Operations != 1 {
				t.Fatalf("nested counts: %+v", r)
			}
		} else if _, err := Evaluate(context.Background(), bundle, map[string]model.TypedValue{"value": integer(7)}); err == nil {
			t.Fatal("ninth nesting level accepted")
		}
		locks = bundle.Packages
	}
	bundle := bundleDefinition(t, primitiveChain("test.runtime", "integer", 1))
	prepared, err := prepare(context.Background(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ operations, invocations, depth int }{{MaxOperations, 0, 1}, {0, MaxInvocations, 1}, {0, 0, MaxDepth + 1}} {
		e := evaluator{ctx: context.Background(), prepared: prepared, result: Result{Operations: tc.operations, Invocations: tc.invocations, Outputs: map[string]model.TypedValue{}, Events: []Event{}}}
		if _, err := e.call(prepared.root, map[string]model.TypedValue{"value": integer(1)}, "$", tc.depth); !errors.Is(err, ErrLimit) {
			t.Fatalf("runtime budget defense failed: %v", err)
		}
	}
}

func TestSerializedEvidenceBound(t *testing.T) {
	leaf := lockDefinition(t, primitiveChain("test.largeleaf", "string", 32))
	root := callChain("test.root", "string", reference(leaf), 4)
	bundle := bundleDefinition(t, root, leaf)
	inputs := map[string]model.TypedValue{"value": textValue(strings.Repeat("\n", MaxStringBytes))}
	if r, err := Evaluate(context.Background(), bundle, inputs); !errors.Is(err, ErrLimit) || r.EngineVersion != "" {
		t.Fatalf("escaped JSON evidence must be bounded without partial success: %+v, %v", r, err)
	}
	inputs["value"] = textValue("short")
	evaluateOK(t, bundle, inputs)
}

// stepContext cancels deterministically at a cancellation checkpoint. It avoids
// tests that depend on wall-clock races or machine speed.
type stepContext struct {
	calls atomic.Int32
	after int32
}

func (c *stepContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *stepContext) Done() <-chan struct{}       { return nil }
func (c *stepContext) Value(any) any               { return nil }
func (c *stepContext) Err() error {
	if c.calls.Add(1) >= c.after {
		return context.Canceled
	}
	return nil
}

func TestCancellationReturnsNoPartialResult(t *testing.T) {
	leaf := lockDefinition(t, primitiveChain("test.leaf", "integer", 32))
	bundle := bundleDefinition(t, callChain("test.root", "integer", reference(leaf), 8), leaf)
	inputs := map[string]model.TypedValue{"value": integer(1)}
	for _, after := range []int32{1, 20, 100} {
		ctx := &stepContext{after: after}
		if r, err := Evaluate(ctx, bundle, inputs); !errors.Is(err, context.Canceled) || r.EngineVersion != "" {
			t.Fatalf("cancelled evaluation returned partial evidence: %+v, %v", r, err)
		}
	}
}
