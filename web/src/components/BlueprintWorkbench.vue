<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue';
import {ArrowRight, CheckCircle2, CircleDashed, CircleDot} from 'lucide-vue-next';
import {blueprint, capabilities, capabilityLocation, canOpenCapability, milestones, statusLabels, type CapabilityStatus} from '../studioBlueprint';
const props = defineProps<{selected: string}>();
const emit = defineEmits<{inspect:[id:string]; open:[id:string]}>();
const query = ref('');
const status = ref<CapabilityStatus | 'all'>('all');
const planeFilter = ref('all');
const tab = ref<'map'|'sequence'>('map');
const focused = computed(() => capabilityLocation(props.selected));
const detail = ref<HTMLElement | null>(null);
watch(() => props.selected, async id => {
  if (!id) return;
  await nextTick();
  detail.value?.focus({preventScroll:true});
  detail.value?.scrollIntoView({block:'start'});
}, {immediate:true});
const visible = computed(() => blueprint.filter(p => planeFilter.value === 'all' || p.id === planeFilter.value).map(plane => ({...plane,groups:plane.groups.map(group => ({...group,capabilities:group.capabilities.filter(c => (status.value === 'all' || c.status === status.value) && (c.label+' '+c.purpose+' '+c.today+' '+c.next).toLowerCase().includes(query.value.toLowerCase()))})).filter(g => g.capabilities.length)})).filter(p => p.groups.length));
const counts = computed(() => ({available:capabilities.filter(c=>c.status==='available').length,partial:capabilities.filter(c=>c.status==='partial').length,planned:capabilities.filter(c=>c.status==='planned').length}));
const statusIcons = {available:CheckCircle2,partial:CircleDot,planned:CircleDashed};
function clearFilters(){query.value='';status.value='all';planeFilter.value='all';}
</script>

<template>
<section class="blueprint-workbench" aria-label="Product blueprint">
  <header class="blueprint-intro">
    <div><span class="eyebrow">LCE STUDIO · IMPLEMENTATION BLUEPRINT</span><h2>From engineering intent to verified operation</h2><p>Engineering defines and authorizes. Runtime observes and adapts within that authority. Shared foundations support both planes.</p></div>
    <div class="status-counts" aria-label="Capability counts"><span v-for="(count,key) in counts" :key="key" :class="key"><component :is="statusIcons[key]" :size="15"/>{{count}} {{statusLabels[key]}}</span></div>
  </header>
  <p class="scope-note"><strong>Scope matters.</strong> Available means usable within the stated scope. Partial means an implemented foundation with integration still needed. Planned means no working control yet. These are capability counts, not a percentage of product completion.</p>

  <section v-if="focused" ref="detail" class="capability-detail" aria-label="Capability detail" tabindex="-1">
    <div class="detail-heading"><div><span class="eyebrow">{{focused.plane.label}} / {{focused.group.label}}</span><h2>{{focused.capability.label}} <small :class="focused.capability.status">{{statusLabels[focused.capability.status]}}</small></h2></div><button class="quiet" @click="emit('inspect','')">Back to full blueprint</button></div>
    <p>{{focused.capability.purpose}}</p>
    <dl><div><dt>Implemented today</dt><dd>{{focused.capability.today}}</dd></div><div><dt>Next implementation</dt><dd>{{focused.capability.next}}</dd></div><div><dt>Acceptance condition</dt><dd>{{focused.capability.acceptance}}</dd></div></dl>
    <div v-if="focused.capability.dependsOn.length" class="dependencies"><strong>Depends on</strong><button v-for="id in focused.capability.dependsOn" :key="id" @click="emit('inspect',id)">{{capabilityLocation(id)?.capability.label}}</button></div>
    <button v-if="canOpenCapability(focused.capability)" class="primary" @click="emit('open',focused.capability.id)">Open available workspace <ArrowRight :size="14"/></button>
    <p v-else class="planned-note">Blueprint only. No execution or deployment action is available here.</p>
  </section>

  <div class="blueprint-tabs" role="group" aria-label="Blueprint presentation"><button :aria-pressed="tab==='map'" @click="tab='map'">Capability map</button><button :aria-pressed="tab==='sequence'" @click="tab='sequence'">Implementation sequence</button></div>
  <template v-if="tab==='map'">
    <div class="blueprint-filters"><label>Find a capability<input v-model="query" type="search" placeholder="Search capabilities or remaining work"/></label><label>Plane<select v-model="planeFilter"><option value="all">All planes</option><option v-for="plane in blueprint" :key="plane.id" :value="plane.id">{{plane.label}}</option></select></label><label>Status<select v-model="status"><option value="all">All statuses</option><option v-for="(label,key) in statusLabels" :key="key" :value="key">{{label}}</option></select></label><button @click="clearFilters">Clear filters</button></div>
    <p v-if="!visible.length" class="empty">No capabilities match these filters.</p>
    <section v-for="plane in visible" :key="plane.id" class="blueprint-plane"><h2>{{plane.label}}</h2><p>{{plane.description}}</p>
      <div class="blueprint-groups"><section v-for="group in plane.groups" :key="group.id" class="blueprint-group"><h3>{{group.label}}</h3>
        <article v-for="item in group.capabilities" :key="item.id" class="capability-card" :class="{selected:selected===item.id}"><div class="card-heading"><h4>{{item.label}}</h4><span class="capability-status" :class="item.status"><component :is="statusIcons[item.status]" :size="13"/>{{statusLabels[item.status]}}</span></div><p>{{item.today}}</p><p class="remaining"><strong>Next:</strong> {{item.next}}</p><div class="card-actions"><button @click="emit('inspect',item.id)">View blueprint</button><button v-if="canOpenCapability(item)" @click="emit('open',item.id)">Open <ArrowRight :size="12"/></button></div></article>
      </section></div>
    </section>
  </template>
  <section v-else class="milestones" aria-label="Implementation sequence">
    <p class="scope-note"><strong>Security is cross-cutting:</strong> define identity, isolation and release authority from the first milestone. Each gate requires evidence; no milestone is marked complete merely because its menu exists.</p>
    <article v-for="milestone in milestones" :key="milestone.id" class="milestone"><span class="milestone-id">{{milestone.id}}</span><div><h3>{{milestone.name}}</h3><div class="dependencies"><button v-for="id in milestone.features" :key="id" @click="emit('inspect',id)">{{capabilityLocation(id)?.capability.label}} · {{statusLabels[capabilityLocation(id)!.capability.status]}}</button></div><p><strong>Exit gate:</strong> {{milestone.gate}}</p></div></article>
    <p class="scope-note">Runtime findings return to engineering as evidence and proposed changes. New code, policies and schema migrations must pass the implementation and release gates; a runtime planner cannot rewrite its own authority.</p>
  </section>
