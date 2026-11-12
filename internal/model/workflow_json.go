package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// UnmarshalJSON preserves an independently supplied zero or empty expected
// outcome instead of silently manufacturing it from an absent/null field.
func (expectation *WorkflowExpectation) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("workflow expectation must be a nonnull object")
	}
	type plain WorkflowExpectation
	var decoded plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("workflow expectation must contain exactly one JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, required := range []string{"status", "outcome", "activeTaskNode", "tasks", "completedTasks", "cancelledTasks", "eventReceipts"} {
		present := false
		for name, raw := range fields {
			if strings.EqualFold(name, required) {
				present = true
				if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
					return fmt.Errorf("workflow expectation %s must be nonnull", required)
				}
			}
		}
		if !present {
			return fmt.Errorf("workflow expectation %s is required", required)
		}
	}
	*expectation = WorkflowExpectation(decoded)
	return nil
}
