package model

// Infrastructure describes physical resources separately from the application.
// Geography and tier are descriptive metadata; links provide explicit latency.
type Infrastructure struct {
	Sites        []Site        `json:"sites"`
	Hosts        []Host        `json:"hosts"`
	Links        []NetworkLink `json:"links"`
	Placements   []Placement   `json:"placements"`
	MessageBytes int64         `json:"messageBytes"`
}

type Site struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Host struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	SiteID      string   `json:"siteId"`
	Tier        string   `json:"tier"`
	CPUCores    int      `json:"cpuCores"`
	MemoryMB    int64    `json:"memoryMb"`
	IdleWatts   float64  `json:"idleWatts"`
	ActiveWatts float64  `json:"activeWatts"`
	Outages     []Outage `json:"outages"`
}

type NetworkLink struct {
	ID            string   `json:"id"`
	SourceHostID  string   `json:"sourceHostId"`
	TargetHostID  string   `json:"targetHostId"`
	LatencyMs     int64    `json:"latencyMs"`
	BandwidthKbps int64    `json:"bandwidthKbps"`
	Outages       []Outage `json:"outages"`
}

type Placement struct {
	NodeID string `json:"nodeId"`
	HostID string `json:"hostId"`
}

// Outage marks unavailability on the half-open interval [StartMs, EndMs).
type Outage struct {
	StartMs int64 `json:"startMs"`
	EndMs   int64 `json:"endMs"`
}

type InfrastructureMetrics struct {
	EnergyWh float64       `json:"energyWh"`
	Hosts    []HostMetrics `json:"hosts"`
	Links    []LinkMetrics `json:"links"`
}

type HostMetrics struct {
	HostID          string  `json:"hostId"`
	EnergyWh        float64 `json:"energyWh"`
	BusyCoreMs      int64   `json:"busyCoreMs"`
	UptimeMs        int64   `json:"uptimeMs"`
	InterruptedJobs int     `json:"interruptedJobs"`
}

type LinkMetrics struct {
	LinkID           string `json:"linkId"`
	Transfers        int    `json:"transfers"`
	BytesTransferred int64  `json:"bytesTransferred"`
	DroppedTransfers int    `json:"droppedTransfers"`
}

// ResidentMemoryMB returns the v2 default for an omitted memory reservation.
func ResidentMemoryMB(config NodeConfig) int64 {
	if config.MemoryMB == 0 {
		return 64
	}
	return config.MemoryMB
}

// ContinuumExampleProject is a healthy generic deployment comparison fixture.
// Move service and database from edge-host to cloud-host to compare placements.
func ContinuumExampleProject() Project {
	p := ExampleProject()
	p.SchemaVersion = 2
	p.ID = "continuum-placement"
	p.Name = "Edge and cloud placement"
	p.Description = "Compare the same request pipeline on edge or cloud resources. Edit host power and network outages to test interruption and recovery; locations are descriptive and link delays are explicit."
	p.Nodes[0].Name = "Request client"
	p.Nodes[0].Config.MemoryMB = 64
	p.Nodes[1].Name = "Request service"
	p.Nodes[1].Config.ProcessingMs = 20
	p.Nodes[1].Config.MemoryMB = 128
	p.Nodes[2].Name = "Durable store"
	p.Nodes[2].Config.ProcessingMs = 10
	p.Nodes[2].Config.MemoryMB = 256
	for i := range p.Edges {
		p.Edges[i].LatencyMs = 2
	}
	p.Requirements[2].Text = "95% of requests complete within 500 ms."
	p.Requirements[2].TargetMs = 500
	p.Scenario = Scenario{RequestCount: 12, IntervalMs: 250, TimeoutMs: 500, MaxRetries: 2, Seed: 42}
	p.Infrastructure = &Infrastructure{
		Sites: []Site{
			{ID: "field-site", Name: "Field site", Latitude: 2, Longitude: 29},
			{ID: "remote-site", Name: "Remote site", Latitude: 50.85, Longitude: 4.35},
		},
		Hosts: []Host{
			{ID: "device-host", Name: "Client device", SiteID: "field-site", Tier: "device", CPUCores: 1, MemoryMB: 512, IdleWatts: 3, ActiveWatts: 8, Outages: []Outage{}},
			{ID: "edge-host", Name: "Edge server", SiteID: "field-site", Tier: "edge", CPUCores: 4, MemoryMB: 4096, IdleWatts: 12, ActiveWatts: 45, Outages: []Outage{}},
			{ID: "cloud-host", Name: "Remote server", SiteID: "remote-site", Tier: "cloud", CPUCores: 8, MemoryMB: 16384, IdleWatts: 40, ActiveWatts: 160, Outages: []Outage{}},
		},
		Links: []NetworkLink{
			{ID: "local-link", SourceHostID: "device-host", TargetHostID: "edge-host", LatencyMs: 2, BandwidthKbps: 10000, Outages: []Outage{}},
			{ID: "device-wan", SourceHostID: "device-host", TargetHostID: "cloud-host", LatencyMs: 80, BandwidthKbps: 256, Outages: []Outage{}},
			{ID: "edge-wan", SourceHostID: "edge-host", TargetHostID: "cloud-host", LatencyMs: 80, BandwidthKbps: 256, Outages: []Outage{}},
		},
		Placements: []Placement{
			{NodeID: "client", HostID: "device-host"},
			{NodeID: "service", HostID: "edge-host"},
			{NodeID: "database", HostID: "edge-host"},
		},
		MessageBytes: 1024,
	}
	return p
}
