# Versioned component definitions

A component definition owns named references to reusable artifacts. For example,
`hospital.reception@1.0.0` can contain `screening` (behavior), `patients` (data),
`encounter` (state), `dispatch` (messages), and `intake` (workflow). Each reference
pins kind, ID, exact version and SHA-256 digest. These are ownership links;
explicit typed links can now be checked between their declared interfaces.
Pure behavior links can now execute in System architecture; the five execution
engines are not yet integrated. See [SYSTEM_EXECUTION.md](SYSTEM_EXECUTION.md).

Deployment remains a system design concern. Definitions cannot own deployment
plans or other definitions in this increment. Existing behavior and workflow
composition mechanisms still support nested reusable components.

## Create and inspect a reception definition

1. Open **Engineering → Components → Data**. Create a data contract, or explicitly
   load a reference example from File. Set its identity/version, validate it and
   install it. New projects and component definitions remain empty by default.
2. Repeat in **Interface & behavior**, **State**, **Messages** and **Workflows** for
   any aspects you need. Use a composite behavior package for editable behavior;
   the older timing profiles have no behavior editor. You can start with just
   one installed aspect and add others later.
3. Open **Engineering → Components → Component definitions**. Click **Component
   identity**. Set Name to Reception, ID to `hospital.reception`, Version to
   `1.0.0`, and describe its boundary.
4. Click **Refresh library**. Search the left-side installed-aspect library.
   Click **Add aspect** for each exact artifact version you want to own.
5. Click an aspect card. In its canvas inspector, give it a meaningful, unique
   slot such as `patients` or `intake`. The inspector shows its kind, version and
   pinned digest. A slot is a local ownership name, not an executable connection.
6. Click **Check compatibility**. Click any issue to focus its card or component identity.
   Missing versions or changed content must be resolved before installation.
7. Click **Install version**. The manifest is stored in PostgreSQL. Reinstalling
   identical content is idempotent; changing content under the same ID/version
   returns a conflict. **Installed versions** lets you reopen a version.
8. Select an aspect card and click **Open aspect editor**. The exact installed
   artifact opens as a draft in its existing editor. A banner identifies the
   source component, version and slot, and offers **Return to component definition**.
   Existing unsaved-draft replacement checks still apply. A cancelled replacement
   does not claim that the new aspect was opened.
9. To change an aspect, edit it, give the artifact a new version, validate and
   install it. Return to the definition, refresh the library, select the aspect
   card and use **Replace with an installed artifact** to choose the new pin.
10. Change the component definition version (for example, to `1.1.0`) and install.
    Systems using `1.0.0` keep their previous pins. A definition edit does not
    silently upgrade existing system instances.
11. Open **Engineering → Design → System architecture**. Refresh its reference
    library, find the installed definition and add it to the design. **Resolve
    references** verifies the manifest and expands its owned aspect references.
    A design can be saved before it resolves; unresolved references block runs.
12. Use **Product blueprint → Implementation sequence** to inspect remaining
    joint-execution and release work. Static system compatibility is described in
    [SYSTEM_CONNECTIONS.md](SYSTEM_CONNECTIONS.md). The conformance runner still
    accepts only the bounded built-in reception experiment; this manifest does
    not make arbitrary systems executable or deployable.

## Inspect compatibility and connect aspects

This exercise uses the synthetic Health record access behavior to learn the UI.
It is a type-checking exercise, not a recommended access-control architecture.

1. In **Interface & behavior**, load **Health record access** from the reference
   examples and install its bundle (including its policy and same-tenant dependencies).
2. In **Component definitions**, create an empty definition, set its identity,
   refresh the library, and add Health record access twice. Rename the aspect slots
   `first` and `second` in their card inspectors.
3. Click **Check compatibility**, then **Structure → Connections**. Expand the
   dependency table. The policy and same-tenant descendants are visible with exact
   versions/digests and occurrence paths. They do not need separate aspect slots.
4. Choose source `first / outputs / permitted` and target
   `second / inputs / active`. Both are boolean. Click **Add link**, then
   **Check compatibility**. The declared link passes static type checking.
5. Add another link from the same output to `second / inputs / role` (string).
   Check again: this link is highlighted as invalid. Click its issue to open
   Connections, then remove the invalid link.
6. Try a second producer for `second / inputs / active`. Validation rejects it:
   merge/arbitration semantics are not implemented. Remove the duplicate.
7. Check again, then install the definition. Export/import preserves its links.
   To change installed content, use a new definition version as before.

### What the check guarantees

