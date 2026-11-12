package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"software-studio/internal/model"
)

func TestCLIProfileStillProducesExactLock(t *testing.T) {
	p := model.ExampleComponentPackages()[0]
	input, _ := json.Marshal(p)
	var output bytes.Buffer
	if err := check([]string{"-"}, bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	var lock model.PackageLock
	if err := json.Unmarshal(output.Bytes(), &lock); err != nil {
		t.Fatal(err)
	}
	expected, _ := model.ComponentDigest(p)
	if lock.Digest != expected || lock.Package.ID != p.ID {
		t.Fatal("profile identity or digest changed")
	}
}

func TestCLIPortableBundleValidationAndTests(t *testing.T) {
	for _, bundle := range model.ExampleComponentBundles() {
		input, _ := json.Marshal(bundle)
		for _, mode := range []string{"--bundle", "--test"} {
			var output bytes.Buffer
			if err := check([]string{mode, "-"}, bytes.NewReader(input), &output); err != nil {
				t.Fatalf("%s %s: %v", bundle.Root.PackageID, mode, err)
			}
			if !json.Valid(output.Bytes()) {
				t.Fatal("CLI did not produce JSON evidence")
			}
		}
	}
}

func TestCLIRejectsIncompleteCompositeManifest(t *testing.T) {
	bundle := model.ExampleComponentBundles()[0]
	for _, lock := range bundle.Packages {
		if lock.Package.ID != bundle.Root.PackageID {
			continue
		}
		input, _ := json.Marshal(lock.Package)
		var output bytes.Buffer
		if err := check([]string{"-"}, bytes.NewReader(input), &output); err == nil || !strings.Contains(err.Error(), "dependency bundle") {
			t.Fatalf("expected bundle guidance, got %v", err)
		}
	}
}

func TestCLIUnverifiedAndFailingSuitesReturnErrorWithEvidence(t *testing.T) {
	for _, empty := range []bool{false, true} {
		bundle := model.ExampleComponentBundles()[0]
		var root model.ComponentPackage
		dependencies := []model.PackageLock{}
		for _, lock := range bundle.Packages {
			if lock.Package.ID == bundle.Root.PackageID {
				root = lock.Package
			} else {
				dependencies = append(dependencies, lock)
			}
		}
		if empty {
			root.Tests = nil
		} else {
			for key, expected := range root.Tests[0].Expected {
				if expected.Boolean != nil {
					value := !*expected.Boolean
					expected.Boolean = &value
					root.Tests[0].Expected[key] = expected
				}
			}
		}
		bundle, issues := model.PrepareComponentBundle(root, dependencies)
		if len(issues) != 0 {
			t.Fatal(issues)
		}
		input, _ := json.Marshal(bundle)
		var output bytes.Buffer
		err := check([]string{"--test", "-"}, bytes.NewReader(input), &output)
		if err == nil || !strings.Contains(output.String(), `"passed": false`) {
			t.Fatalf("empty=%v: expected failed status and JSON evidence, error=%v output=%s", empty, err, output.String())
		}
	}
}

func TestCLIStrictInput(t *testing.T) {
	for _, input := range []string{`null`, `{}`, `{"schemaVersion":1,"unknown":true}`, `{} {}`, strings.Repeat(" ", (2<<20)+1)} {
		var output bytes.Buffer
		if err := check([]string{"--bundle", "-"}, strings.NewReader(input), &output); err == nil || output.Len() != 0 {
			t.Fatal("invalid input returned successful evidence")
		}
	}
}
