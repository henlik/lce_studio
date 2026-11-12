package simulation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"software-studio/internal/model"
)

func workflowFixture() model.Project {
	return model.Project{
		SchemaVersion: 3, ID: "branch-check", Name: "Independent exclusive paths",
		// Deliberately not topological: neither client nor stores occupy the old
		// pipeline positions, and the two branches have different latencies.
		Nodes: []model.Node{
			{ID: "store-b", Type: "database", Name: "Store B", Config: model.NodeConfig{ProcessingMs: 5, Capacity: 2, Deduplicate: true}},
			{ID: "a", Type: "service", Name: "Worker A", Config: model.NodeConfig{ProcessingMs: 10, Capacity: 2}},
			{ID: "client", Type: "client", Name: "Client", Config: model.NodeConfig{Capacity: 1}},
			{ID: "router", Type: "router", Name: "Exclusive router", Config: model.NodeConfig{ProcessingMs: 2, Capacity: 2}},
			{ID: "store-a", Type: "database", Name: "Store A", Config: model.NodeConfig{ProcessingMs: 3, Capacity: 2, Deduplicate: true}},
			{ID: "b", Type: "service", Name: "Worker B", Config: model.NodeConfig{ProcessingMs: 20, Capacity: 2}},
		},
		Edges: []model.Edge{
			{ID: "route-b", Source: "router", Target: "b", LatencyMs: 4, RouteWeight: 3},
			{ID: "a-store", Source: "a", Target: "store-a", LatencyMs: 3},
			{ID: "client-router", Source: "client", Target: "router", LatencyMs: 1},
			{ID: "b-store", Source: "b", Target: "store-b", LatencyMs: 6},
			{ID: "route-a", Source: "router", Target: "a", LatencyMs: 2},
		},
		Requirements: []model.Requirement{},
		Scenario:     model.Scenario{RequestCount: 12, IntervalMs: 100, TimeoutMs: 1000, Seed: 42},
	}
}

func workflowSelections(t *testing.T, r model.Run) map[string]string {
	t.Helper()
	selected := make(map[string]string)
	for _, event := range r.Events {
		if event.Kind != "route_selected" {
			continue
		}
		if event.NodeID == "" || event.RequestID == "" || event.EdgeID == "" || event.Attempt < 1 {
			t.Fatalf("incomplete route evidence: %+v", event)
		}
		key := event.RequestID + "/" + event.NodeID
		if prior, exists := selected[key]; exists && prior != event.EdgeID {
			t.Fatalf("retry changed route for %s: %s to %s", key, prior, event.EdgeID)
		}
		selected[key] = event.EdgeID
	}
	return selected
}

func eventCounts(r model.Run, kind string) map[string]int {
	counts := make(map[string]int)
	for _, event := range r.Events {
		if event.Kind == kind {
			counts[event.NodeID]++
		}
	}
	return counts
}

func TestWorkflowWeightedRoutesAndAnalyticalTiming(t *testing.T) {
	p := workflowFixture()
	r := runOK(t, p)
	// SHA-256 reference vector, calculated outside the Go engine: modulo-four
	// buckets 1,1,2,0,3,0,2,3,0,1,3,0; weight A=1, B=3.
	const choices = "BBBABABBABBA"
	selected := workflowSelections(t, r)
	if len(selected) != 12 {
		t.Fatalf("expected one router choice per logical request: %v", selected)
	}
	for i, choice := range choices {
		want := "route-a"
		if choice == 'B' {
			want = "route-b"
		}
		key := fmt.Sprintf("request-%03d/router", i+1)
		if selected[key] != want {
			t.Fatalf("%s got %s want %s", key, selected[key], want)
		}
	}
	// A: 2+10+3 CPU + 2*(1+2+3) link =27. B: 2+20+5 +2*(1+4+6)=49.
	if r.Metrics.Completed != 12 || r.Metrics.Writes != 12 || r.Metrics.DuplicateWrites != 0 || r.Metrics.Deduplicated != 0 || r.Metrics.P95LatencyMs != 49 || math.Abs(r.Metrics.AverageLatencyMs-500.0/12) > 1e-12 {
		t.Fatalf("exclusive weighted timing/counts: %+v", r.Metrics)
	}
	if got := eventCounts(r, "write"); !reflect.DeepEqual(got, map[string]int{"store-a": 4, "store-b": 8}) {
		t.Fatalf("branch writes: %v", got)
	}
	for _, event := range r.Events {
		if event.Kind == "completed" {
			var ordinal int
			if _, err := fmt.Sscanf(event.RequestID, "request-%d", &ordinal); err != nil {
				t.Fatal(err)
			}
			latency := int64(27)
			if choices[ordinal-1] == 'B' {
				latency = 49
			}
			if event.TimeMs != int64(ordinal-1)*100+latency {
				t.Fatalf("incorrect reverse route: %+v", event)
			}
		}
		if event.HostID != "" || event.LinkID != "" {
			t.Fatalf("fabricated physical identity: %+v", event)
		}
	}
	if r.Infrastructure != nil || r.Metrics.DurationMs != 2100 {
		t.Fatalf("application-only observation contract: %+v, %+v", r.Metrics, r.Infrastructure)
	}
	for i, j := 0, len(p.Edges)-1; i < j; i, j = i+1, j-1 {
		p.Edges[i], p.Edges[j] = p.Edges[j], p.Edges[i]
	}
	reordered := runOK(t, p)
	if !reflect.DeepEqual(r.Events, reordered.Events) || r.Metrics != reordered.Metrics {
		t.Fatal("edge array ordering changed routing or execution")
	}
	if r.ModelHash == reordered.ModelHash {
		t.Fatal("serialized model hash must still capture ordering")
	}
}

