// Requires Node 22+ with --experimental-strip-types; no third-party dependencies.
import {writeFileSync} from 'node:fs';
import {blueprint, milestones, capabilityLocation, statusLabels} from '../web/src/studioBlueprint.ts';

import {roadmapProgress, progressHistory, progressMethod} from '../web/src/implementationProgress.ts';

let doc = `# LCE Studio implementation blueprint

The studio separates engineering authority from deployed execution. This is the
target product structure and a scoped inventory of the current implementation.
It does not claim that planned capabilities exist because they have menu entries.

Generated from [studioBlueprint.ts](../web/src/studioBlueprint.ts), the same
catalog used by the application navigation. Update that catalog as capabilities
change, then run \`node --experimental-strip-types scripts/export-blueprint.mjs\`.

## Status meanings

- **Available:** usable within the expressly stated scope.
- **Partial:** an implemented foundation with integration or semantics still missing.
- **Planned:** no working control is provided; the menu opens its blueprint details.

Counts in the studio are capability counts, not completion percentages. A status
change requires evidence against the acceptance condition. Developers maintain
these statuses in source control; test results do not automatically promote them.

## Authority and ownership

**Engineering plane:** engineers author reusable components, assemble system
designs, declare deployment environments, run experiments, implement software in
a sandbox, verify conformance and authorize releases. Deployment planning is an
engineering activity; it does not deploy software.

**Runtime plane:** deployed observers and controllers will check claims, interpret
context, propose adaptations and execute only actions permitted by engineering
authority. Runtime controllers must not change their own acceptance criteria,
security policies or release authority. These runtime services are planned.

**Shared foundations:** catalogs, security, trust, evidence and provenance support
both planes. Security work starts with the engineering model, before autonomous
runtime execution is introduced.

Component Data, State, Messages and Workflows are now grouped under Components.
Their artifacts remain independently versioned. Component definitions now own
named aspect pins and open exact artifacts in their editors with return context.
Definitions inspect dependency closures and check declared aspect-link types.
System-wide typed connections and dependency closures are now checked.
Bounded pure and selected-entry state graphs execute with deterministic token traces
and exact offline modeled-state provider bindings. Shared graph scenarios retain state
across repeated commands, faults and restarts with independent expectations. Saved
executions can be replay-verified from their pinned snapshots through the UI/API;
separate immutable checks distinguish reproduction, mismatch and unsupported replay.
Explicit owner-command steps now share state and receipts with graph entry calls.
Shared workflows now execute pure decisions and commands on the same system owners,
then resume broker tasks only from correlated acknowledgements. Exact provider
capabilities, six integrated acceptance scenarios and portable replay close M1
within its bounded offline scope. Manual UI acceptance remains pending. Live
implementation adapters, nested workflow calls and calibrated continuum simulation
remain subsequent work; capability statuses may cover broader scope than M1.
See [M1 manual acceptance](M1_MANUAL_ACCEPTANCE.md).
System architecture executes the supported typed bindings;
the timing/deployment simulator uses its existing project model under Simulation.
Design opens System architecture and its installed library. Library sections distinguish
components, supporting services and actors/workload. The separate timing-profile library
retains its existing simulation behavior; these models are not silently merged.

## Implementation sequence

`;
doc += `**Weighted implementation: ${roadmapProgress.total}%**\n\n${progressMethod}\n\n`;
for (const h of progressHistory) doc += `- ${h.date}: **${h.percentage}%** — ${h.reason}\n`;
doc += '\n';
for (const m of milestones) {
  doc += `### ${m.id}. ${m.name}\n\n`;
  doc += `Capabilities: ${m.features.map(id=>capabilityLocation(id).capability.label).join(', ')}.\n\n`;
  doc += `**Exit gate:** ${m.gate}\n\n`;
  const progress=roadmapProgress.milestones.find(p=>p.id===m.id);
  doc += `**Milestone progress: ${progress.percentage}% · roadmap weight: ${progress.weight}%**\n\n`;
  for (const c of progress.checkpoints) doc += `- [${c.done?'x':' '}] ${c.acceptance}${c.done ? ' Evidence: '+c.evidence.map(e=>`[${e}](../${e})`).join(', ')+'.' : ''}\n`;
  doc += '\n';
}
doc += `Runtime findings return to engineering as evidence and proposed changes.
Source modifications, schema migrations and updated policies pass through the
implementation and release gates. Disconnected runtime autonomy must be scoped
by explicit delegation, consistency rules and expiry behavior.

## Capability inventory

`;
for (const plane of blueprint) {
  doc += `### ${plane.label}\n\n${plane.description}\n\n`;
  for (const group of plane.groups) {
    doc += `#### ${group.label}\n\n`;
    for (const c of group.capabilities) {
      doc += `**${c.label} — ${statusLabels[c.status]}** (\`${c.id}\`)\n\n`;
      doc += `${c.purpose}\n\n- Today: ${c.today}\n- Next: ${c.next}\n- Acceptance: ${c.acceptance}\n`;
      doc += `- Dependencies: ${c.dependsOn.length ? c.dependsOn.map(id=>capabilityLocation(id).capability.label).join(', ') : 'None in this inventory.'}\n\n`;
    }
  }
}
doc += `## Using the blueprint\n\nOpen **Product blueprint** in the sidebar. Search or filter by plane and status.
Choose **View blueprint** for current scope, next work, dependencies and an
acceptance condition. **Open** appears only for a working destination. The
**Implementation sequence** tab shows milestone gates. Planned runtime entries
open details and never invoke deployment controls. Existing drafts and workspace
IDs are retained across the navigation change.
`;
writeFileSync(new URL('../docs/PRODUCT_BLUEPRINT.md',import.meta.url),doc);
console.log('Updated docs/PRODUCT_BLUEPRINT.md from the studio capability catalog.');
