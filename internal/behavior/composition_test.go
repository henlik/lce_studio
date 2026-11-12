package behavior

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/model"
)

func flatPolicy(role, scope string) model.ComponentPackage {
	ports := []model.BehaviorPort{{Name: "subjectTenant", Type: "string"}, {Name: "resourceTenant", Type: "string"}, {Name: "role", Type: "string"}, {Name: "requestedScope", Type: "string"}, {Name: "active", Type: "boolean"}}
	return definition("test.flat-policy", model.Behavior{Inputs: ports, Outputs: []model.BehaviorPort{{Name: "permitted", Type: "boolean"}},
		Nodes: []model.BehaviorNode{
			{ID: "tenant", Op: "equal", Inputs: map[string]model.BehaviorBinding{"left": input("subjectTenant"), "right": input("resourceTenant")}},
			{ID: "role", Op: "equal", Inputs: map[string]model.BehaviorBinding{"left": input("role"), "right": literal(textValue(role))}},
			{ID: "scope", Op: "equal", Inputs: map[string]model.BehaviorBinding{"left": input("requestedScope"), "right": literal(textValue(scope))}},
			{ID: "tenantRole", Op: "and", Inputs: map[string]model.BehaviorBinding{"left": output("tenant"), "right": output("role")}},
			{ID: "tenantRoleScope", Op: "and", Inputs: map[string]model.BehaviorBinding{"left": output("tenantRole"), "right": output("scope")}},
			{ID: "decision", Op: "and", Inputs: map[string]model.BehaviorBinding{"left": output("tenantRoleScope"), "right": input("active")}},
		}, Bindings: map[string]model.BehaviorBinding{"permitted": output("decision")}})
}

func TestSharedPolicyAcrossDomainsAndFlatEquivalence(t *testing.T) {
	bundles := model.ExampleComponentBundles()
	if len(bundles) != 2 {
		t.Fatal("expected two reusable-domain examples")
	}
	shared := map[string]string{}
	for i, bundle := range bundles {
		for _, lock := range bundle.Packages {
			if strings.HasPrefix(lock.Package.ID, "lce.") {
				if prior, exists := shared[lock.Package.ID]; exists && prior != lock.Digest {
					t.Fatalf("domain wrapper changed shared definition %s", lock.Package.ID)
				}
				shared[lock.Package.ID] = lock.Digest
			}
		}
		role, scope := "clinician", "record:read"
		if i == 1 {
			role, scope = "teacher", "student:read"
		}
		flat := bundleDefinition(t, flatPolicy(role, scope))
		for mask := 0; mask < 16; mask++ {
			values := map[string]model.TypedValue{"subjectTenant": textValue("tenant-alpha"), "resourceTenant": textValue("tenant-alpha"), "role": textValue(role), "requestedScope": textValue(scope), "active": boolean(mask&8 != 0)}
			if mask&1 == 0 {
				values["resourceTenant"] = textValue("tenant-beta")
			}
			if mask&2 == 0 {
				values["role"] = textValue("visitor")
			}
			if mask&4 == 0 {
				values["requestedScope"] = textValue("other:write")
			}
			nested := evaluateOK(t, bundle, values)
			expanded := evaluateOK(t, flat, values)
			if !equalValues(nested.Outputs, expanded.Outputs) || *nested.Outputs["permitted"].Boolean != (mask == 15) {
				t.Fatalf("policy truth table/domain%d mask%d: nested%+v flat%+v", i, mask, nested.Outputs, expanded.Outputs)
			}
			if nested.Operations != 6 || expanded.Operations != 6 || nested.Invocations != 3 || expanded.Invocations != 1 {
				t.Fatalf("expanded observable operations differ: nested%d/%d flat%d/%d", nested.Operations, nested.Invocations, expanded.Operations, expanded.Invocations)
			}
			// Hierarchical entry/exit evidence deliberately differs from flat
			// evidence while the typed outputs and primitive count agree.
			if len(nested.Events) != 12 || len(expanded.Events) != 8 {
				t.Fatal("composition evidence lost hierarchy")
			}
			paths := map[string]bool{}
			for _, event := range nested.Events {
				paths[event.Path] = true
			}
			if !paths["$"] || !paths["$/policy"] || !paths["$/policy/tenant"] || !paths["$/policy/tenant/compare"] {
				t.Fatalf("missing depth-first nested evidence: %v", paths)
			}
		}
		suite, err := Test(context.Background(), bundle)
		if err != nil || !suite.Passed || len(suite.Cases) != 5 {
			t.Fatalf("root-only domain cases: %+v, %v", suite, err)
		}
		encoded, err := json.Marshal(bundle)
		if err != nil {
			t.Fatal(err)
		}
		var portable model.ComponentBundle
		if err := json.Unmarshal(encoded, &portable); err != nil {
			t.Fatal(err)
		}
		root := bundle.Packages[0].Package
		for _, lock := range bundle.Packages {
			if reference(lock) == bundle.Root {
				root = lock.Package
			}
		}
		values := root.Tests[0].Inputs
		if !reflect.DeepEqual(evaluateOK(t, bundle, values), evaluateOK(t, portable, values)) {
			t.Fatal("portable bundle required external catalog state")
		}
	}
	if len(shared) != 2 {
		t.Fatalf("expected exact policy and tenant-check reuse: %v", shared)
	}
}

