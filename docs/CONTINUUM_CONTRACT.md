# Continuum increment contract — v0.2

Implementation contract for the first generic deployment simulation increment. The broader product roadmap includes extensible packages, batteries, mobility, application generation, and real deployment; this increment does not claim those capabilities.

## Compatibility and ownership

Existing schemaVersion 1 projects and runs remain readable and executable with their original numerical semantics. A project with infrastructure uses schemaVersion 2. A v1 project must not carry infrastructure that would be ignored. The software topology remains the supported client → service(s) → database pipeline. All application concepts are domain-independent; healthcare is a reference use case, never a core type.

Model agent owns Go types/validation and examples; simulation agent owns engine/semantics; frontend agent owns web/src; root owns server integration, versions, roadmap and component contract.

## Exact JSON additions

Project gains `infrastructure?: Infrastructure`. NodeConfig gains `memoryMb?: number`, a resident memory reservation in MiB; missing or zero means 64 MiB in schema v2. Run gains `infrastructure?: InfrastructureMetrics`. Existing Metrics remains unchanged. TraceEvent gains optional `hostId?: string` and `linkId?: string` (Go HostID/LinkID, omitempty). Infrastructure events leave nodeId and requestId empty when no application node/request applies; use the new IDs for host/link replay highlighting.

```ts
interface Outage { startMs: number; endMs: number }
interface Site { id: string; name: string; latitude: number; longitude: number }
interface Host {
  id: string; name: string; siteId: string;
  tier: 'device' | 'edge' | 'regional' | 'cloud';
  cpuCores: number; memoryMb: number;
  idleWatts: number; activeWatts: number;
  outages: Outage[];
}
interface NetworkLink {
  id: string; sourceHostId: string; targetHostId: string;
  latencyMs: number; bandwidthKbps: number; outages: Outage[];
}
interface Placement { nodeId: string; hostId: string }
interface Infrastructure {
  sites: Site[]; hosts: Host[]; links: NetworkLink[];
  placements: Placement[]; messageBytes: number;
}
interface HostMetrics {
  hostId: string; energyWh: number; busyCoreMs: number;
  uptimeMs: number; interruptedJobs: number;
}
interface LinkMetrics {
  linkId: string; transfers: number; bytesTransferred: number;
  droppedTransfers: number;
}
interface InfrastructureMetrics {
  energyWh: number; hosts: HostMetrics[]; links: LinkMetrics[];
}
```

Go names: Infrastructure, Site, Host, NetworkLink, Placement, Outage, InfrastructureMetrics, HostMetrics, LinkMetrics. Use conventional `ID`, `SiteID`, `SourceHostID`, `TargetHostID`, `NodeID`, `HostID`, `LinkID`, `MemoryMB`, `CPUCores`, `BandwidthKbps`, `EnergyWh`, `BusyCoreMs`, `UptimeMs`, `InterruptedJobs`, `BytesTransferred`, `DroppedTransfers`. JSON casing above is authoritative. New optional fields must use omitempty so v1 snapshots/hashes are preserved.

## Validation and bounds

- v1 and v2 only; v2 requires infrastructure. Drafts may omit placement coverage or be incomplete software topologies, but included objects, references and values must be well formed.
- At most 30 sites, 30 hosts, 60 physical links, 30 placements, 20 outage windows per host/link. IDs are bounded, stable, unique; use the existing ID rules. Within infrastructure, sites/hosts/links must have globally unique IDs and must not collide with software node IDs, so replay highlighting is unambiguous.
- Latitude [-90,90], longitude [-180,180], finite numbers. Coordinates/tier describe location and deployment class; neither automatically generates latency or CPU speed.
- CPU cores integer 1..128; host memory integer 1..1048576 MiB; node memory integer 0..1048576 MiB. In executable v2 projects the sum of resident node reservations on a host cannot exceed host memory. Do not sum node processing Capacity as CPU reservations: contention is simulated.
- Power values finite 0..1000000 W; activeWatts >= idleWatts.
- Link latency integer 0..60000 ms, bandwidth integer 1..10000000 kbit/s; messageBytes integer 1..10485760 bytes.
- Outages are half-open [startMs,endMs), integer 0 <= start < end <= 3600000. Reject overlapping windows; touching windows are allowed. Input order need not be sorted.
- Every application node has exactly one placement when executing. Duplicate placements are invalid even in drafts. Physical links are undirected, no self links, no duplicate unordered host pairs. Each software edge must be mapped to the same host or one direct physical link; multihop routing is not implemented.
- Keep all existing application/scenario bounds and fail explicitly if execution exceeds them.

