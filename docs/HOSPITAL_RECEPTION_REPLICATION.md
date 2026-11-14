# Recreate and test Hospital Reception

This guide accompanies the executable reception reference package in LCE Studio 0.8. It follows the [top-down design](HOSPITAL_RECEPTION_DESIGN.md), then rebuilds from reusable decisions and owners upward. The studio runs bounded synthetic workflow experiments. Registry, affiliation, clinical and payment services supply explicit typed responses in those experiments; the package is not connected to a live hospital database or payment provider.

## What is being built

`Hospital reception` is a reusable workflow definition with a public input/output contract. Its graph contains four executable node kinds:

| Kind | Meaning |
| --- | --- |
| Task | Pause for a human action or an explicit synthetic adapter response |
| Decision | Evaluate an exact reusable pure component and assign its outputs |
| Command | Invoke a command on an owned-state component and follow its accepted/denied result |
| End | Complete with a named outcome and typed public outputs |

Routes use either a direct next node or a Boolean-variable branch. An interrupt signal can cancel the current task and route to emergency handling. These routes execute: engineers do not need to select the next diagram edge manually. A task awaiting a response is a legitimate paused result, not an execution failure or a completed reception.

Definitions include their exact data, pure component and state-machine dependencies. Installing or exporting the workflow preserves those locks. A dependency can also be exported/imported separately and reused in another design. The workflow workbench's hierarchy lets you inspect a decision down to its primitive nodes.

## 1. Start the studio

From the repository root:

```sh
docker compose up --build
```

Open `http://127.0.0.1:8080`, then select **Workflows** in the left navigation. New work starts empty; load the hospital reception example explicitly. Use only synthetic patient identifiers and observations in the reference experiments.

If you already have the development environment configured, the existing Vue/Go development command also works:

```sh
npm run dev
```

The [README](../README.md) describes PostgreSQL setup and the development environment.

## 2. Run the supplied baseline before editing

Select **Routine existing patient** in **Reference experiment**, click **Open workflow example**, then **Run independent scenario**. Open **Execution evidence** to see its verdict. Inspect:

1. The ordered automatic decisions and owned commands between tasks.
2. The registry response, affiliation disposition and encounter reference.
3. The observation/clinical disposition and payment-policy result.
4. The proposed doctor, committed reservation and receiving acceptance.
5. The final outcome and independently supplied expectations.

Receiving acceptance is distinct from having requested an assignment. Queue, encounter and reservation operations run through the existing owned-state kernel. Human and adapter responses are recorded as typed scenario events.

Repeat with the new-patient, no-affiliation, fever-review/release, early-emergency, post-reservation-emergency, no-doctor and unavailable-service examples. Read a waiting result as a visible task that still needs an action. Read a failed expectation as evidence that the declared result was not obtained.

For a database-free command-line reproduction:

```sh
go run ./cmd/workflowcheck --run examples/reception/scenarios/routine-existing.run.json
```

The run file contains both the exact workflow lock and its scenario. It does not fetch mutable definitions from a remote catalog. Other scenario files under `examples/reception/scenarios/` use the same reception definition with different explicit responses and expectations.

## 3. Follow one reception interactively

Open **Interactive tasks**, then click **Start interactive workflow** to begin with public inputs and no prefilled event script. The engine advances to the first task. Read **Request values**, supply the typed response, and click **Submit response & continue**. The engine then executes automatic steps until the next task or an end node.

For the normal route, follow these responsibilities:

| Stage | What the response represents |
| --- | --- |
| Initial screening | Professionally supplied emergency/ordinary disposition |
| Queue call | Clerk records the call and attendance |
| Identity lookup | Synthetic registry completion and match information |
| Registration or confirmation | Accepted registration or confirmed existing identity |
| Affiliation | Verified, confirmed-none or unresolved evidence |
| Vitals | Recorded observation status and assessment reference |
| Clinical review | Current professional release, continued review or escalation |
| Payment policy | Explicit allow, defer, waive or review disposition |
| Dispatcher | Eligible candidate selection or continued waiting |
| Receiving doctor | Actual modeled receiving acceptance |

For an exact manual replay of the ordinary synthetic case, leave the example root inputs unchanged (`case-100`, `arrival-100`, `hospital-alpha`, `outpatient`, `synthetic-id-100`, capacities `1`, queue start `100`). At each response enter `synthetic-actor` as `actorRef` and a distinct `fixture-N` as `evidenceRef`. Fill the remaining fields as follows:

| Task | Response fields |
| --- | --- |
| Initial screening | `assessmentRecorded=true`, `emergencyIdentified=false`, `assessmentRef=screen-100` |
| Call ticket | `ready=true`; the request displays ticket `100` |
| Identity registry | `completed=true`, `matchCount=1`, `patientId=patient-42` |
| Confirm identity | `confirmed=true`, `patientId=patient-42` |
| Affiliation | `status=verified` |
| Observations | `observationsAccepted=true`, `feverAlert=false`, `observationRef=observations-100`, `assessmentRef=assessment-100` |
| Clinical review | `disposition=released`, `authorized=true`, `assessmentRef=assessment-100` |
| Payment policy | `disposition=allow` |
| Workforce | For both candidate records set `facilityMatches`, `serviceMatches`, `qualified`, `onDuty` to `true` and `available=1`; use `resourceId=providerA` for candidateA and `providerB` for candidateB |
| Dispatcher | `choice=candidateA` |
| Receiving physician | `accepted=true`, `resourceId=providerA`, `caseId=case-100`, `reference=consultation-acceptance-100` |

