# LCE Studio product roadmap

Product direction agreed in the project conversation on 12 November 2026.

## Product and reference projects

LCE Studio is an extensible visual engineering environment for designing, building, simulating, deploying and validating distributed systems across heterogeneous, resource-constrained computing environments. Engineers should be able to create new components and domain packages without changing the studio kernel.

A distributed hospital ecosystem is a reference project used to test the platform as it develops. It is not the platform's domain model. Hospital, laboratory, pharmacy, finance and procurement concepts belong to reference projects or reusable domain packages. Additional use cases, such as logistics, payment processing, education and environmental sensing, test whether the abstractions remain generic.

Application behaviour and deployment are both first-class design inputs. The same application must be comparable across different physical resources, network layouts, geographic sites and intermittent power conditions. There is no mandatory VM, Proxmox or Kubernetes layer; actual deployment targets are selected through supported adapters and measured constraints.

## Connected project models

| View | Content |
| --- | --- |
| Requirements | Functional rules, safety/security properties, performance, availability, energy and resource budgets |
| Application | Components, interfaces, data ownership, events, states, workflows, human tasks |
| Organization | Organizations, tenants, facilities, memberships and explicit service-sharing relationships |
| Infrastructure | Compute, memory, storage, network devices and administrative domains |
| Geography | Sites, coordinates, mobility, geographic and administrative failure groups |
| Energy | Grid supply, generation, batteries, consumption, outage and recovery policies |
| Deployment | Application instances, host bindings, dependencies, allocations and data replicas |
| Operations | Scheduling, placement, synchronization, migration, upgrades and recovery policies |
| Experiment | Workload, assumptions, fault/environment timelines, seeds and assertions |
| Evidence | Model validation, simulation traces, implementation tests, measurements and calibration |

Changes in one view must have explicit consequences in the others. A host losing power can interrupt processing and communication; failed work may trigger retries that consume bandwidth, CPU and energy elsewhere. Coordinates do not by themselves imply a measured latency, radio coverage or legal permission to transfer data.

## Release capability ledger

### v0.1 — foundation (delivered)

Vue graphical application editor, Go deterministic linear-pipeline simulator, requirement verdicts, event replay, JSON import/export, project and immutable run persistence, HTTP acceptance checks and CI. PostgreSQL replaced the original prototype SQLite backend.

### v0.2 — first continuum increment

The current increment adds optional infrastructure to the supported pipeline: sites with coordinates; device/edge/regional/cloud hosts; CPU cores, static memory reservations and estimated host power; direct physical links with bandwidth/latency; scheduled host/link outages; application placements; infrastructure metrics; recorded-run comparison. Legacy projects remain usable. See CONTINUUM_CONTRACT.md, MODEL.md and SIMULATION.md for exact implemented semantics.

This increment introduced a bounded resource/deployment model. Application generation, batteries, solar, mobility, multihop routing, placement optimization and real infrastructure orchestration remain future work.

### v0.3 — declarative components and exclusive workflows

Versioned component profiles can be installed in a PostgreSQL catalog, reused as parameterized service/store/router nodes, and embedded with exact content locks in portable projects and run snapshots. The design editor supports exclusive weighted routing, multiple terminal stores and convergence without a synchronization barrier. Routing is deterministic per logical request, including retries. Infrastructure remains optional, and existing CPU/network/outage/energy models apply when it is present.

This is the first implemented subset of the extension system. Profiles reuse supported execution primitives; external code, new primitive definitions, typed arbitrary data contracts, state machines and parallel fork/join semantics remain future work. See COMPONENT_SDK.md and WORKFLOW_CONTRACT.md for the implemented boundary.

## Development milestones and acceptance gates