</section>
</template>

<style scoped>
.blueprint-workbench{padding:24px;min-width:0;color:#345468;max-width:1600px;margin:auto}.blueprint-intro,.detail-heading{display:flex;align-items:flex-start;gap:20px;justify-content:space-between;flex-wrap:wrap}.blueprint-intro>div:first-child{flex:1;min-width:min(300px,100%)}.eyebrow{font-size:10px;letter-spacing:1.2px;color:#668196}.blueprint-workbench h2{font-size:21px;line-height:1.35;margin:8px 0}.blueprint-workbench h3{font-size:16px;margin:12px 0}.blueprint-workbench p{font-size:13px;line-height:1.65;margin:9px 0}.status-counts{display:flex;flex-wrap:wrap;gap:8px}.status-counts span,.capability-status{display:inline-flex;gap:5px;align-items:center;font-size:11px;padding:6px 9px;border-radius:20px;white-space:nowrap}.available{color:#226747;background:#e9f7ef}.partial{color:#805d16;background:#fff4d6}.planned{color:#556b82;background:#edf1f6}.scope-note{padding:13px 16px;border:1px solid #d9e3eb;border-radius:8px;background:#f6f9fc}.capability-detail{margin:20px 0;padding:20px;border:1px solid #75b5a5;border-radius:12px;background:#fff;scroll-margin:20px}.capability-detail h2 small{display:inline-block;font-size:11px;padding:5px 9px;border-radius:12px;vertical-align:middle}.capability-detail dl{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px}.capability-detail dt{font-weight:650;font-size:12px;margin-bottom:8px}.capability-detail dd{margin:0;font-size:13px;line-height:1.6}.dependencies,.card-actions,.blueprint-tabs{display:flex;flex-wrap:wrap;gap:8px;align-items:center;margin:12px 0}.dependencies strong{font-size:12px}.blueprint-workbench button{display:inline-flex;gap:6px;align-items:center;justify-content:center;border:1px solid #c8d7e0;background:#fff;border-radius:6px;padding:8px 11px;color:#345468;cursor:pointer;font:inherit;font-size:12px;max-width:100%;white-space:normal}.blueprint-workbench button:hover{background:#eaf5f0}.blueprint-workbench button:focus-visible{outline:2px solid #247d66;outline-offset:2px}.blueprint-workbench button.primary,.blueprint-tabs button[aria-pressed=true]{background:#246e5d;border-color:#246e5d;color:#fff}.blueprint-filters{display:flex;flex-wrap:wrap;gap:12px;align-items:end;margin:18px 0}.blueprint-filters label{display:grid;gap:6px;font-size:12px;flex:1;min-width:min(180px,100%)}.blueprint-filters input,.blueprint-filters select{box-sizing:border-box;width:100%;min-width:0;border:1px solid #cbdce3;border-radius:6px;padding:10px;background:#fff;color:#345468;font:inherit}.blueprint-plane{margin-top:28px}.blueprint-groups{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(330px,100%),1fr));gap:18px;align-items:start}.blueprint-group{min-width:0}.capability-card{background:#fff;padding:15px;border:1px solid #dbe5eb;border-radius:8px;margin:10px 0;overflow-wrap:anywhere}.capability-card.selected{border-color:#368e74;box-shadow:0 0 0 1px #368e74}.card-heading{display:flex;justify-content:space-between;gap:8px;align-items:flex-start;flex-wrap:wrap}.card-heading h4{font-size:14px;margin:4px 0}.remaining{color:#647b8a}.card-actions{justify-content:space-between;margin-bottom:0}.milestone{display:flex;gap:18px;padding:22px 0;border-bottom:1px solid #d6e2e9}.milestone>div{min-width:0}.milestone-id{display:grid;place-content:center;flex-shrink:0;width:42px;height:42px;border-radius:50%;background:#e6f3ed;color:#286854;font-weight:650}.milestone h3{margin-top:8px}.planned-note,.empty{color:#738495;font-size:12px}@media(max-width:700px){.blueprint-workbench{padding:14px}.capability-detail{padding:14px}.capability-detail dl{grid-template-columns:1fr}.blueprint-groups{grid-template-columns:1fr}.milestone{gap:10px}.blueprint-workbench h2{font-size:18px}}
</style>
