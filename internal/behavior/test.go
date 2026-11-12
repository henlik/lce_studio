package behavior

import (
	"context"
	"encoding/json"
	"fmt"

	"software-studio/internal/model"
)

type TestResult struct {
	Passed bool         `json:"passed"`
	Cases  []CaseResult `json:"cases"`
}

type CaseResult struct {
	Name     string                      `json:"name"`
	Passed   bool                        `json:"passed"`
	Expected map[string]model.TypedValue `json:"expected"`
	Actual   map[string]model.TypedValue `json:"actual,omitempty"`
	Error    string                      `json:"error,omitempty"`
}

// Test evaluates only the root definition's cases, in definition order. A false
// decision is an ordinary typed value, not an evaluator error. Runtime failures
// fail individual cases; invalid bundles, cancellation and oversized aggregate
// test evidence abort the suite without a successful partial result.
func Test(ctx context.Context, bundle model.ComponentBundle) (TestResult, error) {
	prepared, err := prepare(ctx, bundle)
	if err != nil {
		return TestResult{}, err
	}
	definition := prepared.components[prepared.root].definition
	result := TestResult{Passed: len(definition.Tests) > 0, Cases: []CaseResult{}}
	for _, tc := range definition.Tests {
		if err := ctx.Err(); err != nil {
			return TestResult{}, fmt.Errorf("$: test %s: %w", tc.Name, err)
		}
		out, evaluationErr := evaluatePrepared(ctx, prepared, tc.Inputs)
		if err := ctx.Err(); err != nil {
			return TestResult{}, fmt.Errorf("$: test %s: %w", tc.Name, err)
		}
		caseResult := CaseResult{Name: tc.Name, Expected: cloneValues(tc.Expected)}
		if evaluationErr != nil {
			caseResult.Error = evaluationErr.Error()
		} else {
			caseResult.Actual = cloneValues(out.Outputs)
			caseResult.Passed = equalValues(tc.Expected, out.Outputs)
		}
		result.Passed = result.Passed && caseResult.Passed
		result.Cases = append(result.Cases, caseResult)
		encoded, err := json.Marshal(result)
		if err != nil {
			return TestResult{}, fmt.Errorf("$: encode test result: %w", err)
		}
		if len(encoded) > MaxResultBytes {
			return TestResult{}, fmt.Errorf("$: %w: serialized test result exceeds %d bytes", ErrLimit, MaxResultBytes)
		}
	}
	if err := ctx.Err(); err != nil {
		return TestResult{}, fmt.Errorf("$: %w", err)
	}
	return result, nil
}

func equalValues(left, right map[string]model.TypedValue) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		other, found := right[key]
		if !found || !equalValue(value, other) {
			return false
		}
	}
	return true
}
