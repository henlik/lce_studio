package simulation

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"software-studio/internal/model"
)

func continuumFixture() model.Project {
	p := model.ExampleProject()
	p.SchemaVersion = 2
	p.Scenario = model.Scenario{RequestCount: 1, IntervalMs: 1, TimeoutMs: 1000, Seed: 42}
	p.Nodes[1].Config.ProcessingMs = 10
	p.Nodes[2].Config.ProcessingMs = 5
	for i := range p.Nodes {
		p.Nodes[i].Config.Capacity = 2
	}
	for i := range p.Edges {
		p.Edges[i].LatencyMs = 0
	}
	p.Infrastructure = &model.Infrastructure{
		MessageBytes: 1000,
		Sites: []model.Site{
			{ID: "site-local", Name: "Local", Latitude: 0, Longitude: 0},
			{ID: "site-remote", Name: "Remote", Latitude: 50, Longitude: 4},
		},
		Hosts: []model.Host{
			{ID: "host-client", Name: "Client device", SiteID: "site-local", Tier: "device", CPUCores: 1, MemoryMB: 512, IdleWatts: 2, ActiveWatts: 6, Outages: []model.Outage{}},
			{ID: "host-edge", Name: "Edge", SiteID: "site-local", Tier: "edge", CPUCores: 2, MemoryMB: 1024, IdleWatts: 10, ActiveWatts: 30, Outages: []model.Outage{}},
			{ID: "host-cloud", Name: "Cloud", SiteID: "site-remote", Tier: "cloud", CPUCores: 4, MemoryMB: 4096, IdleWatts: 20, ActiveWatts: 60, Outages: []model.Outage{}},
		},
		Links: []model.NetworkLink{
			{ID: "link-local", SourceHostID: "host-client", TargetHostID: "host-edge", LatencyMs: 5, BandwidthKbps: 8000, Outages: []model.Outage{}},
			{ID: "link-wan", SourceHostID: "host-client", TargetHostID: "host-cloud", LatencyMs: 50, BandwidthKbps: 8000, Outages: []model.Outage{}},
			{ID: "link-backhaul", SourceHostID: "host-edge", TargetHostID: "host-cloud", LatencyMs: 20, BandwidthKbps: 8000, Outages: []model.Outage{}},
		},
		Placements: []model.Placement{{NodeID: "client", HostID: "host-client"}, {NodeID: "service", HostID: "host-edge"}, {NodeID: "database", HostID: "host-edge"}},
	}
	return p
}

func assertInfrastructureConservation(t *testing.T, p model.Project, r model.Run) {
	t.Helper()
	if r.Infrastructure == nil || len(r.Infrastructure.Hosts) != len(p.Infrastructure.Hosts) || len(r.Infrastructure.Links) != len(p.Infrastructure.Links) {
		t.Fatal("missing or incomplete infrastructure metrics")
	}
	if r.Metrics.Completed+r.Metrics.Failed != r.Metrics.Requests {
		t.Fatal("request conservation failed")
	}
	var total float64
	for i, m := range r.Infrastructure.Hosts {
		h := p.Infrastructure.Hosts[i]
		if m.HostID != h.ID || m.UptimeMs < 0 || m.UptimeMs > r.Metrics.DurationMs || m.BusyCoreMs < 0 || m.BusyCoreMs > m.UptimeMs*int64(h.CPUCores) {
			t.Fatalf("invalid host resource accounting: %+v", m)
		}
		want := (h.IdleWatts*float64(m.UptimeMs) + (h.ActiveWatts-h.IdleWatts)*float64(m.BusyCoreMs)/float64(h.CPUCores)) / 3600000
		if math.Abs(m.EnergyWh-want) > 1e-10 {
			t.Fatalf("energy not conserved: got %g want %g", m.EnergyWh, want)
		}
		total += m.EnergyWh
	}
	if math.Abs(total-r.Infrastructure.EnergyWh) > 1e-10 {
		t.Fatal("host energy does not sum to total")
	}
	for i, m := range r.Infrastructure.Links {
		if m.LinkID != p.Infrastructure.Links[i].ID || m.Transfers < 0 || m.DroppedTransfers < 0 || m.BytesTransferred < 0 || int64(m.Transfers-m.DroppedTransfers)*p.Infrastructure.MessageBytes != m.BytesTransferred {
			t.Fatalf("transfer conservation failed: %+v", m)
		}
	}
}

