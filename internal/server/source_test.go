package server

import (
	"encoding/json"
	sourceexamples "software-studio/examples/source"
	"software-studio/internal/source"
	"testing"
)

func TestSourceAPIWithoutStorage(t *testing.T) {
	s := New(nil, "")
	w := req(t, s, "GET", "/api/systems/source/example", nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, input := range []string{sourceexamples.M1, "entity broken"} {
		w = req(t, s, "POST", "/api/systems/source/compile", map[string]any{"source": input})
		var result source.Result
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Valid != (input == sourceexamples.M1) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, body := range []map[string]any{{"source": sourceexamples.M1, "execute": true}, {"source": 1}} {
		if w = req(t, s, "POST", "/api/systems/source/compile", body); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w = req(t, s, "GET", "/api/systems/source/compile", nil); w.Code != 405 {
		t.Fatal(w.Code)
	}
}
