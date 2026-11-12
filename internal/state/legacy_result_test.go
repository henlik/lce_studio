package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"software-studio/internal/model"
)

// These cover the entire canonical v0.6 execution result, including every
// decision trace, snapshot, historical receipt and independently checked verdict.
func TestCanonicalLegacyExamples(t *testing.T) {
	want := []string{"710aa43a993e1a846e109e9e56456a06c1b4bbc486141c38ae7c43406fac97cb", "6d8c9720f9b2826d8fed876cd44b1cd3868b1c1573c4ab93a76904359c7c335a"}
	for i, example := range model.ExampleStateScenarios() {
		result, err := Run(context.Background(), example.Machine, example.Scenario)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(encoded)
		got := hex.EncodeToString(hash[:])
		if got != want[i] {
			t.Fatalf("legacy example%d result changed: %s, want %s", i, got, want[i])
		}
	}
}
