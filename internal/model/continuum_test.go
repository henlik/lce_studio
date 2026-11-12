package model

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func issueAt(issues []ValidationIssue, path string) bool {
	for _, issue := range issues {
		if issue.Path == path {
			return true
		}
	}
	return false
}

func TestContinuumExampleAndPlacementAdmission(t *testing.T) {
	p := ContinuumExampleProject()
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("edge example rejected: %+v", issues)
	}
	for i := range p.Infrastructure.Placements {
		if p.Infrastructure.Placements[i].NodeID != "client" {
			p.Infrastructure.Placements[i].HostID = "cloud-host"
		}
	}
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("cloud example rejected: %+v", issues)
	}
	p.Infrastructure.Placements = p.Infrastructure.Placements[:1]
	if issues := ValidateDraft(p); len(issues) != 0 {
		t.Fatalf("unfinished placements must remain savable: %+v", issues)
	}
	if issues := Validate(p); !issueAt(issues, "nodes[1]") || !issueAt(issues, "nodes[2]") {
		t.Fatalf("missing placements accepted: %+v", issues)
	}
	p = ContinuumExampleProject()
	p.Infrastructure.Hosts[1].MemoryMB = 383 // service128 + database256 =384
	if issues := ValidateDraft(p); len(issues) != 0 {
		t.Fatalf("overcommitted placement must remain an editable draft: %+v", issues)
	}
	if issues := Validate(p); !issueAt(issues, "infrastructure.hosts[1].memoryMb") {
		t.Fatalf("overcommitted memory accepted: %+v", issues)
	}
	p.Infrastructure.Hosts[1].MemoryMB = 384
	p.Infrastructure.Hosts[1].CPUCores = 1
	p.Nodes[1].Config.Capacity = 100
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("shared-core contention is executable, not a reservation error: %+v", issues)
	}
}

func TestContinuumDefaultMemoryAndPhysicalConnections(t *testing.T) {
	p := ContinuumExampleProject()
	p.Nodes[1].Config.MemoryMB = 0
	p.Nodes[2].Config.MemoryMB = 0
	p.Infrastructure.Hosts[1].MemoryMB = 128
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("default64MiB reservations rejected: %+v", issues)
	}
	p.Infrastructure.Hosts[1].MemoryMB = 127
	if issues := Validate(p); !issueAt(issues, "infrastructure.hosts[1].memoryMb") {
		t.Fatalf("omitted memory must not bypass admission: %+v", issues)
	}
	p = ContinuumExampleProject()
	p.Infrastructure.Links = p.Infrastructure.Links[1:] // an indirect path remains
	if issues := ValidateDraft(p); len(issues) != 0 {
		t.Fatalf("missing direct path should be an editable draft: %+v", issues)
	}
	if issues := Validate(p); !issueAt(issues, "edges[0]") {
		t.Fatalf("unsupported multihop route accepted: %+v", issues)
	}
	for i := range p.Infrastructure.Placements {
		p.Infrastructure.Placements[i].HostID = "edge-host"
	}
	p.Infrastructure.Links = []NetworkLink{}
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("same-host communication must not require a physical link: %+v", issues)
	}
}

