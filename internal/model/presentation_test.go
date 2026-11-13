package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComponentPresentationIsPortableAndValidated(t *testing.T) {
	bundle := ExampleComponentBundles()[0]
	root := bundle.Packages[0].Package
	for _, p := range bundle.Packages {
		if p.Package.ID == bundle.Root.PackageID {
			root = p.Package
		}
	}
	before, _ := ComponentDigest(root)
	root.DesignLevel = "system"
	root.Behavior.Nodes[0].Color = "#2478ba"
	if issues := ValidateComponentPackage(root); len(issues) > 0 {
		t.Fatal(issues)
	}
	encoded, _ := json.Marshal(root)
	var restored ComponentPackage
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	after, _ := ComponentDigest(restored)
	if before == after || restored.DesignLevel != "system" || restored.Behavior.Nodes[0].Color != "#2478ba" {
		t.Fatal("presentation must survive export and be covered by the immutable digest")
	}
	restored.Behavior.Nodes[0].Color = "url(https://invalid.example)"
	if issues := ValidateComponentPackage(restored); !issueAt(issues, "behavior.nodes[0].color") {
		t.Fatal(issues)
	}
	restored.Behavior.Nodes[0].Color = ""
	restored.DesignLevel = "primitive"
	if issues := ValidateComponentPackage(restored); !issueAt(issues, "designLevel") {
		t.Fatal(issues)
	}
	restored.DesignLevel = ""
	encoded, _ = json.Marshal(restored)
	if strings.Contains(string(encoded), `"color"`) || strings.Contains(string(encoded), `"designLevel"`) {
		t.Fatal("unset presentation must preserve the legacy encoding")
	}
}