func TestWorkflowSeedBoundaryVectors(t *testing.T) {
	for _, tc := range []struct {
		seed int64
		want string
	}{
		{0, "ABBBBABBBABB"}, {2147483647, "AABAABBBBABA"},
	} {
		p := workflowFixture()
		p.Scenario.Seed = tc.seed
		selected := workflowSelections(t, runOK(t, p))
		for i, choice := range tc.want {
			want := "route-a"
			if choice == 'B' {
				want = "route-b"
			}
			if selected[fmt.Sprintf("request-%03d/router", i+1)] != want {
				t.Fatalf("seed boundary %d route vector mismatch", tc.seed)
			}
		}
	}
}

func TestWorkflowConvergenceAndReversePath(t *testing.T) {
	p := workflowFixture()
	nodes := []model.Node{}
	for _, n := range p.Nodes {
		if n.Type != "database" {
			nodes = append(nodes, n)
		}
	}
	p.Nodes = append(nodes, model.Node{ID: "shared-store", Type: "database", Name: "Shared store", Config: model.NodeConfig{ProcessingMs: 5, Capacity: 2, Deduplicate: true}})
	for i := range p.Edges {
		if p.Edges[i].Target == "store-a" || p.Edges[i].Target == "store-b" {
			p.Edges[i].Target = "shared-store"
		}
	}
	r := runOK(t, p)
	// A's 3ms store becomes 5ms: 29ms. B stays49ms. Convergence consumes
	// precisely one arriving branch per request, without waiting for a sibling.
	if r.Metrics.Completed != 12 || r.Metrics.Writes != 12 || r.Metrics.Deduplicated != 0 || r.Metrics.P95LatencyMs != 49 || math.Abs(r.Metrics.AverageLatencyMs-508.0/12) > 1e-12 {
		t.Fatalf("convergence must not fan out or wait for a join: %+v", r.Metrics)
	}
	forwards := eventCounts(r, "response_forwarded")
	if !reflect.DeepEqual(forwards, map[string]int{"router": 12, "a": 4, "b": 8}) {
		t.Fatalf("response crossed an unselected branch: %v", forwards)
	}
}

func TestWorkflowConvergesIntoAnotherRouter(t *testing.T) {
	p := workflowFixture()
	p.Nodes = append(p.Nodes, model.Node{ID: "merge-router", Type: "router", Name: "Second exclusive router", Config: model.NodeConfig{ProcessingMs: 4, Capacity: 2}})
	p.Edges[1].Target, p.Edges[3].Target = "merge-router", "merge-router"
	p.Edges = append(p.Edges,
		model.Edge{ID: "finish-a", Source: "merge-router", Target: "store-a", LatencyMs: 5},
		model.Edge{ID: "finish-b", Source: "merge-router", Target: "store-b", LatencyMs: 7},
	)
	r := runOK(t, p)
	selected := workflowSelections(t, r)
	// The independent second-router vector is ABAAAABAABAA. Combining CPU
	// and reverse-edge costs gives AA41, AB47, BA61, BB67 milliseconds.
	const first, second = "BBBABABBABBA", "ABAAAABAABAA"
	for i := range second {
		request := fmt.Sprintf("request-%03d", i+1)
		want := "finish-a"
		if second[i] == 'B' {
			want = "finish-b"
		}
		if selected[request+"/merge-router"] != want {
			t.Fatalf("second router choice changed for %s", request)
		}
		latency := int64(41)
		if first[i] == 'B' {
			latency += 20
		}
		if second[i] == 'B' {
			latency += 6
		}
		for _, event := range r.Events {
			if event.RequestID == request && event.Kind == "completed" && event.TimeMs != int64(i)*100+latency {
				t.Fatalf("two-router reverse timing: %+v", event)
			}
		}
	}
	if len(selected) != 24 || r.Metrics.Writes != 12 || r.Metrics.Completed != 12 || r.Metrics.P95LatencyMs != 67 || math.Abs(r.Metrics.AverageLatencyMs-670.0/12) > 1e-12 {
		t.Fatalf("converged routing must remain exclusive: %+v", r.Metrics)
	}
	assertWorkflowConservation(t, p, r)
	p.Scenario.IntervalMs, p.Scenario.TimeoutMs, p.Scenario.MaxRetries = 200, 100, 1
	p.Scenario.DropFirstResponse = true
	retry := runOK(t, p)
	if !reflect.DeepEqual(selected, workflowSelections(t, retry)) || retry.Metrics.Writes != 12 || retry.Metrics.Deduplicated != 12 {
		t.Fatalf("nested retry route instability: %+v", retry.Metrics)
	}
	assertWorkflowConservation(t, p, retry)
}

