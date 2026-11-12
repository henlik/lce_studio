package model

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestComponentPackageValidation(t *testing.T) {
	for _, pkg := range ExampleComponentPackages() {
		if issues := ValidateComponentPackage(pkg); len(issues) != 0 {
			t.Fatalf("bundled %s rejected: %+v", pkg.ID, issues)
		}
	}
	tests := []struct {
		name, path string
		mutate     func(*ComponentPackage)
	}{
		{"schema", "schemaVersion", func(p *ComponentPackage) { p.SchemaVersion = 4 }},
		{"namespace", "id", func(p *ComponentPackage) { p.ID = "worker" }},
		{"unsafe ID", "id", func(p *ComponentPackage) { p.ID = "example/worker" }},
		{"long ID", "id", func(p *ComponentPackage) { p.ID = strings.Repeat("a", 128) + ".worker" }},
		{"version range", "version", func(p *ComponentPackage) { p.Version = "^1.0.0" }},
		{"version suffix", "version", func(p *ComponentPackage) { p.Version = "1.0.0-rc1" }},
		{"version leading zero", "version", func(p *ComponentPackage) { p.Version = "1.01.0" }},
		{"version incomplete", "version", func(p *ComponentPackage) { p.Version = "1.0" }},
		{"version long", "version", func(p *ComponentPackage) { p.Version = strings.Repeat("9", 29) + ".0.0" }},
		{"blank name", "name", func(p *ComponentPackage) { p.Name = " \t" }},
		{"long description", "description", func(p *ComponentPackage) { p.Description = strings.Repeat("x", 4001) }},
		{"invalid UTF8", "description", func(p *ComponentPackage) { p.Description = string([]byte{255}) }},
		{"control character", "author", func(p *ComponentPackage) { p.Author = "name\x01" }},
		{"missing license", "license", func(p *ComponentPackage) { p.License = "" }},
		{"executable kernel", "kernel", func(p *ComponentPackage) { p.Kernel = "javascript" }},
		{"client kernel", "kernel", func(p *ComponentPackage) { p.Kernel = "client" }},
		{"missing capabilities", "capabilities", func(p *ComponentPackage) { p.Capabilities = nil }},
		{"extra capability", "capabilities", func(p *ComponentPackage) { p.Capabilities = append(p.Capabilities, "deploy") }},
		{"capability order", "capabilities", func(p *ComponentPackage) { p.Capabilities = []string{"simulate", "design"} }},
		{"negative processing", "config.processingMs.min", func(p *ComponentPackage) { p.Config.ProcessingMs.Min = -1 }},
		{"processing maximum", "config.processingMs.max", func(p *ComponentPackage) { p.Config.ProcessingMs.Max = 60001 }},
		{"capacity minimum", "config.capacity.min", func(p *ComponentPackage) { p.Config.Capacity.Min = 0 }},
		{"capacity maximum", "config.capacity.max", func(p *ComponentPackage) { p.Config.Capacity.Max = 101 }},
		{"memory minimum", "config.memoryMb.min", func(p *ComponentPackage) { p.Config.MemoryMB.Min = 0 }},
		{"memory maximum", "config.memoryMb.max", func(p *ComponentPackage) { p.Config.MemoryMB.Max = 1048577 }},
		{"reversed interval", "config.capacity", func(p *ComponentPackage) { p.Config.Capacity.Min, p.Config.Capacity.Max = 3, 2 }},
		{"default outside interval", "config.capacity.default", func(p *ComponentPackage) { p.Config.Capacity.Default = 101 }},
		{"service deduplication", "config.deduplicate", func(p *ComponentPackage) { p.Config.Deduplicate = true }},
		{"router deduplication", "config.deduplicate", func(p *ComponentPackage) { p.Kernel, p.Config.Deduplicate = "router", true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := ExampleComponentPackages()[0]
			tt.mutate(&pkg)
			if issues := ValidateComponentPackage(pkg); !issueAt(issues, tt.path) {
				t.Fatalf("expected issue at %s; got %+v", tt.path, issues)
			}
		})
	}
}

func TestComponentDigestIsTypedAndContentSensitive(t *testing.T) {
	pkg := ExampleComponentPackages()[0]
	want := "feb920153cc02adf0cc4c242b5adc9f59dd7140389cf28cb04bdcf392efde8d4"
	digest, err := ComponentDigest(pkg)
	if err != nil || digest != want {
		t.Fatalf("typed encoding contract changed: %s, %v", digest, err)
	}
	encoded, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	reordered, err := json.MarshalIndent(fields, "", "    ")
	if err != nil {
		t.Fatal(err)
	}
	var imported ComponentPackage
	if err := json.Unmarshal(reordered, &imported); err != nil {
		t.Fatal(err)
	}
	if got, err := ComponentDigest(imported); err != nil || got != want {
		t.Fatalf("input whitespace/key order affected digest: %s, %v", got, err)
	}
	imported.Config.ProcessingMs.Default++
	if changed, err := ComponentDigest(imported); err != nil || changed == want {
		t.Fatalf("content edit did not change digest: %s, %v", changed, err)
	}
}