The result should be `completed / consultationAccepted`, with 11 completed tasks, ticket 100, one encounter and a held, accepted providerA slot. **Save interactive evidence** preserves the trace as unverified. **Copy events to scenario draft** lets you author independent assertions; use the supplied baseline as an oracle instead of converting the observed result into its own expected result.

Do not treat a default form value as a clinical recommendation. The reference inputs are synthetic protocol facts. Clinical assessment, eligibility evidence and financial policy must be supplied by their responsible real-world services in a future deployed implementation.

Interactive continuation is a reproducible finite experiment. It retains the supplied event sequence and reruns that exact sequence from its declared initialization. Saved runs retain the exact workflow, scenario and evidence; they are not live patient sessions. Starting another experiment initializes independent owners.

## 4. Exercise the emergency interrupt

Start a normal interactive reception and stop at an active task. Under **Interrupt / emergency signal**, select `escalateEmergency`, enter synthetic actor and assessment references, then click **Send escalateEmergency**. Verify that the current routine task is cancelled and that emergency handling becomes active without requiring ordinary identity, affiliation or payment completion.

Repeat after doctor reservation. The reference design must release the routine reservation before waiting for the emergency receiving-unit acceptance. If the receiving unit cannot accept, the outcome remains an explicit pending escalation. No bed, admission or clinical acceptance is invented.

An old task response retains its original task ID. Submitting it after cancellation must be rejected rather than allowed to resume the old route. Repeating an identical event ID/payload replays its historical receipt without performing the transition again. Changing the payload under that ID is a conflict. Inspect these outcomes in the event evidence.

## 5. Import or copy the whole component

The portable files are:

- [Raw workflow definition](../examples/reception/hospital-reception.workflow.json).
- [Exact workflow lock](../examples/reception/hospital-reception.lock.json).
- [Existing-patient run](../examples/reception/scenarios/routine-existing.run.json).
- Reusable decisions under `examples/reception/decisions/`.
- Owned-state component locks under `examples/reception/owners/`.
- Data contracts under `examples/reception/data/`.

Click **Import workflow** and select the raw definition or exact lock. To change only a scenario, use **Scenario & expectations → Import workflow scenario**. Run files wrap both objects for the CLI; select their `scenario` object when editing scenario JSON in the workbench. A raw definition is editable; an exact lock must first validate against its declared digest. Changing an installed version requires a new version or definition ID. Do not edit the digest to make altered content look like the original definition.

Keep a new definition ID for your own independently maintained version, for example `myclinic.reception`. Keep hospital/facility IDs, staff resources and scenario facts in instance inputs/configuration. Do not rename every generic dependency to a hospital-specific name.

## 6. Rebuild the smallest decisions

Open each exported decision in **Components**. Review its typed interface, Composition graph and test cases.

In 0.8.1, add parts from the **Component library** on the left: drag a card onto the canvas or click **+ Add**. Select the placed node to configure its **Required inputs** in the right inspector. Missing bindings stay empty and are marked explicitly. Bind public outputs below the node inspector. **Validate composition → Fix →** brings you back to an invalid node. Drafts restore after reload in the same browser and URL; see [the authoring and recovery guide](STUDIO_UX.md).

The reference package includes small decisions for:

- Successful, unavailable and ambiguous identity lookup.
- Verified, confirmed-none and unresolved affiliation.
- Current professional review disposition.
- Payment-policy disposition.
- Candidate eligibility.
- Dispatcher choice.

