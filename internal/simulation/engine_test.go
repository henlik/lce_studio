package simulation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"software-studio/internal/model"
)

func runOK(t *testing.T, p model.Project) model.Run {
	t.Helper()
	result, err := Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestLostResponseDemoAndDeduplication(t *testing.T) {
	p := model.ExampleProject()
	r := runOK(t, p)
	want := model.Metrics{Requests: 12, Completed: 12, Attempts: 24, Writes: 12, Deduplicated: 12, P95LatencyMs: 160, AverageLatencyMs: 160, DurationMs: 600}
	if r.Metrics != want {
		t.Fatalf("metrics\ngot  %+v\nwant %+v", r.Metrics, want)
	}
	for _, v := range r.Verdicts {
		if !v.Passed {
			t.Errorf("demo requirement failed: %+v", v)
		}
	}
	p.Nodes[2].Config.Deduplicate = false
	unsafe := runOK(t, p)
	if unsafe.Metrics.Writes != 24 || unsafe.Metrics.DuplicateWrites != 12 || unsafe.Metrics.Completed != 12 {
		t.Fatalf("retry must repeat unprotected writes: %+v", unsafe.Metrics)
	}
	if unsafe.Verdicts[0].Passed {
		t.Fatal("duplicate operations must fail the requirement")
	}
	if !unsafe.Verdicts[1].Passed {
		t.Fatal("duplicate operations still complete; requirements are independently evaluated")
	}
	if unsafe.ModelHash == r.ModelHash {
		t.Fatal("changing behaviour must change model hash")
	}
}

func TestLostResponseWithoutRetries(t *testing.T) {
	p := model.ExampleProject()
	p.Scenario.MaxRetries = 0
	r := runOK(t, p)
	if r.Metrics.Completed != 0 || r.Metrics.Failed != 12 || r.Metrics.Writes != 12 {
		t.Fatalf("lost responses must leave committed but failed client requests: %+v", r.Metrics)
	}
	if r.Verdicts[1].Passed || r.Verdicts[2].Passed {
		t.Fatal("incomplete requests cannot pass completion or latency requirements")
	}
}

func TestAnalyticalRoundTripAndFIFOQueue(t *testing.T) {
	p := model.ExampleProject()
	p.Scenario.RequestCount = 1
	p.Scenario.DropFirstResponse = false
	r := runOK(t, p)
	// Two 10 ms links in each direction, 15 ms service + 5 ms database.
	if r.Metrics.P95LatencyMs != 60 || r.Metrics.Attempts != 1 || r.Metrics.DurationMs != 60 {
		t.Fatalf("round trip should be 60ms: %+v", r.Metrics)
	}
	p.Scenario.RequestCount = 3
	p.Scenario.IntervalMs = 1
	p.Scenario.TimeoutMs = 1000
	p.Nodes[1].Config.ProcessingMs = 10
	p.Nodes[1].Config.Capacity = 1
	p.Nodes[2].Config.ProcessingMs = 0
	for i := range p.Edges {
		p.Edges[i].LatencyMs = 0
	}
	r = runOK(t, p)
	// Arrival times 0,1,2; serial completion times 10,20,30 => 10,19,28.
	if r.Metrics.AverageLatencyMs != 19 || r.Metrics.P95LatencyMs != 28 || r.Metrics.DurationMs != 30 {
		t.Fatalf("FIFO queue analytical result differs: %+v", r.Metrics)
	}
	queued := 0
	for _, event := range r.Events {
		if event.Kind == "queued" {
			queued++
		}
	}
	if queued != 2 {
		t.Fatalf("expected two queued requests, got %d", queued)
	}
	p.Nodes[1].Config.Capacity = 3
	r = runOK(t, p)
	if r.Metrics.P95LatencyMs != 10 || r.Metrics.AverageLatencyMs != 10 {
		t.Fatalf("capacity 3 removes queueing for this workload: %+v", r.Metrics)
	}
}

func TestMultipleServicesAndZeroDuration(t *testing.T) {
	p := model.ExampleProject()
	p.Scenario.RequestCount = 1
	p.Scenario.DropFirstResponse = false
	p.Nodes = append(p.Nodes, model.Node{ID: "audit", Name: "Audit", Type: "service", Config: model.NodeConfig{ProcessingMs: 7, Capacity: 1}})
	p.Edges[1].Source = "audit"
	p.Edges = append(p.Edges, model.Edge{ID: "service-audit", Source: "service", Target: "audit", LatencyMs: 3})
	r := runOK(t, p)
	if r.Metrics.P95LatencyMs != 73 {
		t.Fatalf("additional service + link adds 7 + 2*3ms: %+v", r.Metrics)
	}
	for i := range p.Nodes {
		p.Nodes[i].Config.ProcessingMs = 0
	}
	for i := range p.Edges {
		p.Edges[i].LatencyMs = 0
	}
	r = runOK(t, p)
	if r.Metrics.Completed != 1 || r.Metrics.P95LatencyMs != 0 {
		t.Fatalf("zero-duration events must terminate: %+v", r.Metrics)
	}
}

func TestTimeoutDoesNotCancelServerWork(t *testing.T) {
	p := model.ExampleProject()
	p.Scenario.RequestCount = 1
	p.Scenario.DropFirstResponse = false
	p.Scenario.MaxRetries = 0
	p.Nodes[1].Config.ProcessingMs = 200
	p.Nodes[2].Config.ProcessingMs = 0
	for i := range p.Edges {
		p.Edges[i].LatencyMs = 0
	}
	r := runOK(t, p)
	if r.Metrics.Completed != 0 || r.Metrics.Failed != 1 || r.Metrics.Writes != 1 || r.Metrics.DurationMs != 200 {
		t.Fatalf("client expires but in-flight write continues: %+v", r.Metrics)
	}
	ignored := false
	for _, event := range r.Events {
		if event.Kind == "response_ignored" {
			ignored = true
		}
	}
	if !ignored {
		t.Fatal("trace should explain late ignored response")
	}
}

func TestDeadlineTieUsesStableSchedulingOrder(t *testing.T) {
	p := model.ExampleProject()
	p.Scenario.RequestCount = 1
	p.Scenario.DropFirstResponse = false
	p.Scenario.MaxRetries = 0
	p.Nodes[1].Config.ProcessingMs = p.Scenario.TimeoutMs
	p.Nodes[2].Config.ProcessingMs = 0
	for i := range p.Edges {
		p.Edges[i].LatencyMs = 0
	}
	r := runOK(t, p)
	if r.Metrics.Completed != 0 || r.Metrics.Failed != 1 || r.Metrics.Writes != 1 {
		t.Fatalf("earlier scheduled deadline wins exact-time tie: %+v", r.Metrics)
	}
}

func TestReproducibleJitterAndUnmodifiedInput(t *testing.T) {
	p := model.ExampleProject()
	p.Scenario.JitterMs = 20
	before, _ := json.Marshal(p)
	a := runOK(t, p)
	b := runOK(t, p)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same project/seed must reproduce entire computed run")
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("simulation mutated input")
	}
	p.Scenario.Seed++
	c := runOK(t, p)
	if reflect.DeepEqual(a.Events, c.Events) {
		t.Fatal("different seed should change jitter trace for this scenario")
	}
	var last int64 = -1
	for i, event := range a.Events {
		if event.TimeMs < last || event.Sequence != i+1 {
			t.Fatal("trace is not chronological with stable sequence")
		}
		last = event.TimeMs
	}
}

