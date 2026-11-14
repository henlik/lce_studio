# Canvas-first studio navigation

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
| Design | Application canvas | Left library, selected-element inspector, collapsible panels |
| Deployment | Geographic/resource canvas | Resource library and site/host/link/placement inspector |
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
