package model

import (
	"math"
	"strings"
	"testing"
)

func TestExampleAndEmptyDraft(t *testing.T) {
	p := ExampleProject()
	if got := Validate(p); len(got) != 0 {
		t.Fatalf("example rejected: %+v", got)
	}
	p.Nodes = []Node{}
	p.Edges = []Edge{}
	if got := ValidateDraft(p); len(got) != 0 {
		t.Fatalf("incomplete draft rejected: %+v", got)
	}
	if got := Validate(p); len(got) == 0 {
		t.Fatal("incomplete project must not simulate")
	}
}

func TestValidateRejectsMalformedFields(t *testing.T) {
	tests := []struct {
		name, path string
		mutate     func(*Project)
	}{
		{"version", "schemaVersion", func(p *Project) { p.SchemaVersion = 2 }},
		{"identifier", "nodes[0].id", func(p *Project) { p.Nodes[0].ID = "../bad" }},
		{"duplicate node", "nodes[1].id", func(p *Project) { p.Nodes[1].ID = p.Nodes[0].ID }},
		{"duplicate edge", "edges[1].id", func(p *Project) { p.Edges[1].ID = p.Edges[0].ID }},
		{"duplicate requirement", "requirements[1].id", func(p *Project) { p.Requirements[1].ID = p.Requirements[0].ID }},
		{"capacity", "nodes[1].config.capacity", func(p *Project) { p.Nodes[1].Config.Capacity = 0 }},
		{"duration", "nodes[1].config.processingMs", func(p *Project) { p.Nodes[1].Config.ProcessingMs = -1 }},
		{"coordinate NaN", "nodes[0].position.x", func(p *Project) { p.Nodes[0].Position.X = math.NaN() }},
		{"coordinate infinity", "nodes[0].position.y", func(p *Project) { p.Nodes[0].Position.Y = math.Inf(1) }},
		{"dangling", "edges[0].target", func(p *Project) { p.Edges[0].Target = "unknown" }},
		{"zero requests", "scenario.requestCount", func(p *Project) { p.Scenario.RequestCount = 0 }},
		{"too many requests", "scenario.requestCount", func(p *Project) { p.Scenario.RequestCount = 201 }},
		{"negative seed", "scenario.seed", func(p *Project) { p.Scenario.Seed = -1 }},
		{"unknown rule", "requirements[0].rule", func(p *Project) { p.Requirements[0].Rule = "always_pass" }},
		{"long description", "description", func(p *Project) { p.Description = strings.Repeat("a", 4001) }},
		{"nil nodes", "nodes", func(p *Project) { p.Nodes = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ExampleProject()
			tt.mutate(&p)
			got := Validate(p)
			for _, issue := range got {
				if issue.Path == tt.path {
					return
				}
			}
			t.Fatalf("expected issue at %s; got %+v", tt.path, got)
		})
	}
}

func TestValidateRejectsUnsupportedTopology(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Project)
	}{
		{"branch", func(p *Project) { p.Edges = append(p.Edges, Edge{ID: "branch", Source: "client", Target: "database"}) }},
		{"cycle", func(p *Project) { p.Edges[1].Target = "client" }},
		{"disconnected cycle", func(p *Project) {
			p.Nodes = append(p.Nodes, Node{ID: "extra", Type: "service", Name: "Extra", Config: NodeConfig{Capacity: 1}})
			p.Edges = append(p.Edges, Edge{ID: "loop", Source: "extra", Target: "extra"})
		}},
		{"two clients", func(p *Project) { p.Nodes[1].Type = "client" }},
		{"reversed edge", func(p *Project) { p.Edges[0].Source, p.Edges[0].Target = p.Edges[0].Target, p.Edges[0].Source }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ExampleProject()
			tt.mutate(&p)
			if got := ValidateDraft(p); len(got) != 0 {
				t.Fatalf("topology should be savable as draft: %+v", got)
			}
			if len(Validate(p)) == 0 {
				t.Fatal("unsupported topology accepted")
			}
		})
	}
}