func TestContinuumPlacementAndAnalyticalEnergy(t *testing.T) {
	p := continuumFixture()
	edge := runOK(t, p)
	// 1ms serialization + 5ms propagation each way, 10+5ms CPU.
	if edge.Metrics.P95LatencyMs != 27 || edge.Metrics.DurationMs != 1000 || edge.Metrics.Completed != 1 {
		t.Fatalf("edge roundtrip/horizon: %+v", edge.Metrics)
	}
	assertInfrastructureConservation(t, p, edge)
	// The idle unused cloud contributes 20W*1000ms. Edge has 15 core-ms
	// on a two-core host: 10W*1000ms + (30-10)W*15ms/2.
	if math.Abs(edge.Infrastructure.EnergyWh-32150.0/3600000) > 1e-12 {
		t.Fatalf("analytical fleet energy: %g", edge.Infrastructure.EnergyWh)
	}
	if m := edge.Infrastructure.Links[0]; m.Transfers != 2 || m.BytesTransferred != 2000 || m.DroppedTransfers != 0 {
		t.Fatalf("physical request/response accounting: %+v", m)
	}
	p.Infrastructure.Placements[1].HostID = "host-cloud"
	p.Infrastructure.Placements[2].HostID = "host-cloud"
	cloud := runOK(t, p)
	if cloud.Metrics.P95LatencyMs != 117 || cloud.ModelHash == edge.ModelHash {
		t.Fatalf("cloud placement must change routing and immutable snapshot: %+v", cloud.Metrics)
	}
	assertInfrastructureConservation(t, p, cloud)
}

func TestContinuumSharedBandwidthAndSerializationRounding(t *testing.T) {
	p := continuumFixture()
	p.Nodes[1].Config.ProcessingMs, p.Nodes[2].Config.ProcessingMs = 0, 0
	p.Scenario.RequestCount, p.Scenario.TimeoutMs = 2, 10000
	p.Infrastructure.Links[0].LatencyMs = 0
	p.Infrastructure.Links[0].BandwidthKbps = 8
	r := runOK(t, p)
	// Four 1000-byte packets share one 8kbit/s channel: one second each.
	// First response finishes at 3000ms; second at 4000ms after starting at 1ms.
	if r.Metrics.P95LatencyMs != 3999 || r.Metrics.AverageLatencyMs != 3499.5 {
		t.Fatalf("shared half-duplex FIFO: %+v", r.Metrics)
	}
	assertInfrastructureConservation(t, p, r)
	if m := r.Infrastructure.Links[0]; m.Transfers != 4 || m.BytesTransferred != 4000 {
		t.Fatalf("shared bandwidth counters: %+v", m)
	}
	p.Scenario.RequestCount = 1
	p.Infrastructure.MessageBytes = 1
	p.Infrastructure.Links[0].BandwidthKbps = 3
	r = runOK(t, p)
	if r.Metrics.P95LatencyMs != 6 { // ceil(8/3)=3ms per direction
		t.Fatalf("serialization must round up in integer ticks: %+v", r.Metrics)
	}
}

func TestContinuumCPUIsSharedAcrossApplicationNodes(t *testing.T) {
	p := continuumFixture()
	for i := range p.Infrastructure.Placements {
		p.Infrastructure.Placements[i].HostID = "host-edge"
	}
	p.Nodes[1].Config.ProcessingMs, p.Nodes[2].Config.ProcessingMs = 10, 10
	p.Scenario.RequestCount = 2
	p.Infrastructure.Hosts[1].CPUCores = 1
	one := runOK(t, p)
	// Service 1, service 2, DB 1, DB 2 occupy the shared core for 10ms each.
	if one.Metrics.P95LatencyMs != 39 || one.Metrics.AverageLatencyMs != 34.5 || one.Infrastructure.Hosts[1].BusyCoreMs != 40 {
		t.Fatalf("single shared core: %+v %+v", one.Metrics, one.Infrastructure.Hosts[1])
	}
	assertInfrastructureConservation(t, p, one)
	p.Infrastructure.Hosts[1].CPUCores = 2
	two := runOK(t, p)
	if two.Metrics.P95LatencyMs != 20 || two.Infrastructure.Hosts[1].BusyCoreMs != 40 {
		t.Fatalf("two shared cores: %+v %+v", two.Metrics, two.Infrastructure.Hosts[1])
	}
	assertInfrastructureConservation(t, p, two)
}

