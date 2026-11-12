package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/workflow"
)

func invoke(t *testing.T, mode string, value any) (string, error) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-"}
	if mode != "" {
		args = []string{mode, "-"}
	}
	var output bytes.Buffer
	err = check(args, bytes.NewReader(raw), &output)
	return output.String(), err
}

func TestPortableWorkflowExamples(t *testing.T) {
	examples := model.ExampleWorkflowScenarios()
	if len(examples) == 0 {
		t.Fatal("expected reference workflows")
	}
	for _, example := range examples {
		raw, err := invoke(t, "", example.Workflow.Workflow)
		if err != nil {
			t.Fatal(err)
		}
		var lock model.WorkflowLock
		if json.Unmarshal([]byte(raw), &lock) != nil || lock.Digest != example.Workflow.Digest {
			t.Fatal("workflow package lock changed")
		}
		if _, err := invoke(t, "--lock", lock); err != nil {
			t.Fatal(err)
		}
		raw, err = invoke(t, "--run", example)
		var result workflow.Result
		if err != nil || json.Unmarshal([]byte(raw), &result) != nil || !result.Verified || !result.Passed || len(result.Steps) == 0 {
			t.Fatalf("missing complete passing evidence for %s: %s %v", example.Scenario.Name, raw, err)
		}
	}
}

func TestFailedAndUnverifiedExpectations(t *testing.T) {
	example := model.ExampleWorkflowScenarios()[0]
	if example.Scenario.Expected == nil {
		t.Fatal("example needs independent expectations")
	}
	example.Scenario.Expected.Tasks++
	raw, err := invoke(t, "--run", example)
	var failed workflow.Result
	if err == nil || json.Unmarshal([]byte(raw), &failed) != nil || !failed.Verified || failed.Passed || len(failed.Issues) == 0 {
		t.Fatalf("missing failed verdict: %s %v", raw, err)
	}
	example.Scenario.Expected = nil
	example.Scenario.Events = []model.WorkflowEvent{}
	raw, err = invoke(t, "--run", example)
	var waiting workflow.Result
	if err != nil || json.Unmarshal([]byte(raw), &waiting) != nil || waiting.Status != "waiting" || waiting.Task == nil || waiting.Verified || waiting.Passed {
		t.Fatalf("waiting/unverified scenario claimed completion or a pass: %s %v", raw, err)
	}
}

func TestStrictInputAndTamperedLock(t *testing.T) {
	for _, raw := range []string{"null", "{} {}", `{"unknown":true}`, strings.Repeat(" ", 2*1024*1024+1)} {
		var output bytes.Buffer
		if err := check([]string{"-"}, strings.NewReader(raw), &output); err == nil || output.Len() != 0 {
			t.Fatal("invalid workflow produced output")
		}
	}
	example := model.ExampleWorkflowScenarios()[0]
	example.Workflow.Digest = strings.Repeat("0", 64)
	for _, mode := range []string{"--lock", "--run"} {
		var value any = example
		if mode == "--lock" {
			value = example.Workflow
		}
		if output, err := invoke(t, mode, value); err == nil || output != "" {
			t.Fatal("tampered lock produced output")
		}
	}
}

func TestRequiredExpectationCounters(t *testing.T) {
	example := model.ExampleWorkflowScenarios()[0]
	raw, err := json.Marshal(example)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tasks", "completedTasks", "cancelledTasks", "eventReceipts"} {
		for _, missing := range []bool{false, true} {
			var input map[string]json.RawMessage
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			var scenario map[string]json.RawMessage
			if err := json.Unmarshal(input["scenario"], &scenario); err != nil {
				t.Fatal(err)
			}
			var expected map[string]json.RawMessage
			if err := json.Unmarshal(scenario["expected"], &expected); err != nil {
				t.Fatal(err)
			}
			if missing {
				delete(expected, field)
			} else {
				expected[field] = json.RawMessage("null")
			}
			scenario["expected"], err = json.Marshal(expected)
			if err != nil {
				t.Fatal(err)
			}
			input["scenario"], err = json.Marshal(scenario)
			if err != nil {
				t.Fatal(err)
			}
			if output, err := invoke(t, "--run", input); err == nil || output != "" {
				t.Fatalf("missing/null %s produced evidence", field)
			}
		}
	}
}

func TestFileAndUsage(t *testing.T) {
	example := model.ExampleWorkflowScenarios()[0]
	raw, err := json.Marshal(example.Workflow.Workflow)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "workflow.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := check([]string{path}, strings.NewReader("ignored"), &output); err != nil {
		t.Fatal(err)
	}
	var lock model.WorkflowLock
	if err := json.Unmarshal(output.Bytes(), &lock); err != nil || lock.Digest != example.Workflow.Digest {
		t.Fatalf("file preparation: %s %v", output.String(), err)
	}
	for _, args := range [][]string{nil, {"--unknown", "-"}, {path, path}, {filepath.Join(t.TempDir(), "missing.json")}} {
		output.Reset()
		if err := check(args, strings.NewReader("{}"), &output); err == nil || output.Len() != 0 {
			t.Fatalf("invalid arguments produced evidence: %#v", args)
		}
	}
}
