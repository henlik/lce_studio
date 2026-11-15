<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import {ChevronRight,Folder,FileCode2,FlaskConical,Link2,Box} from 'lucide-vue-next';
import type {Artifact,Compilation,Span} from '../configuration';
import {componentChildren,componentTests,matchesComponent,parentCount} from '../configurationNavigation';
const props=defineProps<{artifact:Artifact;compilation:Compilation;chain:string[];active:string;activeChain:string[];query:string}>();
const emit=defineEmits<{open:[symbol:string,chain:string[],span?:Span];create:[symbol:string,chain:string[]]}>();
const expanded=ref(props.chain.length===1),children=computed(()=>componentChildren(props.compilation,props.artifact.symbol).filter(c=>matchesComponent(props.compilation,c.artifact,props.query)));
const tests=computed(()=>componentTests(props.compilation,props.artifact)),shared=computed(()=>parentCount(props.compilation,props.artifact.symbol)>1);
const recursive=computed(()=>props.chain.slice(0,-1).includes(props.artifact.symbol));
const primitives=computed(()=>props.artifact.kind==='behavior'?(props.artifact.body.behavior?.nodes||[]).filter((n:any)=>n.op):[]);
watch(()=>props.activeChain,chain=>{if(chain.includes(props.artifact.symbol))expanded.value=true});
</script>
<template>
 <div v-if="matchesComponent(compilation,artifact,query)" class="component-tree-node">
  <div class="tree-component" :class="{selected:active===artifact.symbol}">
   <button class="tree-toggle" :aria-label="(expanded?'Collapse ':'Expand ')+(artifact.body.name||artifact.id)" :aria-expanded="expanded||!!query" @click="expanded=!expanded"><ChevronRight :size="14" :class="{expanded:expanded||query}"/></button>
   <button class="tree-name" :title="artifact.id+' @ '+artifact.version" @click="emit('open',artifact.symbol,chain)"><Folder :size="15"/><span>{{artifact.body.name||artifact.id}}</span><Link2 v-if="shared" :size="13" aria-label="Shared component"/></button>
  </div>
  <div v-if="(expanded||query)&&!recursive" class="tree-contents">
   <small class="tree-kind">{{artifact.body.designLevel||artifact.kind}} · {{artifact.version}}{{shared?' · shared reference':''}}</small>
   <button class="tree-file" @click="emit('open',artifact.symbol,chain,artifact.origin)"><FileCode2 :size="14"/>Source</button>
   <button v-for="t in tests" :key="t.name" class="tree-file" :title="t.origin.file" @click="emit('open',artifact.symbol,chain,t.origin)"><FlaskConical :size="14"/><span>Testbench · {{t.name.split('.').at(-1)}}</span></button>
   <button v-if="!tests.length&&artifact.kind!=='definition'" class="tree-file untested" @click="emit('create',artifact.symbol,chain)"><FlaskConical :size="14"/>No tests yet · Add testbench</button>
   <small v-else-if="!tests.length" class="tree-kind">Test the executable aspects below.</small>
   <small v-if="children.some(c=>c.relation==='subcomponent')" class="tree-group">Subcomponents</small>
   <ConfigurationTreeNode v-for="child in children.filter(c=>c.relation==='subcomponent')" :key="child.artifact.symbol" :artifact="child.artifact" :compilation="compilation" :chain="[...chain,child.artifact.symbol]" :active="active" :active-chain="activeChain" :query="query" @open="(s,c,p)=>emit('open',s,c,p)" @create="(s,c)=>emit('create',s,c)"/>
   <small v-if="children.some(c=>c.relation==='aspect')" class="tree-group">Aspects</small>
   <ConfigurationTreeNode v-for="child in children.filter(c=>c.relation==='aspect')" :key="child.artifact.symbol" :artifact="child.artifact" :compilation="compilation" :chain="[...chain,child.artifact.symbol]" :active="active" :active-chain="activeChain" :query="query" @open="(s,c,p)=>emit('open',s,c,p)" @create="(s,c)=>emit('create',s,c)"/>
   <details v-if="primitives.length" class="tree-primitives"><summary>Engine primitives ({{primitives.length}})</summary><button v-for="p in primitives" :key="p.id" class="tree-file" :title="'Built-in operation configured in this source'" @click="emit('open',artifact.symbol,chain,artifact.origin)"><Box :size="13"/>{{p.id}} · {{p.op}}</button><small>Built-in semantics; tested through this component.</small></details>
  </div>
  <small v-if="recursive" class="tree-kind">Reference to an ancestor; open its source above.</small>
 </div>
</template>
<style scoped>
.component-tree-node{min-width:0}.tree-component{display:flex;align-items:center;border-radius:4px;min-width:0;margin:2px 0}.tree-component:hover,.selected{background:#e1ecef}.tree-toggle,.tree-name,.tree-file{background:transparent;color:#314858;border:0;display:flex;align-items:center;gap:6px;font-size:12px;text-align:left;border-radius:4px;min-width:0;padding:6px}.tree-toggle{flex:none;padding:6px 2px}.tree-toggle svg{transition:transform .15s}.tree-toggle .expanded{transform:rotate(90deg)}.tree-name{flex:1}.tree-name span,.tree-file span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.tree-name svg,.tree-file svg{flex:none;color:#527a8c}.tree-contents{padding-left:12px;margin-left:8px;border-left:1px solid #cad9df}.tree-file{width:100%}.tree-file:hover{background:#dce9ed}.tree-kind{font-size:10px;color:#647c89;display:block;padding:3px 6px;overflow-wrap:anywhere}.tree-group{font-size:10px;font-weight:700;letter-spacing:.06em;text-transform:uppercase;display:block;margin:10px 6px 4px;color:#647c89}.untested{color:#806230}.tree-primitives{font-size:11px;color:#607987;padding:4px}.tree-primitives small{font-size:10px}.tree-primitives summary{cursor:pointer}button:focus-visible,summary:focus-visible{outline:2px solid #247d99;outline-offset:1px}
</style>