The earlier [EntryRoute bundle](../examples/composites/reception-entry-route.json) remains a small independent exercise. Its two Select nodes, Not node and And node are described in the [design guide](HOSPITAL_RECEPTION_DESIGN.md#8-author-the-first-pure-component-entryroute).

For every decision, enter expected results before running it. Change one relevant input at a time: unavailable lookup must not become a new-patient instruction; unresolved affiliation must not set the no-affiliation flag; an obsolete assessment must not release the current review; zero capacity must not create an eligible candidate.

In **Definition & hierarchy**, select a decision node and choose **Inspect exact decision & primitives**; use **Inspect nested component** to descend further. Export the exact dependency for editing in Components.

Export/install a tested immutable version. Higher-level nodes reference that exact ID, version and digest. A component call is inspectable reuse, not copied handwritten logic in every workflow step.

## 7. Rebuild the owners

Open the exported owner locks in **State**. Each command has a pure decision, explicit state/input bindings, an accepted Boolean output, proposed writes and any effect intentions.

| Owner responsibility | Inspect |
| --- | --- |
| Queue | Ticket sequence and accepted issue/call behavior |
| Encounter | The explicit visit identity and duplicate-safe opening behavior |
| Assignment | Available capacity, reservation identity, acceptance and release |

Retain their public command interfaces. The workflow binds inputs and consumes outputs; it must not edit their private state directly. State revisions and receipts handle conflicting or repeated commands within the modeled owner.

The queue owner is a single-case ticket allocator: `queueStart` supplies its next number, repeated issue for that case preserves its ticket, and a different case requires another owner. Independent workflow experiments do not share a facility-wide queue. The reservation owner represents one occupied slot; modeled doctor owners are `providerA` and `providerB`.

The same queue and reservation components are reused by the service-desk reference workflow. Running that example demonstrates reuse outside hospital reception without changing the owner's implementation.

## 8. Rebuild the workflow from an empty definition

Click **New workflow**, open **Definition & hierarchy**, and define its name, ID/version and public inputs/outputs. Use the supplied raw definition as the complete field-by-field reference rather than inventing additional schema fields.

1. Add variables for the facts shared between steps, with exact types and explicit initial values.
2. Embed the tested pure bundles, state-machine locks and data locks.
3. Create owner instances, binding their initial configuration to workflow inputs or literals.
4. Add the human/adapter tasks and declare each response's typed ports.
5. Map task response fields into workflow variables.
6. Add decision nodes; map their input ports from workflow inputs, variables or literals, then map results back into variables.
7. Add command nodes for ticket issuance, encounter opening and resource reservation/release. Bind the acceptance result and error route explicitly.
8. Connect direct routes or Boolean branches. Add end nodes with named outcomes and complete public output mappings.
9. Add the emergency signal and its route/variable updates.
10. Validate the complete definition, then run independent scenarios before installing a version.

The API also supports reproducible authoring:

```text
POST /api/workflow/prepare         raw definition → validated exact lock
POST /api/workflow/validate-lock   exact lock → validity and canonical lock
POST /api/workflow/check           {workflow, scenario} → preview evidence
POST /api/workflow/runs            {workflow, scenario} → saved evidence
POST /api/workflow/packages        exact lock → installed immutable version
```

The [task workflow contract](TASK_WORKFLOW_CONTRACT.md) defines every field, binding direction and execution bound. Unknown fields, unresolved references and incompatible bindings must be fixed before execution. JSON import is an explicit authoring path for detailed definitions; it does not bypass those checks.

## 9. Write independent scenarios

A scenario declares workflow inputs and ordered response/signal events. Each response names the specific task activation ID, not merely the diagram node's name. Revisiting a task creates a new activation. Give each business event a stable ID for testing duplicate delivery.

An expectation can check the terminal/waiting status, outcome, active task node, selected variables, owner state and task/event counts. Leave expectations absent only when exploring; that result is unverified, not passing.

Minimum regression set:

| Case | Required outcome |
| --- | --- |
| Ordinary existing patient | Consultation handoff only after receiving acceptance |
| New patient | Explicit successful registration before normal encounter continuation |
| No affiliation | Confirmed-none flag, followed by configured payment disposition |
| Registry unavailable/ambiguous | Identity remains unresolved; no silent registration or arbitrary match |
| Affiliation unknown | Review, not confirmed absence |
| Fever review pending | Routine dispatch remains blocked until a current professional disposition |
| Emergency at arrival or after reservation | Routine task cancelled, any reservation released, emergency acceptance still explicit |
| No eligible/available doctor | Waiting/reselection, not a fabricated assignment |
| Late or repeated task event | Rejection or historical replay, with no duplicate mutation |
| Wrong expected outcome/count | A saved failed verdict, not automatically rewritten expectations |

## 10. Keep the execution boundary visible

This increment executes workflow routing, typed tasks, exact pure decisions and modeled owner commands. It does not enforce real staff identities, connect to a patient database, prescribe fever treatment, charge money, provide a message broker or couple workflow execution to the hardware/network/power simulator. Those require explicit runtime adapters and further integration contracts.

The package is reusable as a versioned workflow with different inputs and scenarios. Pure definitions support recursive nesting; workflows can inspect and reuse those definitions and owned-state components. Nested workflow calls, a common project/system containment tree and generated deployed applications are separate capabilities, not implied by the hierarchy display.

## 11. Regenerate the portable fixtures

The source of the reference graph and handwritten scenario oracles lives in `internal/model/reception_*.go`. After an intentional reference-design change, regenerate exports with:

```sh
go run ./cmd/receptionexport
go run ./cmd/workflowcheck --lock examples/reception/hospital-reception.lock.json
go run ./cmd/workflowcheck --run examples/reception/scenarios/routine-existing.run.json
go run ./cmd/workflowcheck --run examples/reception/scenarios/service-desk.run.json
```

The exporter serializes declared expectations; it does not run the workflow to manufacture them. Run `go test ./internal/model ./internal/workflow` and the live `scripts/acceptance-reception.mjs` suite after editing. The service-desk example reuses exact ticket and slot-owner locks in a nonclinical workflow.
