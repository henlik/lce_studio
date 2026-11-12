// Package model defines the portable project model shared by the studio API and simulator.
package model

type Project struct {
	SchemaVersion  int             `json:"schemaVersion"`
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Nodes          []Node          `json:"nodes"`
	Edges          []Edge          `json:"edges"`
	Requirements   []Requirement   `json:"requirements"`
	Scenario       Scenario        `json:"scenario"`
	Infrastructure *Infrastructure `json:"infrastructure,omitempty"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Node struct {
	ID       string     `json:"id"`
	Type     string     `json:"type"`
	Name     string     `json:"name"`
	Position Position   `json:"position"`
	Config   NodeConfig `json:"config"`
}

type NodeConfig struct {
	ProcessingMs int64 `json:"processingMs"`
	Capacity     int   `json:"capacity"`
	Deduplicate  bool  `json:"deduplicate"`
	MemoryMB     int64 `json:"memoryMb,omitempty"`
}

type Edge struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	LatencyMs int64  `json:"latencyMs"`
}

type Requirement struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Rule     string `json:"rule"`
	TargetMs int64  `json:"targetMs"`
}

type Scenario struct {
	RequestCount      int   `json:"requestCount"`
	IntervalMs        int64 `json:"intervalMs"`
	TimeoutMs         int64 `json:"timeoutMs"`
	MaxRetries        int   `json:"maxRetries"`
	DropFirstResponse bool  `json:"dropFirstResponse"`
	JitterMs          int64 `json:"jitterMs"`
	Seed              int64 `json:"seed"`
}

type ValidationIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type Validation struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues"`
}

type TraceEvent struct {
	Sequence  int    `json:"sequence"`
	TimeMs    int64  `json:"timeMs"`
	Kind      string `json:"kind"`
	NodeID    string `json:"nodeId"`
	RequestID string `json:"requestId"`
	Attempt   int    `json:"attempt"`
	Message   string `json:"message"`
	HostID    string `json:"hostId,omitempty"`
	LinkID    string `json:"linkId,omitempty"`
}

type Verdict struct {
	RequirementID string  `json:"requirementId"`
	Text          string  `json:"text"`
	Rule          string  `json:"rule"`
	Passed        bool    `json:"passed"`
	Actual        float64 `json:"actual"`
	Expected      float64 `json:"expected"`
	Message       string  `json:"message"`
}

type Metrics struct {
	Requests         int     `json:"requests"`
	Completed        int     `json:"completed"`
	Failed           int     `json:"failed"`
	Attempts         int     `json:"attempts"`
	Writes           int     `json:"writes"`
	DuplicateWrites  int     `json:"duplicateWrites"`
	Deduplicated     int     `json:"deduplicated"`
	P95LatencyMs     int64   `json:"p95LatencyMs"`
	AverageLatencyMs float64 `json:"averageLatencyMs"`
	DurationMs       int64   `json:"durationMs"`
}

type Run struct {
	ID             string                 `json:"id"`
	ProjectID      string                 `json:"projectId"`
	CreatedAt      string                 `json:"createdAt"`
	ModelHash      string                 `json:"modelHash"`
	EngineVersion  string                 `json:"engineVersion"`
	Seed           int64                  `json:"seed"`
	Metrics        Metrics                `json:"metrics"`
	Verdicts       []Verdict              `json:"verdicts"`
	Events         []TraceEvent           `json:"events"`
	Project        Project                `json:"project"`
	Infrastructure *InfrastructureMetrics `json:"infrastructure,omitempty"`
}

func ExampleProject() Project {
	return Project{
		SchemaVersion: 1, ID: "retry-lab", Name: "Reliable request pipeline",
		Description: "A lost response triggers a retry. Compare database deduplication on and off to see why a retry can repeat an operation.",
		Nodes: []Node{
			{ID: "client", Type: "client", Name: "Client", Position: Position{60, 160}, Config: NodeConfig{Capacity: 1}},
			{ID: "service", Type: "service", Name: "Order service", Position: Position{360, 160}, Config: NodeConfig{ProcessingMs: 15, Capacity: 2}},
			{ID: "database", Type: "database", Name: "Ledger", Position: Position{660, 160}, Config: NodeConfig{ProcessingMs: 5, Capacity: 1, Deduplicate: true}},
		},
		Edges: []Edge{
			{ID: "client-service", Source: "client", Target: "service", LatencyMs: 10},
			{ID: "service-database", Source: "service", Target: "database", LatencyMs: 10},
		},
		Requirements: []Requirement{
			{ID: "REQ-001", Text: "A retried request must not create a duplicate operation.", Rule: "no_duplicates"},
			{ID: "REQ-002", Text: "Every request receives a successful response before retries are exhausted.", Rule: "all_complete"},
			{ID: "REQ-003", Text: "95% of requests complete within 300 ms, including retries.", Rule: "latency_p95", TargetMs: 300},
		},
		Scenario: Scenario{RequestCount: 12, IntervalMs: 40, TimeoutMs: 100, MaxRetries: 2, DropFirstResponse: true, Seed: 42},
	}
}
