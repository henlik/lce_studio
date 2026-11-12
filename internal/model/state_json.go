package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// UnmarshalJSON keeps required expectation numbers distinct from a supplied
// zero. It preserves strict decoding even when called by an outer decoder.
func (expectation *StateExpectation) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("state expectation must be a nonnull object")
	}
	type plain StateExpectation
	var decoded plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("state expectation must contain exactly one JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, required := range []string{"revision", "receipts", "effects"} {
		present := false
		for name, raw := range fields {
			if strings.EqualFold(name, required) {
				present = true
				if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
					return fmt.Errorf("state expectation %s must be a nonnull integer", required)
				}
			}
		}
		if !present {
			return fmt.Errorf("state expectation %s is required", required)
		}
	}
	*expectation = StateExpectation(decoded)
	return nil
}
