package model

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestWorkflowSupportsOptionalInfrastructureAndConvergence(t *testing.T) {
	p := BranchingExampleProject()
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("placed example rejected: %+v", issues)
	}
	p.Infrastructure = nil
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("application-only workflow rejected: %+v", issues)
	}
	// Two exclusive paths may converge on one store. Only one path is taken.
	p.Nodes = p.Nodes[:len(p.Nodes)-1]
	p.Edges[len(p.Edges)-1].Target = "store-edge"
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("exclusive convergence rejected: %+v", issues)
	}
	for i := range p.Edges {
		p.Edges[i].RouteWeight = 0
	}
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("implicit equal route weights rejected: %+v", issues)
	}
	p.Edges = []Edge{}
	if issues := ValidateDraft(p); len(issues) != 0 {
		t.Fatalf("unfinished workflow rejected: %+v", issues)
	}
	if issues := Validate(p); len(issues) == 0 {
		t.Fatal("unfinished workflow accepted for simulation")
	}
}

func TestWorkflowRejectsMalformedDraftGraphs(t *testing.T) {
	tests := []struct {
		name, path string
		mutate     func(*Project)
	}{
		{"self link", "edges[5]", func(p *Project) { p.Edges = append(p.Edges, Edge{ID: "self", Source: "router", Target: "router"}) }},
		{"parallel pair", "edges[5]", func(p *Project) {
			p.Edges = append(p.Edges, Edge{ID: "duplicate-pair", Source: "router", Target: "worker-edge"})
		}},
		{"cycle", "edges", func(p *Project) { p.Edges = append(p.Edges, Edge{ID: "cycle", Source: "store-edge", Target: "router"}) }},
		{"negative weight", "edges[1].routeWeight", func(p *Project) { p.Edges[1].RouteWeight = -1 }},
		{"weight limit", "edges[1].routeWeight", func(p *Project) { p.Edges[1].RouteWeight = 101 }},
		{"nonrouter weight", "edges[0].routeWeight", func(p *Project) { p.Edges[0].RouteWeight = 1 }},
		{"router deduplication", "nodes[1].config.deduplicate", func(p *Project) { p.Nodes[1].Config.Deduplicate = true }},
		{"service deduplication", "nodes[2].config.deduplicate", func(p *Project) { p.Nodes[2].Config.Deduplicate = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := BranchingExampleProject()
			tt.mutate(&p)
			if issues := ValidateDraft(p); !issueAt(issues, tt.path) {
				t.Fatalf("expected issue at %s; got %+v", tt.path, issues)
			}
		})
	}
}

func TestWorkflowRejectsIncompleteOrParallelExecution(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Project)
	}{
		{"service fanout", func(p *Project) {
			p.Nodes[1].Type, p.Nodes[1].Component = "service", nil
			p.Edges[1].RouteWeight, p.Edges[2].RouteWeight = 0, 0
		}},
		{"router single route", func(p *Project) { p.Edges = append(p.Edges[:2], p.Edges[3:]...) }},
		{"client fanout", func(p *Project) {
			p.Edges = append(p.Edges, Edge{ID: "client-extra", Source: "client", Target: "worker-edge"})
		}},
		{"database onward edge", func(p *Project) {
			p.Edges = append(p.Edges, Edge{ID: "store-onward", Source: "store-edge", Target: "store-cloud"})
		}},
		{"disconnected node", func(p *Project) {
			p.Nodes = append(p.Nodes, Node{ID: "unreachable", Name: "Unreachable", Type: "service", Config: NodeConfig{Capacity: 1}})
		}},
		{"no client", func(p *Project) { p.Nodes[0].Type = "service" }},
		{"two clients", func(p *Project) {
			p.Nodes = append(p.Nodes, Node{ID: "client-2", Name: "Another client", Type: "client", Config: NodeConfig{Capacity: 1}})
		}},
		{"no terminal database", func(p *Project) {
			for i := range p.Nodes {
				if p.Nodes[i].Type == "database" {
					p.Nodes[i].Type, p.Nodes[i].Component, p.Nodes[i].Config.Deduplicate = "service", nil, false
				}
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := BranchingExampleProject()
			p.Infrastructure = nil
			tt.mutate(&p)
			if issues := ValidateDraft(p); len(issues) != 0 {
				t.Fatalf("incomplete acyclic topology must remain editable: %+v", issues)
			}
			if issues := Validate(p); len(issues) == 0 {
				t.Fatal("unsupported executable workflow accepted")
			}
		})
	}
}

func TestLegacySchemasRejectWorkflowDeclarationsAndKeepSerializedShape(t *testing.T) {
	for _, version := range []int{1, 2} {
		base := ExampleProject()
		path := "../../examples/retry-pipeline.json"
		if version == 2 {
			base, path = ContinuumExampleProject(), "../../examples/continuum-placement.json"
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(base)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, compact.Bytes()) {
			t.Fatalf("schema%d JSON shape changed", version)
		}
		for _, tt := range []struct {
			path   string
			mutate func(*Project)
		}{
			{"nodes[1].type", func(p *Project) { p.Nodes[1].Type = "router" }},
			{"edges[0].routeWeight", func(p *Project) { p.Edges[0].RouteWeight = 1 }},
			{"nodes[1].component", func(p *Project) { p.Nodes[1].Component = &ComponentRef{} }},
			{"packages", func(p *Project) { p.Packages = BranchingExampleProject().Packages }},
		} {
			var p Project
			if err := json.Unmarshal(encoded, &p); err != nil {
				t.Fatal(err)
			}
			tt.mutate(&p)
			if issues := ValidateDraft(p); !issueAt(issues, tt.path) {
				t.Fatalf("schema%d silently accepted %s: %+v", version, tt.path, issues)
			}
		}
	}
}
