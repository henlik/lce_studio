package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// UnmarshalJSON distinguishes an independently supplied zero from absent/null
// attempts and retains strict unknown-field handling through outer decoders.
func (expectation *DeliveryMessageExpectation) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("message expectation must be a nonnull object")
	}
	type plain DeliveryMessageExpectation
	var decoded plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("message expectation must contain exactly one JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	present := false
	for name, raw := range fields {
		if strings.EqualFold(name, "attempts") {
			present = true
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				return fmt.Errorf("message expectation attempts must be a nonnull integer")
			}
		}
	}
	if !present {
		return fmt.Errorf("message expectation attempts is required")
	}
	*expectation = DeliveryMessageExpectation(decoded)
	return nil
}
