package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func compositeTestPackage(id string) ComponentPackage {
	return ComponentPackage{SchemaVersion: 2, ID: id, Version: "1.0.0", Name: "Test composite", Author: "Test", License: "MIT", Kernel: "composite", Capabilities: []string{"design", "evaluate"}, Behavior: &Behavior{Inputs: []BehaviorPort{{Name: "value", Type: "boolean"}}, Outputs: []BehaviorPort{{Name: "result", Type: "boolean"}}, Nodes: []BehaviorNode{}, Bindings: map[string]BehaviorBinding{"result": {Input: "value"}}}}
}

func compositeTestLock(t *testing.T, p ComponentPackage) PackageLock {
	t.Helper()
	digest, err := ComponentDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	return PackageLock{Package: p, Digest: digest}
}

func compositeTestRef(lock PackageLock) ComponentRef {
	return ComponentRef{PackageID: lock.Package.ID, Version: lock.Package.Version, Digest: lock.Digest}
}

func compositeRelockRoot(t *testing.T, bundle *ComponentBundle) {
	t.Helper()
	bundle.Packages[0] = compositeTestLock(t, bundle.Packages[0].Package)
	bundle.Root = compositeTestRef(bundle.Packages[0])
}

func TestCompositionReferenceFilesAndSharedExactCore(t *testing.T) {
	examples := ExampleComponentBundles()
	for i, name := range []string{"health-record-access", "school-record-access"} {
		raw, err := os.ReadFile("../../examples/composites/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var imported ComponentBundle
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&imported); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(imported, examples[i]) {
			t.Fatalf("%s fixture differs from helper", name)
		}
		if issues := ValidateComponentBundle(imported); len(issues) != 0 {
			t.Fatal(issues)
		}
	}
	for i := 1; i <= 2; i++ {
		if !reflect.DeepEqual(examples[0].Packages[i], examples[1].Packages[i]) {
			t.Fatal("domain examples do not pin the exact same policy and tenant check")
		}
	}
	before, err := json.Marshal(examples[0])
	if err != nil {
		t.Fatal(err)
	}
	prepared, issues := PrepareComponentBundle(examples[0].Packages[0].Package, examples[0].Packages[1:])
	if len(issues) != 0 || !reflect.DeepEqual(prepared, examples[0]) {
		t.Fatalf("portable preparation changed definition: %+v", issues)
	}
	after, err := json.Marshal(examples[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("validation/preparation mutated the input bundle")
	}
	changed := examples[0].Packages[0].Package
	changed.Name += " revised"
	if digest, err := ComponentDigest(changed); err != nil || digest == examples[0].Root.Digest {
		t.Fatal("versioned display name did not affect exact-content identity")
	}
}

func TestCompositionBundleRejectsClosureAndGraphErrors(t *testing.T) {
	tests := []struct {
		name, path string
		mutate     func(*ComponentBundle)
		relock     bool
	}{
		{"bundle version", "schemaVersion", func(b *ComponentBundle) { b.SchemaVersion = 2 }, false},
		{"empty locks", "packages", func(b *ComponentBundle) { b.Packages = nil }, false},
		{"lock count", "packages", func(b *ComponentBundle) { b.Packages = make([]PackageLock, 51) }, false},
		{"missing root", "root", func(b *ComponentBundle) { b.Root.PackageID = "missing.root" }, false},
		{"root digest", "root.digest", func(b *ComponentBundle) { b.Root.Digest = strings.Repeat("0", 64) }, false},
		{"tampered package", "packages[0].digest", func(b *ComponentBundle) { b.Packages[0].Package.Name += " edited" }, false},
		{"duplicate identity", "packages[3].package", func(b *ComponentBundle) { b.Packages = append(b.Packages, b.Packages[0]) }, false},
		{"profile lock", "packages[3].package.schemaVersion", func(b *ComponentBundle) {
			b.Packages = append(b.Packages, compositeTestLock(t, ExampleComponentPackages()[0]))
		}, false},
		{"missing dependency", "packages[0].package.behavior.nodes[0].component", func(b *ComponentBundle) { b.Packages = b.Packages[:1] }, false},
		{"dependency digest", "packages[0].package.behavior.nodes[0].component.digest", func(b *ComponentBundle) {
			b.Packages[0].Package.Behavior.Nodes[0].Component.Digest = strings.Repeat("0", 64)
		}, true},
		{"unused lock", "packages[3]", func(b *ComponentBundle) {
			b.Packages = append(b.Packages, compositeTestLock(t, compositeTestPackage("test.unused")))
		}, false},
		{"missing call input", "packages[0].package.behavior.nodes[0].inputs.active", func(b *ComponentBundle) { delete(b.Packages[0].Package.Behavior.Nodes[0].Inputs, "active") }, true},
		{"extra call input", "packages[0].package.behavior.nodes[0].inputs.extra", func(b *ComponentBundle) {
			b.Packages[0].Package.Behavior.Nodes[0].Inputs["extra"] = BehaviorBinding{Input: "active"}
		}, true},
		{"mistyped call input", "packages[0].package.behavior.nodes[0].inputs.active", func(b *ComponentBundle) {
			b.Packages[0].Package.Behavior.Nodes[0].Inputs["active"] = BehaviorBinding{Input: "role"}
		}, true},
		{"missing output port", "packages[0].package.behavior.bindings.permitted.port", func(b *ComponentBundle) {
			b.Packages[0].Package.Behavior.Bindings["permitted"] = BehaviorBinding{Node: "policy", Port: "missing"}
		}, true},
		{"mistyped output", "packages[0].package.behavior.bindings.permitted", func(b *ComponentBundle) {
			b.Packages[0].Package.Behavior.Bindings["permitted"] = BehaviorBinding{Input: "role"}
		}, true},
		{"self cycle", "packages[0].package.behavior.nodes", func(b *ComponentBundle) {
			b.Packages[0].Package.Behavior.Nodes[0].Inputs["active"] = BehaviorBinding{Node: "policy", Port: "permitted"}
		}, true},
		{"unobservable node", "packages[0].package.behavior.nodes[1]", func(b *ComponentBundle) {
			b.Packages[0].Package.Behavior.Nodes = append(b.Packages[0].Package.Behavior.Nodes, BehaviorNode{ID: "unused", Op: "identity", Inputs: map[string]BehaviorBinding{"value": {Input: "active"}}})
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := ExampleComponentBundles()[0]
			tt.mutate(&bundle)
			if tt.relock {
				compositeRelockRoot(t, &bundle)
			}
			if issues := ValidateComponentBundle(bundle); !issueAt(issues, tt.path) {
				t.Fatalf("expected %s; got %+v", tt.path, issues)
			}
		})
	}
}

func TestCompositeManifestByteBoundAndNoNodeOutputs(t *testing.T) {
	p := compositeTestPackage("test.direct")
	_, issues := PrepareComponentBundle(p, nil)
	if len(issues) != 0 {
		t.Fatalf("direct component rejected: %+v", issues)
	}
	literal := behaviorBool(false)
	p.Behavior.Inputs = []BehaviorPort{}
	p.Behavior.Bindings["result"] = BehaviorBinding{Literal: &literal}
	if _, issues := PrepareComponentBundle(p, nil); len(issues) != 0 {
		t.Fatalf("literal component rejected: %+v", issues)
	}
	p = compositeTestPackage("test.direct")
	p.Behavior.Inputs[0].Type, p.Behavior.Outputs[0].Type = "string", "string"
	for i := 0; i < 17; i++ {
		value := behaviorString(strings.Repeat("x", 4096))
		p.Tests = append(p.Tests, BehaviorCase{Name: fmt.Sprintf("Case %d", i), Inputs: map[string]TypedValue{"value": value}, Expected: map[string]TypedValue{"result": value}})
	}
	if issues := ValidateComponentPackage(p); !issueAt(issues, "") {
		t.Fatalf("oversized canonical manifest accepted: %+v", issues)
	}
	p.Tests = p.Tests[:1]
	if issues := ValidateComponentPackage(p); len(issues) != 0 {
		t.Fatalf("bounded manifest rejected: %+v", issues)
	}
}

// Repeated call occurrences are observable via a linear chain, so no fan-out or
// dead-node shortcut can hide expansion from the validator.
func compositeChain(t *testing.T, levels, callsPerLevel int, primitiveLeaf bool) ComponentBundle {
	t.Helper()
	leaf := compositeTestPackage("test.level0")
	if primitiveLeaf {
		leaf.Behavior.Nodes = []BehaviorNode{{ID: "identity", Op: "identity", Inputs: map[string]BehaviorBinding{"value": {Input: "value"}}}}
		leaf.Behavior.Bindings["result"] = BehaviorBinding{Node: "identity", Port: "result"}
	}
	locks := []PackageLock{compositeTestLock(t, leaf)}
	for level := 1; level < levels; level++ {
		p := compositeTestPackage(fmt.Sprintf("test.level%d", level))
		ref := compositeTestRef(locks[len(locks)-1])
		binding := BehaviorBinding{Input: "value"}
		for n := 0; n < callsPerLevel; n++ {
			id := fmt.Sprintf("call%02d", n)
			p.Behavior.Nodes = append(p.Behavior.Nodes, BehaviorNode{ID: id, Component: &ref, Inputs: map[string]BehaviorBinding{"value": binding}})
			binding = BehaviorBinding{Node: id, Port: "result"}
		}
		p.Behavior.Bindings["result"] = binding
		locks = append(locks, compositeTestLock(t, p))
	}
	return ComponentBundle{SchemaVersion: 1, Root: compositeTestRef(locks[len(locks)-1]), Packages: locks}
}

func TestCompositionDepthAndRepeatedInvocationBounds(t *testing.T) {
	if issues := ValidateComponentBundle(compositeChain(t, 8, 1, true)); len(issues) != 0 {
		t.Fatalf("depth8 rejected: %+v", issues)
	}
	for _, tt := range []struct {
		name    string
		bundle  ComponentBundle
		message string
	}{
		{"depth9", compositeChain(t, 9, 1, true), "depth"},
		{"repeated empty calls", compositeChain(t, 4, 8, false), "invocations"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			issues := ValidateComponentBundle(tt.bundle)
			for _, issue := range issues {
				if strings.Contains(issue.Message, tt.message) {
					return
				}
			}
			t.Fatalf("expected %s bound; got %+v", tt.message, issues)
		})
	}
	leaf := compositeTestPackage("test.operations")
	previous := BehaviorBinding{Input: "value"}
	for i := 0; i < 32; i++ {
		id := fmt.Sprintf("op%02d", i)
		leaf.Behavior.Nodes = append(leaf.Behavior.Nodes, BehaviorNode{ID: id, Op: "identity", Inputs: map[string]BehaviorBinding{"value": previous}})
		previous = BehaviorBinding{Node: id, Port: "result"}
	}
	leaf.Behavior.Bindings["result"] = previous
	leafLock := compositeTestLock(t, leaf)
	root := compositeTestPackage("test.root")
	ref := compositeTestRef(leafLock)
	previous = BehaviorBinding{Input: "value"}
	for i := 0; i < 9; i++ {
		id := fmt.Sprintf("call%02d", i)
		root.Behavior.Nodes = append(root.Behavior.Nodes, BehaviorNode{ID: id, Component: &ref, Inputs: map[string]BehaviorBinding{"value": previous}})
		previous = BehaviorBinding{Node: id, Port: "result"}
	}
	root.Behavior.Bindings["result"] = previous
	if _, issues := PrepareComponentBundle(root, []PackageLock{leafLock}); len(issues) == 0 {
		t.Fatal("288 expanded primitives accepted")
	}
	root.Behavior.Nodes = root.Behavior.Nodes[:8]
	root.Behavior.Bindings["result"] = BehaviorBinding{Node: "call07", Port: "result"}
	if _, issues := PrepareComponentBundle(root, []PackageLock{leafLock}); len(issues) != 0 {
		t.Fatalf("256 primitive boundary rejected: %+v", issues)
	}
}
