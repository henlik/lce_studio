package model

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ValidateDraft checks bounded fields and references while allowing incomplete topology.
// An empty project ID is allowed until the API assigns one on save.
func ValidateDraft(p Project) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	text := func(path, value string, max int, required bool) {
		if (required && strings.TrimSpace(value) == "") || len(value) > max || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			add(path, fmt.Sprintf("Must be valid text%s of at most %d bytes.", map[bool]string{true: " (not blank)", false: ""}[required], max))
		}
		for _, r := range value {
			if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
				add(path, "Unsupported control character in text.")
				break
			}
		}
	}
	bound := func(path string, v, low, high int64) {
		if v < low || v > high {
			add(path, fmt.Sprintf("Must be an integer from %d to %d.", low, high))
		}
	}
	id := func(path, value string) {
		if !safeID.MatchString(value) {
			add(path, "Use 1–128 letters, numbers, dots, underscores or dashes, starting with a letter or number.")
		}
	}
	if p.SchemaVersion != 1 && p.SchemaVersion != 2 && p.SchemaVersion != 3 {
		add("schemaVersion", "Supported schema versions are 1 (linear application), 2 (linear deployment) and 3 (exclusive workflows and packages).")
	}
	if p.SchemaVersion == 1 && p.Infrastructure != nil {
		add("infrastructure", "Infrastructure requires schema version 2 or 3.")
	}
	if p.SchemaVersion == 2 && p.Infrastructure == nil {
		add("infrastructure", "Schema version 2 requires an infrastructure model.")
	}
	if p.ID != "" {
		id("id", p.ID)
	}
	text("name", p.Name, 200, true)
	text("description", p.Description, 4000, false)
	if p.Nodes == nil {
		add("nodes", "Must be an array (use [] for an empty draft).")
	}
	if p.Edges == nil {
		add("edges", "Must be an array (use [] for an empty draft).")
	}
	if p.Requirements == nil {
		add("requirements", "Must be an array (use [] if no requirements are defined).")
	}
	if len(p.Nodes) > 30 {
		add("nodes", "At most 30 nodes are supported.")
	}
	if len(p.Edges) > 30 {
		add("edges", "At most 30 edges are supported.")
	}
	if len(p.Requirements) > 50 {
		add("requirements", "At most 50 requirements are supported.")
	}
	if len(p.Packages) > 50 {
		add("packages", "At most 50 embedded package versions are supported.")
	}
	if p.SchemaVersion != 3 && len(p.Packages) != 0 {
		add("packages", "Component packages require schema version 3.")
	}
	// Bound work even for direct callers that bypass the HTTP body limit.
	if len(p.Nodes) > 30 || len(p.Edges) > 30 || len(p.Requirements) > 50 || len(p.Packages) > 50 {
		return issues
	}
	nodes := make(map[string]bool)
	nodeTypes := make(map[string]string)
	for i, n := range p.Nodes {
		path := fmt.Sprintf("nodes[%d]", i)
		id(path+".id", n.ID)
		if nodes[n.ID] {
			add(path+".id", "Node ID must be unique.")
		}
		nodes[n.ID] = true
		nodeTypes[n.ID] = n.Type
		text(path+".name", n.Name, 200, true)
		if n.Type != "client" && n.Type != "service" && n.Type != "database" && !(p.SchemaVersion == 3 && n.Type == "router") {
			add(path+".type", "Supported node types are client, service and database; routers require schema version 3.")
		}
		if p.SchemaVersion != 3 && n.Component != nil {
			add(path+".component", "Component references require schema version 3.")
		}
		if p.SchemaVersion == 3 && (n.Type == "service" || n.Type == "router") && n.Config.Deduplicate {
			add(path+".config.deduplicate", "Deduplication is supported only by database processing nodes.")
		}
		if math.IsNaN(n.Position.X) || math.IsInf(n.Position.X, 0) || math.Abs(n.Position.X) > 1e6 {
			add(path+".position.x", "Position must be finite and between -1000000 and 1000000.")
		}
		if math.IsNaN(n.Position.Y) || math.IsInf(n.Position.Y, 0) || math.Abs(n.Position.Y) > 1e6 {
			add(path+".position.y", "Position must be finite and between -1000000 and 1000000.")
		}
		bound(path+".config.processingMs", n.Config.ProcessingMs, 0, 60000)
		bound(path+".config.capacity", int64(n.Config.Capacity), 1, 100)
		bound(path+".config.memoryMb", n.Config.MemoryMB, 0, 1048576)
		if p.SchemaVersion == 1 && n.Config.MemoryMB != 0 {
			add(path+".config.memoryMb", "Memory reservations require schema version 2 or 3.")
		}
	}
	edges := make(map[string]bool)
	for i, e := range p.Edges {
		path := fmt.Sprintf("edges[%d]", i)
		id(path+".id", e.ID)
		if edges[e.ID] {
			add(path+".id", "Edge ID must be unique.")
		}
		edges[e.ID] = true
		if !nodes[e.Source] {
			add(path+".source", "Source must refer to an existing node.")
		}
		if !nodes[e.Target] {
			add(path+".target", "Target must refer to an existing node.")
		}
		bound(path+".latencyMs", e.LatencyMs, 0, 60000)
		if p.SchemaVersion != 3 && e.RouteWeight != 0 {
			add(path+".routeWeight", "Routing weights require schema version 3.")
		} else if e.RouteWeight < 0 || e.RouteWeight > 100 {
			add(path+".routeWeight", "Routing weights must be from 0 to 100; zero uses the default weight of 1.")
		} else if e.RouteWeight != 0 && nodeTypes[e.Source] != "router" {
			add(path+".routeWeight", "Routing weights are supported only on edges from routers.")
		}
	}
	requirements := make(map[string]bool)
	for i, r := range p.Requirements {
		path := fmt.Sprintf("requirements[%d]", i)
		id(path+".id", r.ID)
		if requirements[r.ID] {
			add(path+".id", "Requirement ID must be unique.")
		}
		requirements[r.ID] = true
		text(path+".text", r.Text, 2000, true)
		if r.Rule != "no_duplicates" && r.Rule != "all_complete" && r.Rule != "latency_p95" {
			add(path+".rule", "Supported rules are no_duplicates, all_complete and latency_p95.")
		}
		bound(path+".targetMs", r.TargetMs, 0, 3600000)
	}
	s := p.Scenario
	bound("scenario.requestCount", int64(s.RequestCount), 1, 200)
	bound("scenario.intervalMs", s.IntervalMs, 1, 60000)
	bound("scenario.timeoutMs", s.TimeoutMs, 1, 60000)
	bound("scenario.maxRetries", int64(s.MaxRetries), 0, 5)
	bound("scenario.jitterMs", s.JitterMs, 0, 1000)
	bound("scenario.seed", s.Seed, 0, 2147483647)
	if p.Infrastructure != nil {
		issues = append(issues, validateInfrastructureDraft(p)...)
	}
	if p.SchemaVersion == 3 {
		issues = append(issues, validateProjectPackages(p)...)
		issues = append(issues, validateWorkflowDraft(p)...)
	}
	return issues
}

