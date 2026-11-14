# Versioned component definitions

A component definition owns named references to reusable artifacts. For example,
`hospital.reception@1.0.0` can contain `screening` (behavior), `patients` (data),
`encounter` (state), `dispatch` (messages), and `intake` (workflow). Each reference
pins kind, ID, exact version and SHA-256 digest. These are ownership links;
adding them does not yet wire the five execution engines together.

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
6. Click **Validate pins**. Click any issue to focus its card or component identity.
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
    compatibility, joint-execution and release work. The conformance runner still
    accepts only the bounded built-in reception experiment; this manifest does
    not make arbitrary systems executable or deployable.

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
| `/definitions/validate` | POST | Validate structure and resolve pinned aspects |
| `/definitions/aspect` | POST | Resolve one exact aspect and return its editor payload |
| `/catalog` | GET | Include definitions alongside installed artifact kinds |
| `/resolve` | POST | Resolve a system and expand owned aspects |

A system component selects a manifest with
`definition: {kind: "definition", id: "hospital.reception", version: "1.0.0"}`.
Omitting kind preserves the existing component-package reference behavior.
Definitions allow 1–32 aspects; the local catalog allows 200 definition versions.

## Implementation percentage policy

The product blueprint and footer display **roadmap v1 progress: 23%**, up from
an explicitly reconstructed **18% baseline**. M1 is 60%; M2 is 40%; M3–M6 are 0%.
Weights are 25%, 20%, 20%, 15%, 10%, 10%, respectively. Each milestone has five
binary acceptance checkpoints, equally weighted within that milestone.

This estimates accepted scope on the current roadmap. It is not measured effort,
test coverage, production readiness or a prediction of delivery time. Existing
standalone editors are foundations; menus do not earn completion credit. Security,
provenance, general execution and autonomous runtime gates remain incomplete.

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