## Execution semantics

- Preserve the v1 engine path. Dispatch v2 to an infrastructure-aware engine.
- Each service/database job requires one host CPU core for node.config.processingMs. Node capacity still limits its concurrency. Host CPU cores are shared across colocated nodes using deterministic work-conserving FIFO admission (earliest eligible job). Memory is a static admission check, not a simulated allocator.
- Each crossing of a physical link, in either direction, transfers messageBytes. The link is one shared FIFO transmission channel. Serialization time is ceil(messageBytes*8/bandwidthKbps) milliseconds. Transfer delay also includes physical latency, application edge latency, and seeded scenario jitter. Same-host communication uses application edge latency and jitter without physical bandwidth serialization.
- Host/link outages affect work and communication, including a failure while work is in flight. They must not merely change labels. Power-down interrupts volatile running/queued processing on that host; completed database writes and deduplication state persist. Retry remains the existing bounded client mechanism; no invented automatic application recovery that masks failures.
- The external workload generator retains request schedules and timeout state. A client-host outage can prevent its send/receive; an already scheduled timeout still expires and can trigger the next bounded retry. This is an explicit workload abstraction rather than a model of client-device persistent storage.
- At equal times, environmental transitions are applied before work; availability at time t follows [start,end). A response at an outage start can be lost after its write committed.
- Network transfers that fail are dropped; no packet-level retransmission or migration. Document exact queue/interruption details in SIMULATION.md and test them independently. `transfers` counts attempted physical-link transfers, `bytesTransferred` counts full successfully delivered payload bytes, and `droppedTransfers` counts failed transfers (not partial byte estimates).
- CPU-related host power = idleWatts + (activeWatts-idleWatts)*(busyCores/cpuCores) while powered; 0 while down. Integrate W*ms/3600000 to Wh. This excludes network/storage device energy and battery state. All defined hosts contribute their idle energy while up, even if no app is placed on them.
- The run horizon ends when all work, client timeouts, and configured environmental transitions have drained. All host energy/uptime metrics cover [0, final event time]. busyCoreMs sums occupied core-time. Report the assumptions in the UI/doc; these are uncalibrated estimates, not measured hardware predictions.
- Stable deterministic ordering, seeded jitter, cancellation, event/time bounds and non-mutating input remain mandatory.

## API and editor

GET /api/example continues to return the legacy reference pipeline. GET /api/example?template=continuum returns model.ContinuumExampleProject(). Existing project/run/validation endpoints operate on both schema versions.

The Deployment workspace edits infrastructure and placements and invokes the real API. Run comparison selects immutable recorded runs and labels their snapshots explicitly; changing placement must not relabel an earlier result as current. Editor can enable infrastructure on a v1 design, making it v2; removing it returns to v1 and removes infrastructure-only memory declarations if necessary.

## First reference fixture

A generic client → request service → durable store model with device, edge and cloud hosts, explicit locations and direct links. Start with service/store placed at edge, then move both to cloud and run the same workload. Include configurable edge power and WAN outages so comparison demonstrates changed completion/latency/energy. Provide at least one healthy baseline that passes and one deliberate outage scenario whose failure is expected. No healthcare-specific behaviour in the core fixture.
