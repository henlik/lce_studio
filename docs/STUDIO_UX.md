# Visual authoring and recovery (0.8.1)

This release changes the component-authoring workflow and adds browser draft recovery across the studio. Existing exact definitions remain valid: the new `color` and `designLevel` fields are optional. Adding either field changes a definition's digest; publish a new version when changing an installed definition.

## Build a component with the palette

1. Open **Components → New definition**. Define its identity and use **Typed interface** to add public inputs and outputs. New definitions remain empty.
2. Open **Composition**. Search the **Component library** on the left. Expand **Primitives**, **Components**, or **Systems**. Cards identify built-in examples and workspace definitions.
3. Drag a card onto the canvas, or select its **+ Add** button. The inserted node is selected and its inspector opens on the right.
4. Give the node a useful local ID. Under **Required inputs**, bind each port to a compatible public input, a node output, or a typed literal. Missing bindings are visibly marked; adding a node no longer silently supplies false/zero literals.
5. Bind each public output in the inspector. Automatic typed routing stays in place: changing a binding draws its wire and restricts available sources to compatible types. Moving a node does not change what it computes.
6. Drag nodes to arrange them, choose a node color, use **Move earlier / Move later**, or choose **Auto-arrange** to lay them out by dependency. Order changes affect presentation; dependencies determine execution.
7. Select **Validate composition**. Invalid nodes and required inputs are highlighted. Use **Fix →** beside an issue to select its node or open the relevant editor, then validate again.
8. Use **Evaluate & test** to supply inputs and independent expected outputs. Publish a tested version using **Install bundle**, or **Reuse & sharing → Publish to this studio**. Export its bundle for reuse in another studio.

**Hide inspector**, **Hide library**, the main navigation collapse control, and **Expand** on the graph provide more drawing space. The layout follows the available workspace width: on wide screens the library sits left of the canvas and the inspector sits on the right; at intermediate widths the inspector moves below the canvas, and in narrow workspaces both panels stack beneath it. Required fields remain accessible. In the application designer, narrow screens start with both panels collapsed; their toolbar buttons open one panel at a time. Choosing a workspace closes the expanded phone navigation.

## Visual conventions

| Element | Convention |
| --- | --- |
| Primitive | Beveled shape and diamond icon |
| Reusable component | Rounded rectangle and component icon |
| Pure system composition | Double border, system icon, and Systems catalog group |
| Input pin | Blue |
| Output pin | Amber |
| Feedback edge | Purple dashed return arrow in workflows; cyclic wire in pure components |
| Validation error | Red outline and text / error marker |

Color is supplementary: shapes, labels and error text also identify meaning. A feedback edge belongs to an actual directed cycle; drawing an ordinary wire from right to left does not make it feedback. Pure component cycles are highlighted but still fail validation. Use owned state or an explicit workflow loop for behavior over time. Workflow feedback remains bounded by the existing execution limits.

Set **Definition identity & description → Design level → System · pure composition** to classify a pure composition as a system. The classification does not add state or human tasks. Systems with those responsibilities are authored in State, Messages and Workflows. Workflow steps also have colors and presentation-order controls in their inspector; their target routes remain explicit.

## Reading workflow flow and token travel

Open **Workflows → Definition & hierarchy**. The graph starts in **Flow ↓** mode: the entry is at the top, ordinary routes move down, branches split into lanes and merges wait for their forward predecessors in the drawing. The hierarchy uses this same flow order. Purple dashed loop returns run outside the node lanes. This arrangement is a visual projection; it does not schedule parallel execution.

- **Readable size** shows legible cards in a scrollable canvas. **Fit** shows the whole graph; large workflows become an overview. **Go to start** returns to the entry. Selecting a hierarchy step brings it into view.
- Route labels show the boolean condition, command failure, or interrupt. Select a route label to jump to its target. Interrupt chips above the graph locate their targets without drawing a line from every possible waiting task.
- **Manual** shows the existing saved positions and permits dragging. **Save flow arrangement** copies the automatic coordinates into the draft for autosave/export. It changes presentation coordinates and therefore the definition digest, but not routes, bindings, node order or execution rules. Merely switching views does not edit the definition.

After **Run independent scenario**, open **Execution evidence → Token journey**. Use **Previous**, **Next**, the **Recorded visit** selector, or **Latest position**. The blue token identifies the selected recorded visit; its departure is blue, and earlier travel is green. Each repeated visit through a loop is listed separately. The status header identifies the final/current position of the recorded run, independently of the selected replay visit.