- Every aspect root resolves to its exact installed artifact.
- Composite behavior dependencies are followed through the installed catalog;
  the existing bundle validator verifies the full closure and rejects missing,
  changed, cyclic or invalid dependencies.
- State, messages and workflows use their validated embedded dependency bundles.
  Their embedded locks need not also exist as separate catalog installations.
- The report includes data locks, behavior bundles and state-machine dependencies.
  The same kind/ID/version cannot have different content digests anywhere inside
  one definition. Different explicit versions remain distinct identities.
- Each declared link uses an existing output and input. Scalar types must match.
  Records and collections additionally require the same contract ID, version,
  digest and record name. No implicit conversions or structural subtyping occur.
- One target input can have at most one declared producer. Optional links are
  included in the immutable manifest digest; older link-free hashes are unchanged.
- Installation and subsequent system resolution apply these checks too. They are
  not merely a frontend warning. A formerly installed definition whose dependency
  disappears or becomes inconsistent will no longer resolve successfully.

| Aspect | Declared interfaces |
| --- | --- |
| Composite behavior | `inputs`, `outputs` |
| Workflow | `inputs`, `outputs` |
| State | `command/<name>/inputs`, `command/<name>/outputs`, `effect/<name>` |
| Messages | State interfaces prefixed by `instance/<id>/` |
| Data, timing profiles | No executable ports; data supplies nominal types |

Limits: checks cover declared manifest links, not every unconnected required
input, trigger ordering, task response bindings, cycles between aspects, delivery
semantics or provider capabilities. System-instance connections are checked
separately in System architecture. This static report is not an execution plan; preparing pure execution separately
checks supported tasks, required inputs and cycles. Component definitions still cannot be nested. Reports are bounded
at 4096 dependency occurrences and manifests at 128 links.

## Saving and portability

The current definition draft uses browser IndexedDB autosave and recovery, separate
from installed PostgreSQL versions. Export/import transfers the definition JSON,
not its artifacts. Install the exact dependencies in the receiving studio first.
An invalid import preserves the current draft. Import does not install or execute
code. SHA-256 pins detect changed content; they are not publisher signatures.

## API

All routes are relative to `/api/systems`:

| Route | Method | Purpose |
| --- | --- | --- |
| `/definitions` | GET | List installed definitions and digests |
| `/definitions` | POST | Install immutable definition; 409 on conflicting content |
| `/definitions/validate` | POST | Validate pins, closures and typed links; return ports and dependencies |
| `/definitions/aspect` | POST | Resolve one exact aspect and return its editor payload |
| `/catalog` | GET | Include definitions alongside installed artifact kinds |
| `/resolve` | POST | Resolve a system and expand owned aspects |

A system component selects a manifest with
`definition: {kind: "definition", id: "hospital.reception", version: "1.0.0"}`.
Omitting kind preserves the existing component-package reference behavior.
Definitions allow 1–32 aspects; the local catalog allows 200 definition versions.

## Implementation percentage policy

The product blueprint and footer display **roadmap v1 progress: 28%**, up from
**23%** after completing static system compatibility. The original reconstructed
baseline was 18%. M1 is 80%; M2 is 40%; M3–M6 are 0%.
Weights are 25%, 20%, 20%, 15%, 10%, 10%, respectively. Each milestone has five
binary acceptance checkpoints, equally weighted within that milestone.

This estimates accepted scope on the current roadmap. It is not measured effort,
test coverage, production readiness or a prediction of delivery time. Existing
standalone editors are foundations; menus do not earn completion credit. Security,
provenance, general execution and autonomous runtime gates remain incomplete.

The earlier manifest-only increment kept the score at 23%. The completed
system-wide static checks now earn **+5 percentage points**. See
[SYSTEM_CONNECTIONS.md](SYSTEM_CONNECTIONS.md) for the acceptance exercise and limits.
Joint execution remains a separate unfinished checkpoint.

For every implementation increment:

1. Update the relevant capability scope in `web/src/studioBlueprint.ts`.
2. Accept checkpoints only with evidence; otherwise leave the percentage unchanged.
3. Update `web/src/implementationProgress.ts` and append a dated history entry.
4. If scope or weights change, create an explicit new baseline instead of silently
   inflating the score.
5. Run tests and `node --experimental-strip-types scripts/export-blueprint.mjs`.
6. Report the current percentage, its change, tested scope and remaining work.

## Manual UI acceptance still required

In a browser, perform steps 1–12, including an unsaved-draft cancellation, opening
an aspect into an editor that has not yet loaded, reloading a saved definition
draft, invalid import retention, and narrow-screen panel collapse. Automated tests
cover domain/API behavior and frontend types; they do not replace these UI checks.