func TestContinuumRejectsMalformedInfrastructure(t *testing.T) {
	tests := []struct {
		name, path string
		mutate     func(*Project)
	}{
		{"legacy infrastructure", "infrastructure", func(p *Project) { p.SchemaVersion = 1 }},
		{"missing infrastructure", "infrastructure", func(p *Project) { p.Infrastructure = nil }},
		{"site node collision", "infrastructure.sites[0].id", func(p *Project) { p.Infrastructure.Sites[0].ID = "client" }},
		{"host site collision", "infrastructure.hosts[0].id", func(p *Project) { p.Infrastructure.Hosts[0].ID = "field-site" }},
		{"duplicate link id", "infrastructure.links[1].id", func(p *Project) { p.Infrastructure.Links[1].ID = "local-link" }},
		{"duplicate site id", "infrastructure.sites[1].id", func(p *Project) { p.Infrastructure.Sites[1].ID = "field-site" }},
		{"unsafe identifier", "infrastructure.hosts[0].id", func(p *Project) { p.Infrastructure.Hosts[0].ID = "../host" }},
		{"empty site name", "infrastructure.sites[0].name", func(p *Project) { p.Infrastructure.Sites[0].Name = " " }},
		{"host name control", "infrastructure.hosts[0].name", func(p *Project) { p.Infrastructure.Hosts[0].Name = "host\x01" }},
		{"latitude NaN", "infrastructure.sites[0].latitude", func(p *Project) { p.Infrastructure.Sites[0].Latitude = math.NaN() }},
		{"longitude range", "infrastructure.sites[0].longitude", func(p *Project) { p.Infrastructure.Sites[0].Longitude = 181 }},
		{"host site reference", "infrastructure.hosts[0].siteId", func(p *Project) { p.Infrastructure.Hosts[0].SiteID = "missing" }},
		{"tier", "infrastructure.hosts[0].tier", func(p *Project) { p.Infrastructure.Hosts[0].Tier = "quantum" }},
		{"cpu zero", "infrastructure.hosts[0].cpuCores", func(p *Project) { p.Infrastructure.Hosts[0].CPUCores = 0 }},
		{"cpu bound", "infrastructure.hosts[0].cpuCores", func(p *Project) { p.Infrastructure.Hosts[0].CPUCores = 129 }},
		{"host memory", "infrastructure.hosts[0].memoryMb", func(p *Project) { p.Infrastructure.Hosts[0].MemoryMB = 0 }},
		{"node memory", "nodes[0].config.memoryMb", func(p *Project) { p.Nodes[0].Config.MemoryMB = -1 }},
		{"idle infinity", "infrastructure.hosts[0].idleWatts", func(p *Project) { p.Infrastructure.Hosts[0].IdleWatts = math.Inf(1) }},
		{"active NaN", "infrastructure.hosts[0].activeWatts", func(p *Project) { p.Infrastructure.Hosts[0].ActiveWatts = math.NaN() }},
		{"active below idle", "infrastructure.hosts[0].activeWatts", func(p *Project) { p.Infrastructure.Hosts[0].ActiveWatts = 1 }},
		{"link source reference", "infrastructure.links[0].sourceHostId", func(p *Project) { p.Infrastructure.Links[0].SourceHostID = "missing" }},
		{"link self", "infrastructure.links[0].targetHostId", func(p *Project) { p.Infrastructure.Links[0].TargetHostID = "device-host" }},
		{"duplicate unordered pair", "infrastructure.links[1]", func(p *Project) {
			p.Infrastructure.Links[1].SourceHostID = "edge-host"
			p.Infrastructure.Links[1].TargetHostID = "device-host"
		}},
		{"link latency", "infrastructure.links[0].latencyMs", func(p *Project) { p.Infrastructure.Links[0].LatencyMs = -1 }},
		{"link bandwidth", "infrastructure.links[0].bandwidthKbps", func(p *Project) { p.Infrastructure.Links[0].BandwidthKbps = 0 }},
		{"message size", "infrastructure.messageBytes", func(p *Project) { p.Infrastructure.MessageBytes = 10485761 }},
		{"placement node reference", "infrastructure.placements[0].nodeId", func(p *Project) { p.Infrastructure.Placements[0].NodeID = "missing" }},
		{"placement host reference", "infrastructure.placements[0].hostId", func(p *Project) { p.Infrastructure.Placements[0].HostID = "missing" }},
		{"duplicate placement", "infrastructure.placements[1].nodeId", func(p *Project) { p.Infrastructure.Placements[1].NodeID = "client" }},
		{"nil sites", "infrastructure.sites", func(p *Project) { p.Infrastructure.Sites = nil }},
		{"nil hosts", "infrastructure.hosts", func(p *Project) { p.Infrastructure.Hosts = nil }},
		{"nil links", "infrastructure.links", func(p *Project) { p.Infrastructure.Links = nil }},
		{"nil placements", "infrastructure.placements", func(p *Project) { p.Infrastructure.Placements = nil }},
		{"nil host outages", "infrastructure.hosts[0].outages", func(p *Project) { p.Infrastructure.Hosts[0].Outages = nil }},
		{"nil link outages", "infrastructure.links[0].outages", func(p *Project) { p.Infrastructure.Links[0].Outages = nil }},
		{"negative outage start", "infrastructure.hosts[0].outages[0].startMs", func(p *Project) { p.Infrastructure.Hosts[0].Outages = []Outage{{-1, 1}} }},
		{"empty outage interval", "infrastructure.hosts[0].outages[0].endMs", func(p *Project) { p.Infrastructure.Hosts[0].Outages = []Outage{{10, 10}} }},
		{"outage horizon", "infrastructure.links[0].outages[0].endMs", func(p *Project) { p.Infrastructure.Links[0].Outages = []Outage{{0, 3600001}} }},
		{"overlapping outage windows", "infrastructure.hosts[0].outages", func(p *Project) { p.Infrastructure.Hosts[0].Outages = []Outage{{30, 50}, {10, 40}} }},
		{"too many outage windows", "infrastructure.hosts[0].outages", func(p *Project) { p.Infrastructure.Hosts[0].Outages = make([]Outage, 21) }},
		{"too many hosts", "infrastructure.hosts", func(p *Project) { p.Infrastructure.Hosts = make([]Host, 31) }},
		{"too many sites", "infrastructure.sites", func(p *Project) { p.Infrastructure.Sites = make([]Site, 31) }},
		{"too many links", "infrastructure.links", func(p *Project) { p.Infrastructure.Links = make([]NetworkLink, 61) }},
		{"too many placements", "infrastructure.placements", func(p *Project) { p.Infrastructure.Placements = make([]Placement, 31) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := ContinuumExampleProject()
			test.mutate(&p)
			if issues := ValidateDraft(p); !issueAt(issues, test.path) {
				t.Fatalf("expected issue at %s; got %+v", test.path, issues)
			}
		})
	}
}

func TestOutageValidationPreservesOrderAndAcceptsTouching(t *testing.T) {
	p := ContinuumExampleProject()
	windows := []Outage{{100, 200}, {0, 100}, {200, 300}}
	p.Infrastructure.Hosts[1].Outages = append([]Outage(nil), windows...)
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("unsorted touching intervals rejected: %+v", issues)
	}
	if !reflect.DeepEqual(p.Infrastructure.Hosts[1].Outages, windows) {
		t.Fatal("validation reordered the original outage windows")
	}
}

func TestLegacyJSONAndReferenceFixtures(t *testing.T) {
	legacy, err := os.ReadFile("../../examples/retry-pipeline.json")
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, legacy); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(ExampleProject())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, compact.Bytes()) {
		t.Fatal("v0.2 additions changed legacy serialized data and would change snapshot hashes")
	}
	p := ExampleProject()
	p.Nodes[0].Config.MemoryMB = 1
	if issues := Validate(p); !issueAt(issues, "nodes[0].config.memoryMb") {
		t.Fatalf("v1 must not silently ignore memory declarations: %+v", issues)
	}
	raw, err := os.ReadFile("../../examples/continuum-placement.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var imported Project
	if err := decoder.Decode(&imported); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(imported, ContinuumExampleProject()) {
		t.Fatal("JSON fixture differs from the API example")
	}
	if issues := Validate(imported); len(issues) != 0 {
		t.Fatalf("imported fixture rejected: %+v", issues)
	}
}
