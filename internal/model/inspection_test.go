package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInspectionStageMetadataPreservesLegacyIdentity(t *testing.T) {
	legacy := HospitalReceptionWorkflow()
	before, _ := json.Marshal(legacy)
	// Isolate the editable view before applying stage metadata.
	var copy WorkflowDefinition
	data, _ := json.Marshal(legacy.Workflow)
	if err := json.Unmarshal(data, &copy); err != nil {
		t.Fatal(err)
	}
	staged := ReceptionInspectionStages(copy)
	if issues := ValidateWorkflowDefinition(staged); len(issues) > 0 {
		t.Fatal(issues)
	}
	for _, n := range staged.Nodes {
		if n.Stage == "Unassigned" {
			t.Errorf("unassigned step %s", n.ID)
		}
	}
	after, _ := json.Marshal(legacy)
	if string(before) != string(after) {
		t.Fatal("legacy reference mutated")
	}
	if strings.Contains(string(before), `"stage"`) {
		t.Fatal("legacy JSON shape changed")
	}
	for _, invalid := range []string{"a//b", "/a", "a/", "a/b/c/d/e/f/g", strings.Repeat("x", 241)} {
		staged.Nodes[0].Stage = invalid
		if issues := ValidateWorkflowDefinition(staged); len(issues) == 0 {
			t.Errorf("accepted invalid path %q", invalid)
		}
	}
}
func TestInspectionPackCoverage(t *testing.T) {
	p := StudioInspectionPack()
	seen := map[string]bool{}
	workspaces := map[string]bool{}
	for _, c := range p.Cases {
		if seen[c.ID] {
			t.Fatalf("duplicate case %s", c.ID)
		}
		seen[c.ID] = true
		workspaces[c.Workspace] = true
		if c.VerdictPath == "" || c.Body == nil {
			t.Fatalf("missing oracle or fixture: %s", c.ID)
		}
		if body, ok := c.Body.(map[string]any); ok {
			if project, ok := body["project"].(Project); ok {
				if issues := Validate(project); len(issues) > 0 {
					t.Fatalf("%s invalid project: %v", c.ID, issues)
				}
			}
		}
		if c.Workspace == "workflows" {
			e := c.Body.(WorkflowExample)
			if issues := ValidateWorkflowLock(e.Workflow); len(issues) > 0 {
				t.Fatalf("%s: %v", c.ID, issues)
			}
			if e.Scenario.Expected == nil {
				t.Fatalf("%s has no independent expectations", c.ID)
			}
		}
	}
	for _, w := range []string{"workflows", "components", "data", "state", "messages", "design", "deployment"} {
		if !workspaces[w] {
			t.Errorf("missing %s", w)
		}
	}
}