func TestWorkflowRetryChoicesAndStoreSpecificDeduplication(t *testing.T) {
	p := workflowFixture()
	p.Scenario.IntervalMs = 200
	p.Scenario.TimeoutMs = 100
	p.Scenario.MaxRetries = 1
	p.Scenario.DropFirstResponse = true
	r := runOK(t, p)
	if r.Metrics.Completed != 12 || r.Metrics.Attempts != 24 || r.Metrics.Writes != 12 || r.Metrics.DuplicateWrites != 0 || r.Metrics.Deduplicated != 12 || r.Metrics.P95LatencyMs != 149 {
		t.Fatalf("lost response must retry the same store: %+v", r.Metrics)
	}
	workflowSelections(t, r)
	if got := eventCounts(r, "route_selected")["router"]; got != 24 {
		t.Fatalf("expected retry route evidence, got %d selections", got)
	}
	want := map[string]int{"store-a": 4, "store-b": 8}
	if !reflect.DeepEqual(eventCounts(r, "write"), want) || !reflect.DeepEqual(eventCounts(r, "deduplicated"), want) {
		t.Fatal("store-specific dedup evidence missing")
	}
	for i := range p.Nodes {
		if p.Nodes[i].ID == "store-b" {
			p.Nodes[i].Config.Deduplicate = false
		}
	}
	r = runOK(t, p)
	workflowSelections(t, r)
	if r.Metrics.Writes != 20 || r.Metrics.DuplicateWrites != 8 || r.Metrics.Deduplicated != 4 || r.Metrics.Completed != 12 {
		t.Fatalf("only unprotected store B should duplicate: %+v", r.Metrics)
	}
	if !reflect.DeepEqual(eventCounts(r, "duplicate_write"), map[string]int{"store-b": 8}) {
		t.Fatal("duplicate writes escaped their database scope")
	}
}

func workflowCPUFixture() model.Project {
	p := workflowFixture()
	p.Scenario.RequestCount = 3
	p.Scenario.IntervalMs = 1
	for i := range p.Edges {
		p.Edges[i].LatencyMs, p.Edges[i].RouteWeight = 0, 0
	}
	for i := range p.Nodes {
		p.Nodes[i].Config.ProcessingMs = 0
		if p.Nodes[i].Type == "service" {
			p.Nodes[i].Config.ProcessingMs = 10
		}
	}
	return p
}

func colocateWorkflow(p *model.Project) {
	p.Infrastructure = continuumFixture().Infrastructure
	p.Infrastructure.Placements = nil
	for _, n := range p.Nodes {
		p.Infrastructure.Placements = append(p.Infrastructure.Placements, model.Placement{NodeID: n.ID, HostID: "host-edge"})
	}
}

func TestWorkflowNodeCapacityAndSharedCPU(t *testing.T) {
	p := workflowCPUFixture()
	for i := range p.Nodes {
		p.Nodes[i].Config.Capacity = 1
	}
	r := runOK(t, p)
	// Independent node queues: requests B,B,A finish at10,20,12.
	if r.Metrics.P95LatencyMs != 19 || r.Metrics.AverageLatencyMs != 13 {
		t.Fatalf("independent node queues: %+v", r.Metrics)
	}
	for i := range p.Nodes {
		p.Nodes[i].Config.Capacity = 2
	}
	colocateWorkflow(&p)
	for _, tc := range []struct {
		cores   int
		p95     int64
		average float64
	}{{1, 29, 67.0 / 3}, {2, 18, 38.0 / 3}} {
		p.Infrastructure.Hosts[1].CPUCores = tc.cores
		r = runOK(t, p)
		// One core: zero-time router/DB admissions queue behind processing,
		// so completion times are10,30,30; two cores give10,11,20.
		if r.Metrics.P95LatencyMs != tc.p95 || math.Abs(r.Metrics.AverageLatencyMs-tc.average) > 1e-12 || r.Infrastructure.Hosts[1].BusyCoreMs != 30 {
			t.Fatalf("%d shared cores: %+v, %+v", tc.cores, r.Metrics, r.Infrastructure.Hosts[1])
		}
		assertInfrastructureConservation(t, p, r)
	}
}