func TestContinuumPowerInterruptionAndRecovery(t *testing.T) {
	p := continuumFixture()
	p.Scenario.TimeoutMs, p.Scenario.MaxRetries = 60, 1
	p.Infrastructure.Hosts[1].Outages = []model.Outage{{StartMs: 10, EndMs: 50}}
	r := runOK(t, p)
	if r.Metrics.Completed != 1 || r.Metrics.P95LatencyMs != 87 || r.Metrics.Writes != 1 || r.Metrics.Attempts != 2 || r.Infrastructure.Hosts[1].InterruptedJobs != 1 {
		t.Fatalf("interrupted job must require bounded retry: %+v %+v", r.Metrics, r.Infrastructure.Hosts[1])
	}
	assertInfrastructureConservation(t, p, r)
	for _, v := range r.Events {
		if v.Kind == "host_power_lost" && (v.HostID != "host-edge" || v.NodeID != "" || v.RequestID != "") {
			t.Fatalf("environment event invents application IDs: %+v", v)
		}
	}
}

func TestContinuumOutageStartWinsProcessingCompletionTie(t *testing.T) {
	p := continuumFixture()
	p.Scenario.TimeoutMs = 100
	// DB would commit at 21ms, but environmental transitions precede work.
	p.Infrastructure.Hosts[1].Outages = []model.Outage{{StartMs: 21, EndMs: 50}}
	r := runOK(t, p)
	if r.Metrics.Writes != 0 || r.Metrics.Completed != 0 || r.Infrastructure.Hosts[1].InterruptedJobs != 1 {
		t.Fatalf("power loss must win the exact commit-time tie: %+v", r.Metrics)
	}
	assertInfrastructureConservation(t, p, r)
}

func TestContinuumCompletedWriteSurvivesResponseLossAndPowerCycle(t *testing.T) {
	for _, outage := range []string{"host", "link"} {
		t.Run(outage, func(t *testing.T) {
			p := continuumFixture()
			p.Scenario.TimeoutMs, p.Scenario.MaxRetries = 100, 1
			window := []model.Outage{{StartMs: 27, EndMs: 40}}
			if outage == "host" {
				p.Infrastructure.Hosts[1].Outages = window
			} else {
				p.Infrastructure.Links[0].Outages = window
			}
			r := runOK(t, p)
			// Write committed at 21ms; outage wins response-arrival tie at 27ms.
			if r.Metrics.Completed != 1 || r.Metrics.P95LatencyMs != 127 || r.Metrics.Writes != 1 || r.Metrics.Deduplicated != 1 || r.Metrics.DuplicateWrites != 0 {
				t.Fatalf("durable deduplication must survive interrupted return path: %+v", r.Metrics)
			}
			if m := r.Infrastructure.Links[0]; m.Transfers != 4 || m.BytesTransferred != 3000 || m.DroppedTransfers != 1 {
				t.Fatalf("response loss accounting: %+v", m)
			}
			assertInfrastructureConservation(t, p, r)
		})
	}
}

func TestContinuumPowerDropsRunningAndQueuedJobs(t *testing.T) {
	p := continuumFixture()
	p.Scenario.RequestCount, p.Scenario.TimeoutMs = 3, 300
	p.Nodes[1].Config.ProcessingMs = 100
	p.Infrastructure.Hosts[1].CPUCores = 1
	p.Infrastructure.Hosts[1].Outages = []model.Outage{{StartMs: 10, EndMs: 200}}
	r := runOK(t, p)
	if r.Metrics.Completed != 0 || r.Metrics.Writes != 0 || r.Infrastructure.Hosts[1].InterruptedJobs != 3 || r.Infrastructure.Hosts[1].BusyCoreMs != 4 {
		t.Fatalf("power loss must discard running and queued work: %+v %+v", r.Metrics, r.Infrastructure.Hosts[1])
	}
	assertInfrastructureConservation(t, p, r)
}

