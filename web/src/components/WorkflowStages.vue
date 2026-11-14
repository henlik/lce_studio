<script setup lang="ts">
import { computed } from 'vue';
import type { WorkflowDefinition } from '../workflowTypes';
import { stageBoundary, stageProgress, workflowStages } from '../workflowStages';
const props = defineProps<{ workflow: WorkflowDefinition; path: string; visits?: {nodeId:string}[]; cursor?: number }>();
const emit = defineEmits<{ 'update:path': [path: string]; select: [nodeId: string] }>();
const groups = computed(() => workflowStages(props.workflow));
const children = computed(() => workflowStages(props.workflow, props.path).filter(g => g.path !== props.path));
const boundary = computed(() => stageBoundary(props.workflow, props.path));
const crumbs = computed(() => props.path.split('/').filter(Boolean).map((name,i,a) => ({name,path:a.slice(0,i+1).join('/')})));
</script>
<template>
  <section class="workflow-stages" aria-label="Workflow stages">
    <nav class="stage-crumbs" aria-label="Composition breadcrumb">
      <button :aria-current="!path ? 'page' : undefined" @click="emit('update:path','')">{{ workflow.name || 'Workflow' }}</button>
      <template v-for="crumb in crumbs" :key="crumb.path"><span>›</span><button @click="emit('update:path',crumb.path)">{{ crumb.name }}</button></template>
    </nav>
    <div class="stage-strip">
      <button v-for="group in groups" :key="group.path" :class="[{selected: path === group.path || (!!group.path && path.startsWith(group.path + '/'))}, visits ? stageProgress(group.nodes,visits,cursor || 0).replace(' ','-') : '']" :aria-pressed="path === group.path || (!!group.path && path.startsWith(group.path + '/'))" @click="emit('update:path',group.path)">
        <strong>{{ group.name }}</strong><small>{{ group.nodes.length }} steps <template v-if="visits">· {{ stageProgress(group.nodes,visits,cursor || 0) }}</template></small>
      </button>
    </div>
    <div v-if="path" class="stage-detail">
      <p><strong>{{ path }}</strong> · {{ boundary.ids.length }} steps. Presentation boundary; execution follows the explicit routes.</p>
      <button v-for="child in children" :key="child.path" @click="emit('update:path',child.path)">Open {{ child.name }} →</button>
      <details><summary>Boundary routes and interrupts</summary>
        <p v-if="!boundary.incoming.length && !boundary.outgoing.length">No routes cross this boundary.</p>
        <button v-for="r in [...boundary.incoming,...boundary.outgoing]" :key="r.id" @click="emit('select',boundary.ids.includes(r.from) ? r.to : r.from)">{{ r.from }} → {{ r.to }} · {{ r.label }}</button>
        <p v-for="s in boundary.signals" :key="s.name">Interrupt {{ s.name }} → {{ s.target }}</p>
      </details>
    </div>
  </section>
</template>
<style scoped>
.workflow-stages { overflow-wrap:anywhere; min-width:0; margin:12px 0; border:1px solid #d5e3ec; border-radius:10px; padding:12px; background:#f7fafc; }
.stage-crumbs { display:flex; flex-wrap:wrap; gap:8px; align-items:center; margin-bottom:10px; }
.workflow-stages button { cursor:pointer; border:1px solid #d5e3ec; background:white; color:#365c72; border-radius:7px; padding:9px 12px; overflow-wrap:anywhere; text-align:left; max-width:100%; }
.stage-strip { display:flex; gap:8px; overflow-x:auto; padding-bottom:5px; }
.stage-strip button { flex:1 0 135px; }
.stage-strip button.selected { border-color:#1683bd; background:#e9f6ff; }
.stage-strip strong, .stage-strip small { display:block; }
.stage-strip button.not-visited { background:#f3f5f7; color:#7b8f9a; }
.stage-strip button.visited { border-left:4px solid #16846a; }
.stage-strip button.current { border-left:4px solid #137fca; background:#e9f6ff; }
.stage-strip small { font-size:11px; margin-top:6px; color:#617d8e; }
.stage-detail { font-size:12px; color:#526d80; line-height:1.6; }
.stage-detail button { margin:4px; }
.stage-detail summary { cursor:pointer; }
</style>