func TestWorkflowBranchPowerLossAndRecovery(t *testing.T) {
	p := workflowCPUFixture()
	p.Scenario.TimeoutMs, p.Scenario.MaxRetries = 100, 1
	colocateWorkflow(&p)
	for i := range p.Infrastructure.Placements {
		if id := p.Infrastructure.Placements[i].NodeID; id == "b" || id == "store-b" {
			p.Infrastructure.Placements[i].HostID = "host-cloud"
		}
	}
	p.Infrastructure.Hosts[2].Outages = []model.Outage{{StartMs: 0, EndMs: 50}}
	p.Infrastructure.Links[2].LatencyMs = 0
	r := runOK(t, p)
	// First B attempts cannot cross to the down host. A completes at12.
	// Retried B requests complete at112 and113 (both112ms from first send).
	if r.Metrics.Completed != 3 || r.Metrics.Attempts != 5 || r.Metrics.Writes != 3 || r.Metrics.P95LatencyMs != 112 || r.Metrics.AverageLatencyMs != 78 {
		t.Fatalf("branch-specific outage/recovery: %+v", r.Metrics)
	}
	workflowSelections(t, r)
	m := r.Infrastructure.Links[2]
	if m.Transfers != 6 || m.DroppedTransfers != 2 || m.BytesTransferred != 4000 {
		t.Fatalf("branch physical-link accounting: %+v", m)
	}
	assertInfrastructureConservation(t, p, r)
}

func TestWorkflowDeterminismJitterAndBounds(t *testing.T) {
	p := workflowFixture()
	p.Scenario.IntervalMs = 2
	p.Scenario.TimeoutMs = 100
	p.Scenario.MaxRetries = 2
	p.Scenario.DropFirstResponse = true
	before, _ := json.Marshal(p)
	baseline := workflowSelections(t, runOK(t, p))
	p.Scenario.JitterMs = 9
	r := runOK(t, p)
	if !reflect.DeepEqual(r, runOK(t, p)) {
		t.Fatal("same seed/model must reproduce the complete run")
	}
	if !reflect.DeepEqual(baseline, workflowSelections(t, r)) {
		t.Fatal("jitter changed logical routing choices")
	}
	p.Scenario.JitterMs = 0
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("simulation mutated the project")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled run: %v", err)
	}
	p.Scenario.RequestCount, p.Scenario.IntervalMs, p.Scenario.TimeoutMs = 200, 1, 1
	p.Scenario.MaxRetries, p.Scenario.DropFirstResponse = 0, false
	for i := range p.Nodes {
		p.Nodes[i].Config.Capacity = 1
		if p.Nodes[i].Type == "service" {
			p.Nodes[i].Config.ProcessingMs = 60000
		}
	}
	if partial, err := Run(context.Background(), p); !errors.Is(err, ErrLimit) || partial.ProjectID != "" {
		t.Fatalf("bounded branch backlog must fail without partial success: %+v, %v", partial.Metrics, err)
	}
}

func TestWorkflowPortablePackageProfilesUseConcreteNodeConfig(t *testing.T) {
	p := model.BranchingExampleProject()
	if len(p.Packages) == 0 {
		t.Fatal("example lacks portable package locks")
	}
	bound := runOK(t, p)
	p.Packages = nil
	for i := range p.Nodes {
		p.Nodes[i].Component = nil
	}
	unbound := runOK(t, p)
	if !reflect.DeepEqual(bound.Events, unbound.Events) || bound.Metrics != unbound.Metrics || !reflect.DeepEqual(bound.Infrastructure, unbound.Infrastructure) {
		t.Fatal("package provenance changed concrete kernel execution")
	}
	if bound.ModelHash == unbound.ModelHash {
		t.Fatal("package provenance must be part of the snapshot hash")
	}
}