func TestContinuumOutageDropsLinkQueueAndInFlightMessages(t *testing.T) {
	p := continuumFixture()
	p.Scenario.RequestCount, p.Scenario.TimeoutMs = 3, 1000
	p.Infrastructure.Links[0].BandwidthKbps = 8
	p.Infrastructure.Links[0].Outages = []model.Outage{{StartMs: 10, EndMs: 100}}
	r := runOK(t, p)
	if r.Metrics.Completed != 0 || r.Metrics.Writes != 0 {
		t.Fatalf("disconnected transmissions unexpectedly completed: %+v", r.Metrics)
	}
	if m := r.Infrastructure.Links[0]; m.Transfers != 3 || m.BytesTransferred != 0 || m.DroppedTransfers != 3 {
		t.Fatalf("queued and transmitting packets must be dropped once: %+v", m)
	}
	assertInfrastructureConservation(t, p, r)
}

func TestContinuumTouchingOutagesAndEnvironmentalHorizon(t *testing.T) {
	p := continuumFixture()
	p.Scenario.TimeoutMs = 100
	// Unsorted touching windows mean continuous outage, not a brief power-up.
	p.Infrastructure.Hosts[2].Outages = []model.Outage{{StartMs: 500, EndMs: 600}, {StartMs: 400, EndMs: 500}}
	r := runOK(t, p)
	if r.Metrics.Completed != 1 || r.Metrics.DurationMs != 600 || r.Infrastructure.Hosts[2].UptimeMs != 400 {
		t.Fatalf("environment-only horizon/half-open intervals: %+v %+v", r.Metrics, r.Infrastructure.Hosts[2])
	}
	transitions := 0
	for _, v := range r.Events {
		if v.HostID == "host-cloud" && (v.Kind == "host_power_lost" || v.Kind == "host_power_restored") {
			transitions++
		}
	}
	if transitions != 2 {
		t.Fatalf("touching windows produced spurious transitions: %d", transitions)
	}
	assertInfrastructureConservation(t, p, r)
}

func TestContinuumUnavailableClientAndOutageEndBoundary(t *testing.T) {
	p := continuumFixture()
	p.Scenario.TimeoutMs, p.Scenario.MaxRetries = 50, 1
	p.Infrastructure.Hosts[0].Outages = []model.Outage{{StartMs: 0, EndMs: 50}}
	r := runOK(t, p)
	if r.Metrics.Completed != 1 || r.Metrics.Attempts != 2 || r.Metrics.P95LatencyMs != 77 {
		t.Fatalf("retry at power restoration should succeed: %+v", r.Metrics)
	}
	assertInfrastructureConservation(t, p, r)
}

func TestContinuumCancellationLimitsAndDeterminism(t *testing.T) {
	p := continuumFixture()
	p.Scenario.JitterMs = 20
	p.Scenario.RequestCount = 4
	p.Infrastructure.Hosts[1].Outages = []model.Outage{{StartMs: 20, EndMs: 30}}
	p.Scenario.MaxRetries = 1
	before, _ := json.Marshal(p)
	a, b := runOK(t, p), runOK(t, p)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same v2 model/seed did not reproduce trace and energy")
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("v2 simulator mutated input")
	}
	assertInfrastructureConservation(t, p, a)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled v2 run returned %v", err)
	}
	p = continuumFixture()
	p.Infrastructure.MessageBytes = 10485760
	p.Infrastructure.Links[0].BandwidthKbps = 1
	if r, err := Run(context.Background(), p); !errors.Is(err, ErrLimit) || len(r.Verdicts) != 0 || r.Infrastructure != nil {
		t.Fatalf("overlong transmission must return error without partial success: %v", err)
	}
}

func FuzzContinuumConservation(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6})
	f.Add([]byte{255, 0, 255, 1, 0, 50})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) < 6 {
			return
		}
		p := continuumFixture()
		p.Scenario.RequestCount = int(b[0]%5) + 1
		p.Scenario.TimeoutMs = int64(b[1]) + 1
		p.Scenario.MaxRetries = int(b[2] % 3)
		p.Scenario.JitterMs = int64(b[3] % 10)
		p.Infrastructure.Hosts[1].CPUCores = int(b[4]%3) + 1
		start := int64(b[5])
		p.Infrastructure.Hosts[1].Outages = []model.Outage{{StartMs: start, EndMs: start + 20}}
		p.Infrastructure.Links[0].Outages = []model.Outage{{StartMs: start + 10, EndMs: start + 40}}
		a, c := runOK(t, p), runOK(t, p)
		if !reflect.DeepEqual(a, c) {
			t.Fatal("non-deterministic continuum simulation")
		}
		assertInfrastructureConservation(t, p, a)
	})
}
