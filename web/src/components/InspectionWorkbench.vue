<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { api, APIError } from '../api';
import { inspectionVerdict, type InspectionCase, type InspectionPack } from '../inspection';
import { useWorkspaceDraft } from '../composables/useWorkspaceDraft';
import { useActionFeedback } from '../composables/useNotifications';
import { downloadWorkflow } from '../workflowTypes';
import WorkflowJourney from './WorkflowJourney.vue';
import DraftStatus from './DraftStatus.vue';
const emit = defineEmits<{ open: [item: InspectionCase] }>();
const pack = ref<InspectionPack>();
const results = ref<Record<string, { passed: boolean; actual?: unknown; error?: string; at: string; evidence: any }>>({});
const manual = ref<Record<string, boolean>>({});
const selected = ref(''), filter = ref('all'), busy = ref(''), notice = ref(''), error = ref(''), stop = ref(false);
const autosave = useWorkspaceDraft('inspection', { results, manual, selected, filter });
useActionFeedback('Inspection lab', busy, notice, error);
const visible = computed(() => pack.value?.cases.filter(c => filter.value === 'all' || c.workspace === filter.value) || []);
const current = computed(() => pack.value?.cases.find(c => c.id === selected.value));
const evidence = computed(() => results.value[selected.value]?.evidence);
const passed = computed(() => pack.value?.cases.filter(c => results.value[c.id]?.passed).length || 0);
const checks = [
  ['hierarchy', 'Open the routine reception → Structure. Open Arrival, then Queue; select a step and inspect required bindings. Open a decision dependency down to its primitives.'],
  ['execution', 'Run routine and emergency scenarios. In Execution, step Previous/Next, toggle Follow token and Full map. Confirm repeated visits and interrupt cleanup remain visible.'],
  ['interactive', 'In Workflows → Execution → Interactive tasks, start a fresh session, supply a task response, then signal an emergency. Reload with an unsent response and verify recovery.'],
  ['authoring', 'Create an empty project and empty definitions. Add primitives/components from the left library, configure on the right, reorder/color nodes and inspect input/output pins.'],
  ['validation', 'Remove a required binding, validate, and click the reported issue. Repair it and validate again. In Data, inspect the broken affiliation fixture; failure is expected.'],
  ['portability', 'Export each definition and its scenario/fixture, import into a fresh draft, install a version, reopen its exact package, and compare immutable history with an edited draft.'],
  ['data-evolution', 'Data → Behavior: compare the same contract, then remove or change a field and inspect the directional compatibility report.'],
  ['persistence', 'Edit a draft, wait for Saved locally, reload and verify it. Open a second tab and check conflict protection and recovery checkpoints. Save/reopen a project from PostgreSQL.'],
  ['responsive', 'Collapse both panels, expand and exit every canvas, test keyboard navigation and narrow screens. Confirm the page itself does not overflow horizontally.'],
  ['continuum', 'Open the edge placement exercise; compare edge/cloud placements, memory admission, CPU saturation, link bandwidth/outages and host power outages. Inspect timeline, energy and requirement verdicts.'],
];
async function refresh() {
  error.value = '';
  try { pack.value = await api<InspectionPack>('/inspection'); if (!selected.value) selected.value = pack.value.cases[0]?.id || ''; }
  catch (e) { error.value = (e as Error).message; }
}
async function execute(item: InspectionCase) {
  busy.value = item.name;
  try {
    const record = await api<any>(item.endpoint, 'POST', item.body);
    results.value[item.id] = { ...inspectionVerdict(item,record), at:new Date().toISOString(), evidence:record };
  } catch (e) {
    results.value[item.id] = { passed:false, error:(e as Error).message + (e instanceof APIError && e.issues.length ? ' ' + e.issues.map(i => i.path + ': ' + i.message).join('; ') : ''), at:new Date().toISOString(), evidence:null };
  }
}
async function run(item?: InspectionCase) {
  if (busy.value) return;
  stop.value = false; error.value = ''; notice.value = '';
  const cases = item ? [item] : [...visible.value];
  try { for (const c of cases) { if (stop.value) break; selected.value = c.id; await execute(c); } }
  finally { busy.value = ''; }
  const failures = cases.filter(c => results.value[c.id] && !results.value[c.id].passed).length;
  if (failures) error.value = `${failures} inspection checks failed. Open their evidence to investigate.`;
  else notice.value = stop.value ? 'Stopped after the current experiment. Completed evidence retained.' : 'Inspection checks completed. Manual UX checks remain separate.';
}
onMounted(async () => { await autosave.ready; await refresh(); });
</script>
<template>
  <section class="inspection-lab" aria-label="Inspection lab">
    <header><small>REFERENCE USE CASE · SYNTHETIC DATA</small><h2>{{ pack?.name || 'Inspection lab' }}</h2><p>{{ pack?.description }}</p></header>
    <DraftStatus :draft="autosave" />
    <p v-if="error" role="alert" class="inspection-error">{{ error }} <button @click="refresh">Reload pack</button></p>
    <div class="inspection-controls">
      <label>Engine <select v-model="filter"><option value="all">All engines</option><option v-for="w in ['workflows','components','data','state','messages','design','deployment']" :key="w">{{ w }}</option></select></label>
      <button :disabled="!!busy || !pack" @click="run()">Run {{ visible.length }} visible checks</button>
      <button v-if="busy" @click="stop = true" :disabled="stop">Stop after current check</button>
      <button :disabled="!pack" @click="downloadWorkflow({pack,results,manual}, 'regional-care-inspection.json')">Export inspection report</button>
      <strong>{{ passed }} / {{ pack?.cases.length || 0 }} passed</strong>
    </div>
    <p aria-live="polite">{{ busy ? `Running: ${busy}` : notice || 'Choose an exercise, open its design, or run the inspection suite.' }}</p>
    <p class="inspection-note">Runs use the real studio engines. Saved runs remain in their workspace history; component test reports and this checklist are saved locally. A passed check means its stated expectation matched, including intentional failure cases. UI checks are manual.</p>
    <div class="inspection-layout">
      <aside aria-label="Inspection exercises"><button v-for="item in visible" :key="item.id" :class="{active:selected === item.id}" @click="selected = item.id"><small>{{ item.workspace }} · {{ results[item.id] ? results[item.id].passed ? 'PASS' : 'FAIL' : 'NOT RUN' }}</small><strong>{{ item.name }}</strong></button></aside>
      <article v-if="current">
        <h3>{{ current.name }}</h3><p>{{ current.covers }}</p>
        <div class="inspection-controls"><button :disabled="!!busy" @click="emit('open',current)">Open design in {{ current.workspace }}</button><button :disabled="!!busy" @click="run(current)">Run this check</button><button @click="downloadWorkflow(current.body, current.id + '.json')">Export exercise</button></div>
        <p>Expected <code>{{ current.verdictPath }}</code> = <code>{{ JSON.stringify(current.expected) }}</code></p>
        <p v-if="results[current.id]" role="status"><strong>{{ results[current.id].passed ? 'PASS' : 'FAIL' }}</strong> · {{ results[current.id].error || `Observed ${JSON.stringify(results[current.id].actual)}` }} · {{ results[current.id].at }}</p>
        <WorkflowJourney v-if="current.workspace === 'workflows' && evidence?.result" :workflow="current.body.workflow.workflow" :scenario="current.body.scenario" :result="evidence.result" />
        <details v-if="evidence"><summary>Exact execution evidence</summary><pre>{{ JSON.stringify(evidence,null,2) }}</pre></details>
        <details><summary>Exact definition and inputs</summary><pre>{{ JSON.stringify(current.body,null,2) }}</pre></details>
      </article>
    </div>
    <details class="inspection-manual" open><summary>Manual coverage · {{ Object.values(manual).filter(Boolean).length }} / {{ checks.length }} inspected</summary><label v-for="[id,text] in checks" :key="id"><input type="checkbox" v-model="manual[id!]" />{{ text }}</label></details>
    <p class="inspection-note">Not implemented: live clinical systems, authentication enforcement, community marketplace, real deployment, nested executable workflow calls, automatic cross-engine execution, multihop network routing, battery/solar models or migration. Stage paths are view organization, not independently callable workflows.</p>
  </section>