// Validate adds executable topology and placement constraints to draft validation.
func Validate(p Project) []ValidationIssue {
	issues := ValidateDraft(p)
	if len(issues) > 0 {
		return issues
	}
	if p.SchemaVersion == 3 {
		issues = append(issues, validateWorkflowExecution(p)...)
		if p.Infrastructure != nil {
			issues = append(issues, validateInfrastructureExecution(p)...)
		}
		return issues
	}
	add := func(path, message string) { issues = append(issues, ValidationIssue{path, message}) }
	clients, services, databases := 0, 0, 0
	client := ""
	for _, n := range p.Nodes {
		switch n.Type {
		case "client":
			clients++
			client = n.ID
		case "service":
			services++
		case "database":
			databases++
		}
	}
	if clients != 1 {
		add("nodes", "The simulation requires exactly one client.")
	}
	if services < 1 {
		add("nodes", "The simulation requires at least one service.")
	}
	if databases != 1 {
		add("nodes", "The simulation requires exactly one database.")
	}
	incoming, outgoing := map[string]int{}, map[string]int{}
	next := map[string]string{}
	for _, e := range p.Edges {
		incoming[e.Target]++
		outgoing[e.Source]++
		next[e.Source] = e.Target
	}
	for i, n := range p.Nodes {
		wantIn, wantOut := 1, 1
		if n.Type == "client" {
			wantIn = 0
		}
		if n.Type == "database" {
			wantOut = 0
		}
		if incoming[n.ID] != wantIn || outgoing[n.ID] != wantOut {
			add(fmt.Sprintf("nodes[%d]", i), fmt.Sprintf("%s needs %d incoming and %d outgoing link(s). This simulator supports one linear pipeline.", n.Name, wantIn, wantOut))
		}
	}
	if clients == 1 {
		visited := map[string]bool{}
		for at := client; at != ""; at = next[at] {
			if visited[at] {
				add("edges", "Cycles are not supported; connect client → service(s) → database.")
				break
			}
			visited[at] = true
		}
		if len(visited) != len(p.Nodes) {
			add("edges", "Every node must be connected in the single client → service(s) → database pipeline.")
		}
	}
	if p.Infrastructure != nil {
		issues = append(issues, validateInfrastructureExecution(p)...)
	}
	return issues
}
