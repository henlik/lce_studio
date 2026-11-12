package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"software-studio/internal/model"
)

func decodePackage(data []byte, pkg *model.ComponentPackage) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(pkg); err != nil {
		return fmt.Errorf("decode package: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("package must contain exactly one JSON value")
	}
	return nil
}
