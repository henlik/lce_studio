package model

import (
	"fmt"
	"testing"
)

func TestProjectRejectsCompositePackageEvenWhenUnused(t *testing.T) {
	p := BranchingExampleProject()
	bundle := ExampleComponentBundles()[0]
	if issues := ValidateComponentBundle(bundle); len(issues) != 0 {
		t.Fatalf("fixture must be a valid portable composite: %v", issues)
	}
	index := len(p.Packages)
	p.Packages = append(p.Packages, bundle.Packages...)
	for _, validate := range []func(Project) []ValidationIssue{ValidateDraft, Validate} {
		found := false
		for _, issue := range validate(p) {
			if issue.Path == fmt.Sprintf("packages[%d].package.schemaVersion", index) {
				found = true
			}
		}
		if !found {
			t.Fatal("application project accepted a composite definition without an explicit unsupported-schema issue")
		}
	}
}