</template>
<style scoped>
.inspection-lab{padding:20px;min-width:0;max-width:100%;color:#294e65}.inspection-lab header{border-bottom:1px solid #dce7ed;margin-bottom:16px}.inspection-lab h2{font-size:24px}.inspection-lab p{font-size:13px;line-height:1.7}.inspection-lab small{font-size:10px;letter-spacing:.06em;color:#617f91}.inspection-controls{display:flex;flex-wrap:wrap;gap:10px;align-items:center}.inspection-lab button,.inspection-lab select{padding:10px;border:1px solid #c9dce8;border-radius:7px;background:#fff;color:#315b74;cursor:pointer}.inspection-lab button:disabled{opacity:.45;cursor:default}.inspection-layout{display:grid;grid-template-columns:minmax(180px,260px) minmax(0,1fr);gap:18px;margin:16px 0}.inspection-layout aside{max-height:680px;overflow:auto;display:flex;flex-direction:column;gap:6px}.inspection-layout aside button{text-align:left}.inspection-layout aside strong,.inspection-layout aside small{display:block;line-height:1.6}.inspection-layout aside button.active{background:#eaf5ff;border-color:#278ac2}.inspection-layout article{min-width:0;border:1px solid #dbe6ed;border-radius:10px;padding:16px;background:white}.inspection-lab pre{max-height:450px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font-size:11px;background:#f3f7fa;padding:12px}.inspection-lab details{margin:12px 0}.inspection-lab summary{cursor:pointer;font-size:13px}.inspection-manual label{display:flex;gap:10px;align-items:start;margin:15px 0;font-size:13px;line-height:1.6}.inspection-note{color:#688292}.inspection-error{color:#b33d35}@media(max-width:800px){.inspection-layout{grid-template-columns:1fr}.inspection-layout aside{max-height:240px}.inspection-lab{padding:12px}}
</style>
