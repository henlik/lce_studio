# Canvas-first studio navigation

## Product navigation

The sidebar is a collapsible hierarchy. **Product blueprint** shows capabilities,
current scope, remaining work, dependencies and acceptance conditions. Its
**Implementation sequence** tab describes six milestone gates. The sidebar and
blueprint use one catalog in `web/src/studioBlueprint.ts`; the complete generated
reference is [PRODUCT_BLUEPRINT.md](PRODUCT_BLUEPRINT.md).

| Plane | Group | Destinations |
| --- | --- | --- |
| Engineering | Components | Interface & behavior, Data, State, Messages, Workflows, Component definitions |
| Engineering | Design | System architecture, Databases & brokers (supporting-services library), Inspection lab |
| Engineering | Simulation | Timing flow, Simulation requirements, Deployment simulation, Simulation results, Open simulation projects, Simulation history |
| Engineering | Implementation | Developers & AI agents, Build & sandbox (planned) |
| Engineering | Assurance & release | Contracts & conformance (opens the system Claims view), Release authority (planned) |
| Runtime | Observe & verify | Sites & resources, Observer, Runtime verification (planned) |
| Runtime | Context & adaptation | Contextualisation, Recovery & optimization, Orchestration, Updates & recovery, Disconnected federation (planned) |
| Shared foundations | Reuse & governance | Installed component library, Community marketplace, Security & trust, Evidence & provenance |

Available entries open their existing editors. Planned entries and partial
capabilities without an editor open blueprint details, with no fake execution
button. Expanding a collapsed plane reveals its groups; selecting an editor
automatically opens its parent group. On mobile, choosing a destination closes
the sidebar. Breadcrumbs retain plane, group and workspace context.

Data, state, messages and workflow editors retain their own saved artifacts and
draft recovery. Component definitions now link their exact installed versions and open pinned
aspects with source-component context and a return button. See
[COMPONENT_DEFINITIONS.md](COMPONENT_DEFINITIONS.md) for the walkthrough. The system-reference editor and the existing
application/deployment simulator retain distinct models under Design and Simulation respectively.
Deployment planning is an engineering simulation, not a live runtime control.

**Design** opens System architecture directly; its separate disclosure arrow expands
or collapses navigation. New sessions start there; recovered sessions keep their saved
workspace. The global **Installed component library** opens this same design library.

**System architecture → Architecture** includes a collapsible design library on the
left with Components, Supporting services, and Actors & workload sections. Components
shows installed definitions and reusable behavior packages; kernel metadata keeps
timing profiles out of this list. Supporting services exposes offline state providers
and identifies fixed conformance adapters. Actors links to scenario inputs and clearly
labels the fixed intake client. General identity actors and live service adapters
are not implied. Advanced artifact attachments remain in a collapsed section.
Timing profiles open the separate simulator, where a visible return button leads
back to system design. **Check connections** checks catalog identities, transitive dependency integrity
and declared port types. Invalid edges are highlighted and issue buttons open
their inspector. Untyped legacy edges stay explicitly unchecked. See
[SYSTEM_CONNECTIONS.md](SYSTEM_CONNECTIONS.md) for the reception-to-consultation
exercise. Structural validation, static compatibility and execution support are
separate checks.

System architecture also includes **Execution** for bounded pure and selected-entry state graphs,
with input forms, token stepping and saved snapshots. **Evidence** continues to
show the separate model-versus-implementation conformance experiment. See
[SYSTEM_EXECUTION.md](SYSTEM_EXECUTION.md).

## Workspace ribbons

The studio uses a compact ribbon above each workspace. Data retains its existing ribbon; Components, State, Messages, Workflows, project views and Inspection Lab use the shared ribbon.

## Common controls

- **File** contains new/open/import/export, installed definitions and reference examples. Project File also contains project save and simulation actions.
- **Structure** opens boundaries, owned data, requirements or deployment.
- **Behavior** opens composition, scenarios or the application design.
- **Execution** opens tests, interactive tasks, evidence and history.
- **Help** contains workspace explanations and the design hierarchy guide.
- **Contract** in Data and Components contains identity/version metadata. State and Messages keep identity and dependency disclosures below their diagrams; Workflow root settings remain in its inspector.
- **Hide commands** collapses the command row. Click any ribbon tab to reopen it. Each shared ribbon remembers this preference in the browser; project views share one preference.
- Left/right arrows, Home and End navigate ribbon tabs. File pickers and draft editors remain mounted when their commands are hidden.

## Workspace mapping

| Workspace | Main surface | Editing |
| --- | --- | --- |
| Plan | Requirements and acceptance criteria | Structure → Requirements → Add requirement |
| Timing flow | Simulation canvas | Left library, selected-element inspector, collapsible panels |
| Deployment simulation | Geographic/resource canvas | Resource library and site/host/link/placement inspector |
| Experiments | Simulation evidence | Execution, run history and replay controls |
| Components | Typed interface or composition canvas | Structure for ports; Behavior for component composition; Contract for metadata |
| Data | Records/entities/relationships canvas | Click cards or connections for local settings |
| State | Ownership/command diagram | Click a state slot, command or effect to open settings inside the canvas |
| Messages | Owner/route diagram | Click an instance or route to open settings inside the canvas |
| Workflows | Stage overview or focused workflow canvas | Stage navigation, hierarchy and selected-step inspector |
| Inspection Lab | Use-case exercises and evidence | Inspect ribbon filters/runs; File exports; manual checklist below |

## State authoring

Open **State → Structure**. Use **Owned state**, **Effects**, or **Commands** to open the corresponding settings, then add an element. Existing diagram elements open their settings directly. Settings scroll independently of the diagram and remain available in expanded-canvas mode. Close them with × or Escape while focused inside settings. Identity and exact data contracts are in disclosures beneath the canvas.

## Message authoring

Open **Messages → Structure**. Expand **Owner machine dependencies** below the diagram to import or embed exact state machines. Open **Instances** to add owners and **Routes** to add their typed connections. Select an existing instance or route on the diagram to configure it in place. Broken/unresolved routes remain editable through **Routes** even if they cannot be drawn.

Validation shortcuts still open the affected editor. Changes retain the original exact-lock, import/export, draft recovery and simulation semantics. These UI changes do not introduce new execution engines or live deployment.

The state handoff exercise adds offline execution providers under System settings,
separate from Deployment resources. See [STATE_SYSTEM_EXECUTION.md](STATE_SYSTEM_EXECUTION.md).

In System architecture, a selected component inspector exposes **State graph entry
commands**. Execution scenarios offer **Add owner command** and a multi-command
reference fixture; see [MULTI_COMMAND_SYSTEMS.md](MULTI_COMMAND_SYSTEMS.md).