func TestDepthFirstCallsAndRootOnlyTests(t *testing.T) {
	leaf := primitiveDefinition(t, "add", map[string]model.TypedValue{"left": integer(0), "right": integer(1)})
	leaf.ID = "test.increment"
	leaf.Behavior.Inputs = []model.BehaviorPort{{Name: "value", Type: "integer"}}
	leaf.Behavior.Nodes[0].Inputs = map[string]model.BehaviorBinding{"left": input("value"), "right": literal(integer(1))}
	leaf.Tests = []model.BehaviorCase{{Name: "Child deliberately mismatched", Inputs: map[string]model.TypedValue{"value": integer(1)}, Expected: map[string]model.TypedValue{"result": integer(999)}}}
	leafLock := lockDefinition(t, leaf)
	middleLock := lockDefinition(t, callChain("test.twice", "integer", reference(leafLock), 2))
	root := callChain("test.wrapper", "integer", reference(middleLock), 1)
	root.Tests = []model.BehaviorCase{{Name: "Root case", Inputs: map[string]model.TypedValue{"value": integer(2)}, Expected: map[string]model.TypedValue{"result": integer(4)}}}
	bundle := bundleDefinition(t, root, middleLock, leafLock)
	r := evaluateOK(t, bundle, map[string]model.TypedValue{"value": integer(2)})
	if *r.Outputs["result"].Integer != 4 || r.Operations != 2 || r.Invocations != 4 {
		t.Fatalf("nested repeated call: %+v", r)
	}
	got := []string{}
	for _, event := range r.Events {
		got = append(got, event.Kind+":"+event.Path)
	}
	want := []string{"component_enter:$", "component_enter:$/call00", "component_enter:$/call00/call00", "primitive:$/call00/call00/operation", "component_exit:$/call00/call00", "component_enter:$/call00/call01", "primitive:$/call00/call01/operation", "component_exit:$/call00/call01", "component_exit:$/call00", "component_exit:$"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("not depth-first: %v", got)
	}
	suite, err := Test(context.Background(), bundle)
	if err != nil || !suite.Passed || len(suite.Cases) != 1 || suite.Cases[0].Name != "Root case" {
		t.Fatalf("test runner executed nested definition tests: %+v, %v", suite, err)
	}
}

func TestCaseOrderFailuresAndEmptySuite(t *testing.T) {
	p := primitiveDefinition(t, "add", map[string]model.TypedValue{"left": integer(0), "right": integer(0)})
	p.Tests = []model.BehaviorCase{
		{Name: "z passes", Inputs: map[string]model.TypedValue{"left": integer(1), "right": integer(2)}, Expected: map[string]model.TypedValue{"result": integer(3)}},
		{Name: "a runtime error", Inputs: map[string]model.TypedValue{"left": integer(model.MaxBehaviorInteger), "right": integer(1)}, Expected: map[string]model.TypedValue{"result": integer(0)}},
		{Name: "b mismatch", Inputs: map[string]model.TypedValue{"left": integer(2), "right": integer(2)}, Expected: map[string]model.TypedValue{"result": integer(9)}},
	}
	r, err := Test(context.Background(), bundleDefinition(t, p))
	if err != nil || r.Passed || len(r.Cases) != 3 {
		t.Fatalf("test suite outcome: %+v, %v", r, err)
	}
	for i := range p.Tests {
		if r.Cases[i].Name != p.Tests[i].Name {
			t.Fatal("test definition order changed")
		}
	}
	if !r.Cases[0].Passed || r.Cases[1].Passed || r.Cases[1].Actual != nil || !strings.Contains(r.Cases[1].Error, "$/operation") || r.Cases[2].Passed || r.Cases[2].Error != "" || *r.Cases[2].Actual["result"].Integer != 4 {
		t.Fatalf("mismatch/error evidence: %+v", r.Cases)
	}
	p.Tests = nil
	r, err = Test(context.Background(), bundleDefinition(t, p))
	if err != nil || r.Passed || r.Cases == nil || len(r.Cases) != 0 {
		t.Fatalf("empty test suite must not imply verification: %+v, %v", r, err)
	}
	boolP := primitiveDefinition(t, "identity", map[string]model.TypedValue{"value": boolean(false)})
	boolP.Tests = []model.BehaviorCase{{Name: "False is a valid decision", Inputs: map[string]model.TypedValue{"value": boolean(false)}, Expected: map[string]model.TypedValue{"result": boolean(false)}}}
	if r, err := Test(context.Background(), bundleDefinition(t, boolP)); err != nil || !r.Passed {
		t.Fatalf("false decision treated as failure: %+v, %v", r, err)
	}
}

