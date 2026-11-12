package simulation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"software-studio/internal/model"
)

// These snapshots were captured from v0.2 before adding schema-3 routing.
// Only the engine version changes; hashes, traces, timing and metrics must not.
func TestLegacyExecutionSnapshots(t *testing.T) {
	v1 := model.ExampleProject()
	v1.Scenario.JitterMs = 7
	v2 := continuumFixture()
	v2.Scenario.RequestCount = 4
	v2.Scenario.IntervalMs = 3
	v2.Scenario.TimeoutMs = 100
	v2.Scenario.MaxRetries = 2
	v2.Scenario.JitterMs = 4
	v2.Scenario.DropFirstResponse = true
	v2.Infrastructure.Hosts[1].Outages = []model.Outage{{StartMs: 10, EndMs: 50}}
	v2.Infrastructure.Links[0].Outages = []model.Outage{{StartMs: 127, EndMs: 140}}
	for _, tc := range []struct {
		name    string
		project model.Project
		want    string
	}{{"schema1", v1, "1ecee761d157c5cdfc84fe143d68ef234b8a94bcdffc734a4558be0452eed92f"}, {"schema2", v2, "0c4af9aab39af71741922a3eaca06dafda832476f0ca592d3f6a71e664b29f01"}} {
		t.Run(tc.name, func(t *testing.T) {
			r := runOK(t, tc.project)
			r.EngineVersion = ""
			encoded, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(encoded)
			got := hex.EncodeToString(hash[:])
			if got != tc.want {
				t.Fatalf("legacy run snapshot changed: got %s; want %s", got, tc.want)
			}
		})
	}
}