// Re-lock an intentionally edited portable definition without consulting a catalog.
func refreshTestLock(t *testing.T, p *Project, index int) {
	t.Helper()
	lock := &p.Packages[index]
	digest, err := ComponentDigest(lock.Package)
	if err != nil {
		t.Fatal(err)
	}
	lock.Digest = digest
	for i := range p.Nodes {
		ref := p.Nodes[i].Component
		if ref != nil && ref.PackageID == lock.Package.ID && ref.Version == lock.Package.Version {
			ref.Digest = digest
		}
	}
}

func TestProjectPackageLocksAndReferences(t *testing.T) {
	tests := []struct {
		name, path string
		mutate     func(*Project)
	}{
		{"tampered content", "packages[0].digest", func(p *Project) { p.Packages[0].Package.Name += " changed" }},
		{"uppercase digest", "packages[0].digest", func(p *Project) { p.Packages[0].Digest = strings.ToUpper(p.Packages[0].Digest) }},
		{"duplicate lock", "packages[3].package", func(p *Project) { p.Packages = append(p.Packages, p.Packages[0]) }},
		{"too many locks", "packages", func(p *Project) { p.Packages = make([]PackageLock, 51) }},
		{"invalid manifest", "packages[0].package.kernel", func(p *Project) { p.Packages[0].Package.Kernel = "script" }},
		{"missing locks", "nodes[1].component", func(p *Project) { p.Packages = nil }},
		{"missing ID", "nodes[1].component", func(p *Project) { p.Nodes[1].Component.PackageID = "missing.package" }},
		{"wrong version", "nodes[1].component", func(p *Project) { p.Nodes[1].Component.Version = "2.0.0" }},
		{"wrong digest", "nodes[1].component.digest", func(p *Project) { p.Nodes[1].Component.Digest = strings.Repeat("0", 64) }},
		{"kernel mismatch", "nodes[1].type", func(p *Project) { p.Nodes[1].Type = "service" }},
		{"processing below minimum", "nodes[2].config.processingMs", func(p *Project) {
			p.Packages[0].Package.Config.ProcessingMs.Min = 20
			refreshTestLock(t, p, 0)
			p.Nodes[2].Config.ProcessingMs = 19
		}},
		{"capacity above maximum", "nodes[2].config.capacity", func(p *Project) {
			p.Packages[0].Package.Config.Capacity.Max = 2
			refreshTestLock(t, p, 0)
			p.Nodes[2].Config.Capacity = 3
		}},
		{"memory above maximum", "nodes[2].config.memoryMb", func(p *Project) { p.Nodes[2].Config.MemoryMB = 8193 }},
		{"omitted memory uses effective default", "nodes[2].config.memoryMb", func(p *Project) {
			p.Packages[0].Package.Config.MemoryMB.Min = 65
			refreshTestLock(t, p, 0)
			p.Nodes[2].Config.MemoryMB = 0
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := BranchingExampleProject()
			tt.mutate(&p)
			if issues := ValidateDraft(p); !issueAt(issues, tt.path) {
				t.Fatalf("expected issue at %s; got %+v", tt.path, issues)
			}
		})
	}
	p := BranchingExampleProject()
	p.Infrastructure = nil
	p.Packages[0].Package.Name = "Portable locally edited profile"
	refreshTestLock(t, &p, 0)
	p.Nodes[2].Config.MemoryMB = 0        // 64MiB is inside this profile.
	p.Nodes[3].Config.Deduplicate = false // DB defaults do not lock the checkbox.
	extra := ExampleComponentPackages()[0]
	extra.ID = "another.unused-worker"
	digest, err := ComponentDigest(extra)
	if err != nil {
		t.Fatal(err)
	}
	p.Packages = append(p.Packages, PackageLock{Package: extra, Digest: digest})
	before, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("consistent portable locks, unused profile or editable defaults rejected: %+v", issues)
	}
	after, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("validation mutated or clamped the project")
	}
}

func TestComponentAndWorkflowReferenceFiles(t *testing.T) {
	paths := []string{"queued-worker", "durable-store", "weighted-router"}
	for i, pkg := range ExampleComponentPackages() {
		raw, err := os.ReadFile("../../examples/components/" + paths[i] + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var imported ComponentPackage
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&imported); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(pkg, imported) {
			t.Fatalf("package fixture %s differs from API example", paths[i])
		}
	}
	raw, err := os.ReadFile("../../examples/branching-workflow.json")
	if err != nil {
		t.Fatal(err)
	}
	var imported Project
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&imported); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(imported, BranchingExampleProject()) {
		t.Fatal("branch fixture differs from API example")
	}
	if issues := Validate(imported); len(issues) != 0 {
		t.Fatalf("imported fixture rejected: %+v", issues)
	}
	packages := ExampleComponentPackages()
	packages[0].Capabilities[0] = "mutated"
	if ExampleComponentPackages()[0].Capabilities[0] != "design" {
		t.Fatal("example packages share mutable arrays")
	}
}