func TestTestRunnerBudgetsResetAndCancellation(t *testing.T) {
	leaf := lockDefinition(t, primitiveChain("test.leaf", "integer", 32))
	root := callChain("test.root", "integer", reference(leaf), 8)
	for i := 0; i < 3; i++ {
		root.Tests = append(root.Tests, model.BehaviorCase{Name: fmt.Sprintf("case%d", i), Inputs: map[string]model.TypedValue{"value": integer(int64(i))}, Expected: map[string]model.TypedValue{"result": integer(int64(i))}})
	}
	bundle := bundleDefinition(t, root, leaf)
	if suite, err := Test(context.Background(), bundle); err != nil || !suite.Passed || len(suite.Cases) != 3 {
		t.Fatalf("operation budget leaked between cases: %+v, %v", suite, err)
	}
	if suite, err := Test(&stepContext{after: 700}, bundle); !errors.Is(err, context.Canceled) || suite.Passed || suite.Cases != nil {
		t.Fatalf("cancelled suite returned partial cases: %+v, %v", suite, err)
	}
}

func TestAggregateTestEvidenceBound(t *testing.T) {
	p := primitiveChain("test.expanding-output", "string", 0)
	p.Behavior.Outputs, p.Behavior.Bindings = nil, map[string]model.BehaviorBinding{}
	expected := map[string]model.TypedValue{}
	for i := 0; i < 16; i++ {
		name := fmt.Sprintf("value%02d", i)
		p.Behavior.Outputs = append(p.Behavior.Outputs, model.BehaviorPort{Name: name, Type: "string"})
		p.Behavior.Bindings[name], expected[name] = input("value"), textValue("")
	}
	for i := 0; i < 17; i++ {
		p.Tests = append(p.Tests, model.BehaviorCase{Name: fmt.Sprintf("case%02d", i), Inputs: map[string]model.TypedValue{"value": textValue(strings.Repeat("x", 4096))}, Expected: cloneValues(expected)})
	}
	bundle := bundleDefinition(t, p)
	// The compact expected values keep this manifest below128KiB, while its
	// mismatching actual outputs would push aggregate case evidence above1MiB.
	if suite, err := Test(context.Background(), bundle); !errors.Is(err, ErrLimit) || suite.Cases != nil {
		t.Fatalf("aggregate test evidence escaped its limit: %+v, %v", suite, err)
	}
}

func TestInvalidBundlesNeverReturnPartialEvidence(t *testing.T) {
	base := model.ExampleComponentBundles()[0]
	for _, mutate := range []func(*model.ComponentBundle){
		func(b *model.ComponentBundle) { b.Packages = b.Packages[:1] },
		func(b *model.ComponentBundle) { b.Root.Digest = strings.Repeat("0", 64) },
		func(b *model.ComponentBundle) { b.Packages[0].Package.Name = "tampered" },
	} {
		encoded, _ := json.Marshal(base)
		var bundle model.ComponentBundle
		if err := json.Unmarshal(encoded, &bundle); err != nil {
			t.Fatal(err)
		}
		mutate(&bundle)
		if r, err := Evaluate(context.Background(), bundle, nil); err == nil || r.EngineVersion != "" {
			t.Fatalf("invalid closure returned evidence: %+v, %v", r, err)
		}
		if r, err := Test(context.Background(), bundle); err == nil || r.Cases != nil {
			t.Fatalf("invalid closure returned tests: %+v, %v", r, err)
		}
	}
}
