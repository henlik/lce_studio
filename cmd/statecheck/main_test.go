package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"software-studio/internal/model"
	"software-studio/internal/state"
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
	var out bytes.Buffer
	err = check(args, bytes.NewReader(raw), &out)
	return out.String(), err
}

func TestPortableMachineAndScenarios(t *testing.T) {
	examples := model.ExampleStateScenarios()
	if len(examples) != 2 {
		t.Fatal("expected two reference domains")
	}
	for _, ex := range examples {
		raw, err := invoke(t, "", ex.Machine.Machine)
		if err != nil {
			t.Fatal(err)
		}
		var lock model.StateMachineLock
		if json.Unmarshal([]byte(raw), &lock) != nil || lock.Digest != ex.Machine.Digest {
			t.Fatal("machine lock changed")
		}
		if _, err := invoke(t, "--lock", lock); err != nil {
			t.Fatal(err)
		}
		raw, err = invoke(t, "--run", ex)
		if err != nil {
			t.Fatal(err)
		}
		var result state.Result
		if json.Unmarshal([]byte(raw), &result) != nil || !result.Verified || !result.Passed || len(result.Steps) == 0 {
			t.Fatal(raw)
		}
	}
}

func TestFailedExpectationsPrintCompleteVerdict(t *testing.T) {
	ex := model.ExampleStateScenarios()[0]
	ex.Scenario.Expected[0].Receipts++
	raw, err := invoke(t, "--run", ex)
	var result state.Result
	if err == nil || json.Unmarshal([]byte(raw), &result) != nil || !result.Verified || result.Passed || len(result.Expectations) == 0 || len(result.Instances) == 0 {
		t.Fatalf("missing failed verdict: %s %v", raw, err)
	}
}

func TestUnverifiedRunDoesNotClaimPassing(t *testing.T) {
	ex := model.ExampleStateScenarios()[0]
	ex.Scenario.Expected = []model.StateExpectation{}
	raw, err := invoke(t, "--run", ex)
	var result state.Result
	if err != nil || json.Unmarshal([]byte(raw), &result) != nil || result.Verified || result.Passed {
		t.Fatalf("unverified semantics: %s %v", raw, err)
	}
}

func TestStrictInputAndTamperedLockProduceNoEvidence(t *testing.T) {
	for _, raw := range []string{"null", "{} {}", `{"unknown":true}`, strings.Repeat(" ", 2*1024*1024+1)} {
		var out bytes.Buffer
		if err := check([]string{"-"}, strings.NewReader(raw), &out); err == nil || out.Len() != 0 {
			t.Fatal("invalid input accepted")
		}
	}
	ex := model.ExampleStateScenarios()[0]
	ex.Machine.Digest = strings.Repeat("0", 64)
	if raw, err := invoke(t, "--lock", ex.Machine); err == nil || raw != "" {
		t.Fatal("tampered machine accepted")
	}
	if raw, err := invoke(t, "--run", ex); err == nil || raw != "" {
		t.Fatal("tampered execution generated evidence")
	}
}

func TestRunRejectsMissingOrNullExpectationCounts(t *testing.T) {
	raw, err := json.Marshal(model.ExampleStateScenarios()[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"revision", "receipts", "effects"} {
		for _, missing := range []bool{false, true} {
			var input map[string]any
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			expected := input["scenario"].(map[string]any)["expected"].([]any)[0].(map[string]any)
			if missing {
				delete(expected, field)
			} else {
				expected[field] = nil
			}
			if output, err := invoke(t, "--run", input); err == nil || output != "" {
				t.Fatalf("%s missing=%t produced evidence: %s (%v)", field, missing, output, err)
			}
		}
	}
}
