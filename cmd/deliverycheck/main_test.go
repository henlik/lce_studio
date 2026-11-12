package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"software-studio/internal/delivery"
	"software-studio/internal/model"
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

func TestPortableDeliveryExamples(t *testing.T) {
	examples := model.ExampleDeliveryScenarios()
	if len(examples) != 2 {
		t.Fatal("expected two reference domains")
	}
	for _, example := range examples {
		raw, err := invoke(t, "", example.Model.Model)
		if err != nil {
			t.Fatal(err)
		}
		var lock model.DeliveryModelLock
		if json.Unmarshal([]byte(raw), &lock) != nil || lock.Digest != example.Model.Digest {
			t.Fatal("delivery model lock changed")
		}
		if _, err := invoke(t, "--lock", lock); err != nil {
			t.Fatal(err)
		}
		raw, err = invoke(t, "--run", example)
		var result delivery.Result
		if err != nil || json.Unmarshal([]byte(raw), &result) != nil || !result.Verified || !result.Passed || len(result.Messages) == 0 {
			t.Fatalf("missing complete passing evidence: %s %v", raw, err)
		}
	}
}

func TestFailedAndUnverifiedExpectations(t *testing.T) {
	example := model.ExampleDeliveryScenarios()[0]
	example.Scenario.ExpectedMessages[0].Attempts++
	raw, err := invoke(t, "--run", example)
	var failed delivery.Result
	if err == nil || json.Unmarshal([]byte(raw), &failed) != nil || !failed.Verified || failed.Passed || len(failed.MessageExpectations) == 0 {
		t.Fatalf("missing failed message verdict: %s %v", raw, err)
	}
	example.Scenario.Expected = []model.StateExpectation{}
	example.Scenario.ExpectedMessages = []model.DeliveryMessageExpectation{}
	raw, err = invoke(t, "--run", example)
	var unverified delivery.Result
	if err != nil || json.Unmarshal([]byte(raw), &unverified) != nil || unverified.Verified || unverified.Passed {
		t.Fatalf("unverified scenario claimed a pass: %s %v", raw, err)
	}
}

func TestStrictInputAndTamperedLock(t *testing.T) {
	for _, raw := range []string{"null", "{} {}", `{"unknown":true}`, strings.Repeat(" ", 2*1024*1024+1)} {
		var out bytes.Buffer
		if err := check([]string{"-"}, strings.NewReader(raw), &out); err == nil || out.Len() != 0 {
			t.Fatal("invalid model produced output")
		}
	}
	example := model.ExampleDeliveryScenarios()[0]
	example.Model.Digest = strings.Repeat("0", 64)
	for _, mode := range []string{"--lock", "--run"} {
		var value any = example
		if mode == "--lock" {
			value = example.Model
		}
		if output, err := invoke(t, mode, value); err == nil || output != "" {
			t.Fatal("tampered lock produced output")
		}
	}
}

func TestMissingAndNullMessageAttemptsDoNotProduceEvidence(t *testing.T) {
	raw, err := json.Marshal(model.ExampleDeliveryScenarios()[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, missing := range []bool{false, true} {
		var input map[string]any
		if err := json.Unmarshal(raw, &input); err != nil {
			t.Fatal(err)
		}
		expected := input["scenario"].(map[string]any)["expectedMessages"].([]any)[0].(map[string]any)
		if missing {
			delete(expected, "attempts")
		} else {
			expected["attempts"] = nil
		}
		if output, err := invoke(t, "--run", input); err == nil || output != "" {
			t.Fatal("missing/null attempt count produced evidence")
		}
	}
}