| Milestone | Deliverables | Acceptance evidence |
| --- | --- | --- |
| 1. Shared modelling foundation | Versioned application, infrastructure and deployment models; reference scenarios; explicit units and assumptions | The same application executes under alternative placements, with deterministic differences under resource/network/power constraints |
| 2. Component extension system | Versioned package manifest, configuration validation, catalog, SDK, compatibility rules and isolated execution protocol | An engineer adds a software or infrastructure component without changing core source; unsupported capabilities are rejected clearly |
| 3. Rich behaviour and coupled simulation | Typed ports, data contracts, branching, state machines, human tasks; resource sharing; fault and environmental models | Independent numerical references and domain invariants hold; injected faults cause expected observable failures rather than fabricated success |
| 4. Experiment workspace | Scenario composition, parameter sweeps, repeated seeds, comparisons, traceability and evidence export | Compare candidate designs reproducibly and identify which stated requirements fail under which conditions |
| 5. Application construction | Supported Go/Vue generation patterns, forms/worklists, API contracts, migrations, tests and protected custom-code extension points | Generated apps run independently; regeneration preserves custom logic; implementation tests agree with supported model semantics |
| 6. Deployment/runtime adapters | Inventory, secure domain agents, resource reservations, configuration, secrets, artifacts, upgrades and recovery | Deploy a project to separate domains; test safe restart and rollback; central control loss does not silently erase local work |
| 7. Calibration | Resource benchmarks, measured parameter profiles, predicted-versus-observed comparison and uncertainty reporting | Record known prediction error for defined workloads; clearly label uncalibrated parameters and unsupported phenomena |
| 8. Reference systems and packages | Growing healthcare reference ecosystem plus at least one contrasting domain | Reuse generic components across cases; organization/tenant/service/location boundaries remain explicit |
| 9. Collaboration | Authenticated studio workspaces, reviews, Git integration, model diffs, private/shared catalogs, package provenance and version locking | Engineers can review, reproduce and evolve models and generated artifacts with traceable changes |
| 10. Supported product | Installation, guided onboarding, documentation, extension publishing, migration and upgrade procedures, operational support tooling | An independent engineering team builds, tests and maintains a new supported system using the product documentation |

Security, testing, documentation and usability progress throughout the milestones. No milestone claims that simulation alone proves correctness of arbitrary software or authorizes clinical deployment. Calendar estimates must be revisited once the required simulation fidelity, deployment targets and staffing are fixed.

The next increment should connect component ports to typed data contracts and explicit workflow state, then add bounded parallel fork/join with independent correctness cases. An external execution adapter follows only once the isolation, cancellation and reproducibility contract has executable conformance tests. Application generation should start from this documented subset rather than implying that every diagram can already become a runnable system.

## Reference project strategy

Each platform milestone adds a small scenario with an independently specified outcome. The hospital example grows incrementally: cross-organization orders, independent service providers, a longitudinal record view, policy-scoped sharing, local work while disconnected, and reconciliation. A second small use case must appear before the core API is considered stable so healthcare assumptions cannot become hidden kernel requirements.

Important reference invariants include tenant isolation, immutable evidence, preservation of confirmed local writes within a stated failure model, bounded offline authorization, and avoidance of repeated business effects during retry. Globally exclusive operations during partitions require explicit ownership, coordination or restricted offline authority. Backup/replication recovery objectives must distinguish a process restart from destruction of an isolated site.

## Simulation fidelity

1. Behaviour models represent states, messages, decisions, permissions and recovery under explicit semantics.
2. Resource models estimate time, contention, bandwidth, memory, storage and energy using supplied demand/capacity profiles.
3. Execution-based tests run actual applications with controlled limits and injected failures.
4. Calibration compares model predictions with measured executions; it does not turn an unsupported phenomenon into a supported one.

Every future result package should retain model/package versions, input parameters and their sources, execution environment, seeds, assumptions and calibration evidence. Proposed specialist engine adapters need a causally correct clock/step protocol, tested unit conversion and a published capability matrix. Their outputs must not be mixed as if they shared fidelity automatically.

## Initial research references

- [SimGrid resource models](https://simgrid.org/doc/latest/Models.html): CPU, network and disk performance models and calibration limits.
- [SimGrid plugins](https://simgrid.org/doc/latest/Plugins.html): host/link energy, batteries and solar examples.
- [SimGrid calibration](https://simgrid.org/doc/latest/Calibrating_the_models.html): validation against real execution.
- [iFogSim](https://github.com/Cloudslab/iFogSim): mobility, orchestration and clustering reference work.

These are evaluation references, not dependencies of the current Go simulator or commitments to replace it.