**Interactive tasks** includes the same journey for its exact running experiment. Submitting a response advances the experiment; browsing its journey only inspects evidence. Accepted emergency signals show a distinct interrupt jump, including the cancelled task. Rejected signals and replayed receipts do not invent another token movement. Earlier immutable evidence always uses its recorded workflow snapshot, even after the draft is edited.

The journey is a sequential view of the existing bounded workflow engine, not a parallel-token scheduler or wall-clock animation. It adds no clinical decisions or live integrations.

## Autosave and recovery

The studio automatically checkpoints project drafts, component definitions and their tests, data/fixture drafts, state/scenario drafts, message/scenario drafts, and workflow/scenario drafts in IndexedDB. Pending advanced JSON is included. Workflow interactive experiments and unsent task-response forms are included. The status bar displays when the last write completed.

- Reload the same studio URL in the same browser profile to restore your draft.
- Starting, importing or opening another definition archives the previous saved draft. **Recover earlier draft** restores one of these checkpoints. These are replacement checkpoints, not an undo entry for every keystroke.
- If two tabs edit the same workspace, the later stale writer stops with a conflict. **Load saved draft** adopts the saved version. **Save my draft (back up previous)** makes the current edits active while retaining the previous saved version as a checkpoint. **Export recovery copy** preserves the current full workspace snapshot as JSON.
- A storage failure leaves the previous saved record intact and reports an error. Keep the page open and export the current work before leaving. A reload cannot recover edits whose save failed.

Autosave is **local to a browser profile and origin**. It is not cross-device synchronization, a server backup, or real-time collaborative editing. `localhost:5173` and `127.0.0.1:8080` have separate stores. Clearing site data removes browser checkpoints. Continue saving projects to PostgreSQL and publishing/exporting tested definitions for durable shared use. Normal bundle imports accept component bundles; the full recovery JSON is a diagnostic workspace snapshot, not a component bundle.

## Race conditions

Two protections serve different purposes:

1. **Editor drafts:** checking the stored revision and replacing the draft happen in one IndexedDB read/write transaction. Two tabs cannot both commit against the same revision. Conflicts require an explicit resolution and do not silently merge graphs.
2. **Modeled state and workflows:** owned-state commands already use expected revisions and command identities. Competing new commands cannot both mutate the same expected revision; a stale request is rejected. Repeating an identical identity and payload replays its receipt; reusing that identity with changed data conflicts. Workflow cancellation similarly prevents a late response from resuming a cancelled task.

Test both request orders and lost-response retries in independent scenarios. The Go race detector additionally checks implementation concurrency. These experiments execute a declared sequence; they do not model arbitrary thread scheduling or provide a production distributed lock service. A generated/deployed runtime must preserve the same atomic revision/receipt contract using transactions or another suitable owner mechanism.

## Sharing now, community marketplace next

The implemented publishing destination is the current studio's PostgreSQL catalog. It stores immutable versions and validates exact dependency closures. Portable import/export allows engineers to exchange components now; importing a bundle does not execute arbitrary plugin code.

A public community marketplace is not connected in 0.8.1. Before exposing one, add authenticated publisher identities and namespace ownership, a versioned registry/search API, license and dependency metadata, integrity verification, review/reporting and revocation policies, and an explicit install/trust flow. The current grouped palette and portable packages provide the client foundation. A catalog install must not be represented as public publication.

## Verification record

The change is checked with the existing Go/model/API acceptance suites, graph-cycle unit tests and browser interactions. Manual checks cover drag/drop and button insertion, required bindings, reordering without changing connections, validation-to-inspector navigation, version publication and System reuse, reload restoration including unsent reception task responses, and stale-tab conflict resolution with a preserved checkpoint. Browser checks use synthetic definitions and patient identifiers.

The responsive layout correction was checked at viewport widths of 320, 390, 600, 768, 820, 1024, 1280 and 1440 pixels. Checks cover the component library remaining accessible, panel hide/show combinations with the main sidebar expanded, no page-level horizontal overflow, phone and desktop canvas expansion/restoration, and the application designer inspector opening at a usable width. The populated hospital reception workflow definition and task forms were also checked at 320 pixels.

Workflow flow/token checks cover deterministic branch and merge layout, loop returns, non-overlapping placement of the 57-step reception workflow, repeated task activations, exact branch selection when targets coincide, command failure versus business denial, and accepted interrupt assignments.

Browser verification includes routine reception and emergency-after-reservation independent runs, step replay and interrupt highlighting, waiting-task position, manual dragging and reload restoration, direct branch navigation, overview/readable sizing, and 320/390/768/1024-pixel layouts with phone expansion. The emergency scenario was rerun successfully after saving presentation coordinates.
