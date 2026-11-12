# Portable project model

LCE Studio models software independently from its deployment. Healthcare, payments, and other domains are reference projects or future component packages; none is a built-in simulator type.

## Supported schema versions

| Version | Supported content | Execution |
| --- | --- | --- |
| `1` | Requirements, client/service/database pipeline, workload | Original v0.1 timing and retry semantics |
| `2` | The same software model plus explicit infrastructure and placements | CPU sharing, memory admission, physical-link transfer costs, outages and host energy |

Version 1 must not include infrastructure or positive memory reservations, since its engine does not interpret them. Version 2 requires an `infrastructure` object. Unset new optional fields are omitted from JSON serialization so legacy project snapshots retain their previous serialized shape and model hashes.

Both versions currently execute one directed client → service(s) → database pipeline. Version 2 does not yet add arbitrary application topology, branching, executable third-party plugins, real deployment, batteries, migration, or multihop network routing.

The authoritative implementation contract is [CONTINUUM_CONTRACT.md](CONTINUUM_CONTRACT.md), and execution details belong to [SIMULATION.md](SIMULATION.md).

## Infrastructure entities

| Entity | Definition | Meaning |
| --- | --- | --- |
| Site | `id`, `name`, `latitude`, `longitude` | A named geographical location |
| Host | `id`, `name`, `siteId`, `tier`, `cpuCores`, `memoryMb`, `idleWatts`, `activeWatts`, `outages` | A powered computing resource at a site |
| Network link | `id`, `sourceHostId`, `targetHostId`, `latencyMs`, `bandwidthKbps`, `outages` | One direct undirected connection between hosts |
| Placement | `nodeId`, `hostId` | Assignment of one software node to a computing resource |
| Outage | `startMs`, `endMs` | Unavailability on the half-open interval `[startMs, endMs)` |
| Infrastructure | `sites`, `hosts`, `links`, `placements`, `messageBytes` | The deployment environment and fixed payload size for link crossings |

Sites, hosts, and links have unique IDs across their infrastructure namespace and cannot reuse application node IDs. Application edges, requirements, and their original identifiers remain unchanged. Host tiers are `device`, `edge`, `regional`, or `cloud`; neither tier nor geographic distance automatically generates network latency or processing speed.

Arrays are explicit, including empty arrays: use `outages: []` for an always-powered host or always-available link. An omitted or null infrastructure collection is invalid. All quantities have explicit units: integer milliseconds, MiB of memory, bytes of payload, decimal kbit/s of bandwidth, watts of power, and watt-hours of estimated energy.

## Drafts and executable placements

Saving and importing a draft validates bounded values, identities, and references, while allowing incomplete software topology or missing placements. This lets an engineer save work before every application has been placed or all resource requirements have been satisfied.

Simulation additionally requires:

- Exactly one placement for every application node, including the client.
- A direct physical link for every application edge whose endpoints are placed on different hosts. A same-host edge needs no physical link. Indirect paths are not inferred.
- Sufficient host memory for the sum of all resident application reservations. `node.config.memoryMb` omitted or zero means **64 MiB** in version 2.
- The existing complete linear application topology and all workload constraints.

Memory is reserved statically; it is not a dynamic allocator or an out-of-memory model. Node `capacity` limits processing concurrency, while colocated nodes share host CPU cores. A total of node capacities greater than the host core count is permitted so CPU contention can be simulated.

Duplicate placements, self links, or duplicate unordered host pairs are invalid even in drafts. Deleting a referenced object requires updating or removing its dependent objects; dangling references are not silently repaired by the backend.

## Bounds

| Property | Limit |
| --- | --- |
| Sites / hosts / placements | 30 of each |
| Physical links | 60 |
| Outage windows | 20 per host or link |
| Latitude / longitude | Finite `[-90,90]` / `[-180,180]` |
| Host CPU cores | Integer `1..128` |
| Host memory | Integer `1..1048576` MiB |
| Node memory | Integer `0..1048576` MiB; zero means the default |
| Idle / active power | Finite `0..1000000` W; active must be at least idle |
| Link latency | Integer `0..60000` ms |
| Link bandwidth | Integer `1..10000000` kbit/s |
| Message size | Integer `1..10485760` bytes |
| Outage interval | `0 <= startMs < endMs <= 3600000` |

Outage windows may be supplied in any order. They cannot overlap; touching windows are allowed. Validation sorts a copy when checking overlaps and never mutates the project.

All original application and workload bounds also apply. A structurally valid experiment can still exceed the simulator's event or time budget; that condition returns an error rather than a passing partial result.

## Generic reference experiment

`GET /api/example?template=continuum` and [continuum-placement.json](../examples/continuum-placement.json) describe the same healthy baseline:

- A request client on `device-host`.
- A request service and durable store on `edge-host`.
- A separate `cloud-host` available for alternative placement.
- A local device/edge link and two explicit WAN links.
- Twelve requests, 1024-byte physical-link payloads, and bounded retries.
- No configured outages initially; both edge and cloud arrangements have valid direct connectivity.

Run the baseline, then move both `service` and `database` to `cloud-host` and run again. Compare immutable run snapshots to see the effect of physical-link delay, bandwidth, host placement, and energy profiles.

For a deliberate failing case, return the service and database to `edge-host` and add a host outage `{ "startMs": 0, "endMs": 5000 }`. This spans the requests and all their permitted retries, so successful completion is not expected. For a disconnected cloud case, use the same interval on `device-wan` after moving the application to the cloud.

The numerical hardware/power profiles are illustrative assumptions, not measured predictions. Geography supplies location metadata only. Total energy covers the reported run horizon, including configured environmental transitions and unused powered hosts; compare duration and availability as well as energy before drawing conclusions about placement efficiency.

## Run evidence

Version 2 runs retain their exact project snapshot and add `infrastructure` results:

- Total host energy in Wh.
- Per-host energy, occupied core-milliseconds, uptime, and interrupted processing-job counts.
- Per-link attempted transfers, successfully delivered payload bytes, and dropped transfer counts.

Trace events can carry `hostId` or `linkId` for environmental events. They need not invent an application `nodeId` or `requestId` when none applies. The original application metrics, verdict types, and stored-run identity fields remain unchanged.
