package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
	var out bytes.Buffer
	err = check(args, bytes.NewReader(raw), &out)
	return out.String(), err
}
func TestPortableDataContractAndFixtures(t *testing.T) {
	examples := model.ExampleDataFixtures()
	if len(examples) != 2 {
		t.Fatal("expected two reference domains")
	}
	for _, ex := range examples {
		raw, err := invoke(t, "", ex.Contract.Contract)
		if err != nil {
			t.Fatal(err)
		}
		var lock model.DataContractLock
		if json.Unmarshal([]byte(raw), &lock) != nil || lock.Digest != ex.Contract.Digest {
			t.Fatal("contract lock changed")
		}
		if _, err := invoke(t, "--lock", lock); err != nil {
			t.Fatal(err)
		}
		raw, err = invoke(t, "--fixture", ex)
		if err != nil {
			t.Fatal(err)
		}
		var result model.DataCheckResult
		if json.Unmarshal([]byte(raw), &result) != nil || !result.Valid || result.RowCount == 0 {
			t.Fatal(raw)
		}
	}
}
func TestInvalidFixturePrintsVerdictAndFails(t *testing.T) {
	ex := model.ExampleDataFixtures()[0]
	ex.Fixture.Rows["Unexpected"] = []map[string]model.TypedValue{{}}
	raw, err := invoke(t, "--fixture", ex)
	var result model.DataCheckResult
	if err == nil || json.Unmarshal([]byte(raw), &result) != nil || result.Valid || len(result.Issues) == 0 {
		t.Fatalf("missing invalid verdict: %s %v", raw, err)
	}
}
func TestComparisonNeverMigratesOrTreatsIdentityAsCompatibility(t *testing.T) {
	old := model.ExampleDataFixtures()[0].Contract
	next := old
	next.Contract.Version = "1.0.1"
	next.Contract.Name += " renamed"
	next.Digest, _ = model.DataContractDigest(next.Contract)
	raw, err := invoke(t, "--compare", map[string]any{"old": old, "new": next})
	var result model.DataCompatibility
	if err != nil || json.Unmarshal([]byte(raw), &result) != nil || !result.Compatible || result.Direction != "old-to-new" {
		t.Fatalf("metadata comparison: %s %v", raw, err)
	}
	next.Contract.ID = "other.identity"
	next.Digest, _ = model.DataContractDigest(next.Contract)
	if _, err := invoke(t, "--compare", map[string]any{"old": old, "new": next}); err == nil {
		t.Fatal("different identity accepted")
	}
}
func TestStrictInputAndLocks(t *testing.T) {
	for _, raw := range []string{"null", "{} {}", `{"unknown":true}`, strings.Repeat(" ", 2*1024*1024+1)} {
		var out bytes.Buffer
		if err := check([]string{"-"}, strings.NewReader(raw), &out); err == nil || out.Len() != 0 {
			t.Fatal("invalid input accepted")
		}
	}
	lock := model.ExampleDataFixtures()[0].Contract
	lock.Digest = strings.Repeat("0", 64)
	if raw, err := invoke(t, "--lock", lock); err == nil || raw != "" {
		t.Fatal("tampered lock accepted")
	}
}