func TestLimitsCancellationAndInvalidModel(t *testing.T) {
	p := model.ExampleProject()
	p.Edges = nil
	if _, err := Run(context.Background(), p); err == nil {
		t.Fatal("invalid topology must fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, model.ExampleProject()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled run returned %v", err)
	}
	p = model.ExampleProject()
	p.Scenario.RequestCount = 200
	p.Scenario.IntervalMs = 60000
	if r, err := Run(context.Background(), p); !errors.Is(err, ErrLimit) || len(r.Verdicts) > 0 {
		t.Fatalf("horizon must return an error without partial passing results: %v", err)
	}
}

func TestTraceLimit(t *testing.T) {
	p := model.ExampleProject()
	p.Nodes = p.Nodes[:1]
	p.Edges = []model.Edge{}
	previous := "client"
	for i := 0; i < 28; i++ {
		id := fmt.Sprintf("service-%d", i)
		p.Nodes = append(p.Nodes, model.Node{ID: id, Name: id, Type: "service", Config: model.NodeConfig{ProcessingMs: 1, Capacity: 100}})
		p.Edges = append(p.Edges, model.Edge{ID: "edge-" + id, Source: previous, Target: id})
		previous = id
	}
	p.Nodes = append(p.Nodes, model.Node{ID: "database", Name: "Database", Type: "database", Config: model.NodeConfig{Capacity: 100, Deduplicate: true}})
	p.Edges = append(p.Edges, model.Edge{ID: "edge-database", Source: previous, Target: "database"})
	p.Scenario = model.Scenario{RequestCount: 200, IntervalMs: 1, TimeoutMs: 1, MaxRetries: 5, Seed: 42}
	if r, err := Run(context.Background(), p); !errors.Is(err, ErrLimit) || len(r.Verdicts) > 0 {
		t.Fatalf("trace cap must fail explicitly without partial verdicts: events=%d err=%v", len(r.Events), err)
	}
}

func FuzzDeterministicSimulation(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5})
	f.Add([]byte{255, 0, 255, 1, 0})
	f.Add([]byte{0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) < 5 {
			return
		}
		p := model.ExampleProject()
		p.Scenario.RequestCount = int(b[0]%5) + 1
		p.Scenario.Seed = int64(b[1])
		p.Scenario.JitterMs = int64(b[2] % 20)
		p.Scenario.TimeoutMs = int64(b[3]) + 1
		p.Scenario.MaxRetries = int(b[4] % 3)
		a := runOK(t, p)
		c := runOK(t, p)
		if !reflect.DeepEqual(a, c) {
			t.Fatal("same model produced different results")
		}
		if a.Metrics.Completed+a.Metrics.Failed != a.Metrics.Requests {
			t.Fatal("requests not conserved")
		}
		if a.Metrics.Writes != a.Metrics.Requests || a.Metrics.DuplicateWrites != 0 {
			t.Fatal("deduplicating store must commit each logical request once despite retries")
		}
	})
}