func assertWorkflowConservation(t *testing.T, p model.Project, r model.Run) {
	t.Helper()
	workflowSelections(t, r)
	if r.Metrics.Completed+r.Metrics.Failed != r.Metrics.Requests || r.Metrics.Writes+r.Metrics.Deduplicated > r.Metrics.Attempts {
		t.Fatalf("exclusive request/write conservation: %+v", r.Metrics)
	}
	paths, forwards := map[string][]string{}, map[string]int{}
	stores, writes := map[string]string{}, map[string]int{}
	for _, event := range r.Events {
		attempt := fmt.Sprintf("%s/%d", event.RequestID, event.Attempt)
		switch event.Kind {
		case "request_arrived":
			for _, node := range paths[attempt] {
				if node == event.NodeID {
					t.Fatalf("attempt duplicated within DAG: %+v", event)
				}
			}
			paths[attempt] = append(paths[attempt], event.NodeID)
		case "write", "duplicate_write", "deduplicated":
			if store, found := stores[event.RequestID]; found && store != event.NodeID {
				t.Fatalf("one logical request reached different terminal stores: %+v", event)
			}
			stores[event.RequestID] = event.NodeID
			key := event.NodeID + "/" + event.RequestID
			if event.Kind == "write" {
				if writes[key] != 0 {
					t.Fatalf("repeat write not counted as duplicate: %+v", event)
				}
				writes[key]++
			} else {
				if writes[key] == 0 {
					t.Fatalf("dedup/duplicate without a prior durable write: %+v", event)
				}
				if event.Kind == "duplicate_write" {
					writes[key]++
				}
			}
		case "response_forwarded":
			path := paths[attempt]
			position := len(path) - 2 - forwards[attempt]
			if position < 0 || path[position] != event.NodeID {
				t.Fatalf("response did not reverse the actual request path: %+v; path%v", event, path)
			}
			forwards[attempt]++
		case "completed", "response_ignored":
			if forwards[attempt] != len(paths[attempt])-1 {
				t.Fatalf("response skipped a return hop: %+v; path%v", event, paths[attempt])
			}
		}
		if p.Infrastructure == nil && (event.HostID != "" || event.LinkID != "") {
			t.Fatalf("application-only physical identity: %+v", event)
		}
	}
	if p.Infrastructure != nil {
		assertInfrastructureConservation(t, p, r)
	} else if r.Infrastructure != nil {
		t.Fatal("application-only run fabricated physical metrics")
	}
}

func FuzzWorkflowConservation(f *testing.F) {
	f.Add([]byte{42, 3, 8, 1, 50, 4, 2, 0, 25})
	f.Add([]byte{0, 1, 4, 2, 10, 0, 1, 3, 8})
	f.Add([]byte{7, 99, 2, 0, 80, 8, 0, 2, 4})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) != 9 {
			t.Skip()
		}
		p := workflowFixture()
		p.Scenario.Seed = int64(data[0])
		p.Edges[0].RouteWeight = int(data[1]%100) + 1
		p.Scenario.RequestCount = int(data[2]%10) + 1
		p.Scenario.IntervalMs = 1
		p.Scenario.MaxRetries = int(data[3] % 3)
		p.Scenario.TimeoutMs = int64(data[4]%100) + 10
		p.Scenario.JitterMs = int64(data[5] % 9)
		p.Scenario.DropFirstResponse = data[6]&1 != 0
		for i := range p.Nodes {
			p.Nodes[i].Config.Capacity = int(data[6]%3) + 1
		}
		if data[7]&1 != 0 {
			// Reconverge both selected branches into A's durable store.
			nodes := []model.Node{}
			for _, n := range p.Nodes {
				if n.ID != "store-b" {
					nodes = append(nodes, n)
				}
			}
			p.Nodes = nodes
			p.Edges[3].Target = "store-a"
		}
		if data[7]&2 != 0 {
			colocateWorkflow(&p)
			p.Infrastructure.Hosts[1].CPUCores = int(data[6]%3) + 1
			for i := range p.Infrastructure.Placements {
				if p.Infrastructure.Placements[i].NodeID == "b" {
					p.Infrastructure.Placements[i].HostID = "host-cloud"
				}
			}
			p.Infrastructure.Hosts[2].Outages = []model.Outage{{StartMs: 10, EndMs: 50 + int64(data[5])}}
			p.Infrastructure.Links[2].BandwidthKbps = int64(data[8]) + 1
			p.Infrastructure.Links[2].Outages = []model.Outage{{StartMs: 20, EndMs: 40 + int64(data[4])}}
		}
		r := runOK(t, p)
		assertWorkflowConservation(t, p, r)
		if !reflect.DeepEqual(r, runOK(t, p)) {
			t.Fatal("same workflow/seed produced different results")
		}
	})
}
