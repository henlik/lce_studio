<script setup lang="ts">
import { computed, ref, watch, nextTick, onBeforeUnmount, useId } from "vue";
import CanvasControls from "./CanvasControls.vue";
import { useCanvasFocus, useCanvasViewport } from "../composables/useCanvasFocus";
import { dataEditorPlacement, type DataSelection } from "../dataCanvas";
import type { ValidationIssue } from "../types";
import type { DataContract } from "../dataTypes";
const props = defineProps<{
  contract: DataContract;
  compact?: boolean;
  selection: DataSelection | null;
  recordPositions: Record<string,{x:number;y:number}>;
  connectionSource: number | null;
  issues?: ValidationIssue[];
  disabled?: boolean;
}>();
const emit = defineEmits<{
  select: [selection: DataSelection];
  close: [];
  'add-record': []; 'add-entity': []; 'add-relationship': []; validate: [];
  'move-record': [name:string,position:{x:number;y:number}];
  connect: [from:number,to:number]; 'cancel-connect': [];
  move: [name: string, position: { x: number; y: number }];
}>();
const svg = ref<SVGSVGElement | null>(null);
const stage = ref<HTMLElement | null>(null), editor = ref<HTMLElement | null>(null);
const editorStyle = ref<Record<string,string>>({left:'8px',top:'8px',width:'390px',maxHeight:'500px'});
const markerID = `data-arrow-${useId().replace(/:/g,'')}`;
const selectionTitle = computed(() => {
 const s=props.selection;
 return !s ? '' : s.kind === 'record' ? (props.contract.records[s.index]?.name ?? '') || 'Unnamed record' : s.kind === 'entity' ? (props.contract.entities[s.index]?.name ?? '') || 'Unnamed entity' : (props.contract.relationships[s.index]?.name ?? '') || 'Unnamed relationship';
});
const orphanRelationships = computed(() => props.contract.relationships.map((r,index) => ({...r,index})).filter(r => !props.contract.entities.some(e => e.name===r.fromEntity) || !props.contract.entities.some(e => e.name===r.toEntity)));
function invalid(kind:DataSelection['kind'],index:number) {
 const prefix = `${kind==='entity'?'entities':kind==='record'?'records':'relationships'}[${index}]`;
 return props.issues?.some(i => i.path.startsWith(prefix));
}
function select(kind:DataSelection['kind'],index:number) {
 if(props.connectionSource !== null && kind==='entity') { if(!props.disabled) emit('connect',props.connectionSource,index); return; }
 emit('select',{kind,index});
}
function placeEditor() {
 if(!stage.value || !props.selection || !editor.value) return;
 const area=stage.value.getBoundingClientRect();
 const s=props.selection;
 const anchor=svg.value?.querySelector(`[data-kind="${s.kind}"][data-index="${s.index}"]`)?.getBoundingClientRect();
 const position=dataEditorPlacement(anchor ? {left:anchor.left-area.left,right:anchor.right-area.left,top:anchor.top-area.top,bottom:anchor.bottom-area.top} : {left:8,right:8,top:8,bottom:8},area.width,area.height);
 editorStyle.value=Object.fromEntries(Object.entries(position).map(([k,v])=>[k,v+'px']));
}
function closeEditor() {
 const selection=props.selection;
 emit('close');
 void nextTick(()=>svg.value?.querySelector<SVGElement>(`[data-kind="${selection?.kind}"][data-index="${selection?.index}"]`)?.focus());
}
function escape(event:KeyboardEvent) {
 if(event.key !== 'Escape') return;
 if(props.connectionSource!==null) { event.preventDefault();event.stopPropagation();emit('cancel-connect'); }
 else if(props.selection) { event.preventDefault();event.stopPropagation();closeEditor(); }
}
let resize:ResizeObserver | undefined;
watch(stage,element=>{ resize?.disconnect(); if(element) { resize=new ResizeObserver(placeEditor);resize.observe(element); } },{flush:'post'});
onBeforeUnmount(()=>resize?.disconnect());
const wrapper = ref<HTMLElement | null>(null);
const viewport = ref<HTMLElement | null>(null);
const { zoom, svgStyle, fit, zoomIn, zoomOut } = useCanvasViewport(viewport);
const { expanded, toggleExpand } = useCanvasFocus(wrapper, fit);
const frozenBounds = ref<{ x: number; y: number; w: number; h: number } | null>(
  null,
);
let drag:
  | {
      name: string;
      kind: "record" | "entity";
      index: number;
      moved: boolean;
      x: number;
      y: number;
      ox: number;
      oy: number;
      transform: DOMMatrix;
    }
  | undefined;
const cards = computed(() =>
  props.contract.entities.map((e,index) => ({
    index,
    ...e,
    fields:
      props.contract.records.find((r) => r.name === e.record)?.fields || [],
  })),
);
const recordCards = computed(() => {
 const bottom = props.contract.entities.length ? Math.max(...cards.value.map(e=>e.position.y+76+e.fields.length*28))+120 : 50;
 const rowHeight=Math.max(160,...props.contract.records.map(r=>110+r.fields.length*28));
 return props.contract.records.map((r,index)=>({...r,index,position:props.recordPositions[r.name] || {x:40+(index%3)*390,y:bottom+Math.floor(index/3)*rowHeight}}));
});
const allCards = computed(()=>[...cards.value,...recordCards.value]);
const wires = computed(() =>
  props.contract.relationships.flatMap((r, index) => {
    const a = cards.value.find((e) => e.name === r.fromEntity),
      b = cards.value.find((e) => e.name === r.toEntity);
    if (!a || !b) return [];
    const lane = props.contract.relationships
      .slice(0, index)
      .filter(
        (k) => k.fromEntity === r.fromEntity && k.toEntity === r.toEntity,
      ).length;
    const forward = a.position.x <= b.position.x,
      sign = forward ? 1 : -1;
    const x1 = a.position.x + (forward ? 280 : 0),
      y1 = a.position.y + 25,
      x2 = b.position.x + (forward ? 0 : 280),
      y2 = b.position.y + 25;
    const top =
      Math.min(...cards.value.map((c) => c.position.y)) - 40 - index * 30;
    if (a.name === b.name) {
      const left = a.position.x,
        right = left + 280,
        selfTop = top - lane * 25;
      return [
        {
          ...r,
          path: `M ${right} ${y1} C ${right + 90} ${selfTop},${left - 90} ${selfTop},${left} ${y2}`,
          x: left + 140,
          y: selfTop + 8,
          minX: left - 100,
          maxX: right + 100,
          minY: selfTop - 15,
          maxY: y1 + 30,
          index,
        },
      ];
    }
    const reach = Math.min(24, Math.max(10, Math.abs(x2 - x1) / 3)),
      out1 = x1 + sign * reach,
      out2 = x2 - sign * reach;
    const path = `M ${x1} ${y1} L ${out1} ${y1} L ${out1} ${top} L ${out2} ${top} L ${out2} ${y2} L ${x2} ${y2}`;
    return [
      {
        ...r,
        path,
        x: (x1 + x2) / 2,
        y: top - 8,
        minX: Math.min(x1, x2) - 140,
        maxX: Math.max(x1, x2) + 140,
        minY: top - 35,
        maxY: Math.max(y1, y2) + 30,
        index,
      },
    ];
  }),
);
const bounds = computed(() => {
  const minX =
      Math.min(
        0,
        ...allCards.value.map((e) => e.position.x),
        ...wires.value.map((w) => w.minX),
      ) - 30,
    minY =
      Math.min(
        0,
        ...allCards.value.map((e) => e.position.y),
        ...wires.value.map((w) => w.minY),
      ) - 30;
  return {
    x: minX,
    y: minY,
    w:
      Math.max(
        700,
        ...allCards.value.map((e) => e.position.x + 310),
        ...wires.value.map((w) => w.maxX),
      ) -
      minX +
      40,
    h:
      Math.max(
        300,
        ...allCards.value.map((e) => e.position.y + 90 + e.fields.length * 28),
        ...wires.value.map((w) => w.maxY),
      ) -
      minY +
      40,
  };
});
const visibleBounds = computed(() => frozenBounds.value || bounds.value);
function point(e: PointerEvent, transform: DOMMatrix) {
  const p = svg.value!.createSVGPoint();
  p.x = e.clientX;
  p.y = e.clientY;
  return p.matrixTransform(transform);
}
function start(kind:"entity"|"record",index:number,e:PointerEvent) {
 if(e.button!==0 || props.disabled || props.connectionSource!==null) return;
 const card=(kind==='entity'?cards.value:recordCards.value)[index];
 const matrix=svg.value?.getScreenCTM();if(!card || !matrix)return;
 const transform=matrix.inverse(),p=point(e,transform);
 suppressClick=false;
 frozenBounds.value={...bounds.value};
 drag={name:card.name,kind,index,moved:false,x:p.x,y:p.y,ox:card.position.x,oy:card.position.y,transform};
 svg.value!.setPointerCapture(e.pointerId);e.preventDefault();
}
function move(e:PointerEvent) {
 if(!drag || props.disabled)return;
 const p=point(e,drag.transform);
 if(Math.hypot(p.x-drag.x,p.y-drag.y)>4)drag.moved=true;
 if(!drag.moved)return;
 const position={x:Math.round(drag.ox+p.x-drag.x),y:Math.round(drag.oy+p.y-drag.y)};
 if(drag.kind==='entity')emit('move',drag.name,position);else emit('move-record',drag.name,position);
}
let suppressClick=false;
function end(event:PointerEvent) {
 const finished=drag; if(!finished)return;
 drag=undefined;frozenBounds.value=null;suppressClick=true;
 if(event.type==='pointerup' && !finished.moved)select(finished.kind,finished.index);
 if(svg.value?.hasPointerCapture(event.pointerId))svg.value.releasePointerCapture(event.pointerId);
}
function cardClick(kind:DataSelection['kind'],index:number) { if(suppressClick) {suppressClick=false;return;} select(kind,index); }
watch(()=>props.selection,async(selection,previous)=>{
 await nextTick();placeEditor();
 if(selection && (selection.kind!==previous?.kind || selection.index!==previous?.index)) {
  const anchor=svg.value?.querySelector<SVGElement>(`[data-kind="${selection.kind}"][data-index="${selection.index}"]`);
  anchor?.scrollIntoView({block:'nearest',inline:'nearest'});
  placeEditor();(editor.value?.querySelector<HTMLElement>('input,select') || editor.value?.querySelector<HTMLElement>('button'))?.focus({preventScroll:true});
 }
},{flush:'post'});
watch([bounds,zoom,expanded],()=>void nextTick(placeEditor));
</script>
<template>
  <section ref="wrapper" class="data-graph" :class="{ 'canvas-focus--expanded': expanded }" @keydown="escape">
    <div class="graph-tools">
      <strong>DATA CANVAS</strong
      ><span
        >{{ contract.records.length }} records · {{ contract.entities.length }} entities ·
        {{ contract.relationships.length }} foreign keys</span
      >
      <CanvasControls label="data diagram" :zoom="zoom" :expanded="expanded" @zoom-in="zoomIn" @zoom-out="zoomOut" @fit="fit" @expand="toggleExpand" />
    </div>
    <div v-if="!compact || expanded" class="canvas-actions" role="toolbar" aria-label="Create data elements">
      <button :disabled="disabled || contract.records.length>=32" @click="emit('add-record')">+ Record</button>
      <button :disabled="disabled || !contract.records.length || contract.entities.length>=32" @click="emit('add-entity')">+ Entity</button>
      <button :disabled="disabled || !contract.entities.length || contract.relationships.length>=64" @click="emit('add-relationship')">+ Relationship</button>
      <button :disabled="disabled" @click="emit('validate')">Validate data</button>
      <span>Click a card or connection to configure it here. Drag cards to arrange.</span>
    </div>
    <div v-if="connectionSource!==null" class="connection-prompt" role="status">Connect from <strong>{{ contract.entities[connectionSource]?.name }}</strong>: choose the target entity on the canvas.<button @click="emit('cancel-connect')">Cancel connection</button></div>
    <div v-if="orphanRelationships.length" class="unresolved-links"><span>Unresolved connections:</span><button v-for="r in orphanRelationships" :key="r.index" @click="select('relationship',r.index)">{{ r.name || 'Unnamed relationship' }} · Fix endpoints</button></div>
    <div ref="stage" class="canvas-stage">
    <div ref="viewport" class="graph-scroll canvas-viewport" @scroll="placeEditor">

      <svg
        ref="svg"
        :viewBox="`${visibleBounds.x} ${visibleBounds.y} ${visibleBounds.w} ${visibleBounds.h}`"
        :style="svgStyle"
        role="group"
        aria-label="Interactive records, entities and relationships"
        @pointermove="move"
        @pointerup="end"
        @pointercancel="end"
        @lostpointercapture="end"
      >
        <defs>
          <marker
            :id="markerID"
            markerWidth="8"
            markerHeight="8"
            refX="7"
            refY="4"
            orient="auto"
          >
            <path d="M0,0 L8,4 L0,8" fill="#649d91" />
          </marker>
        </defs>
        <g v-for="r in wires" :key="r.index" data-kind="relationship" :data-index="r.index" role="button" tabindex="0" :aria-label="`Configure relationship ${r.name}`" :class="{ 'selected-link':selection?.kind==='relationship' && selection.index===r.index, invalid:invalid('relationship',r.index) }" @click="select('relationship',r.index)" @keydown.enter.prevent="select('relationship',r.index)" @keydown.space.prevent="select('relationship',r.index)">
          <path :d="r.path" fill="none" stroke="transparent" stroke-width="18" class="link-hit" />
          <path
            :d="r.path"
            fill="none"
            stroke="#77a99d"
            stroke-width="2"
            :marker-end="`url(#${markerID})`"
          />
          <text :x="r.x" :y="r.y" text-anchor="middle" class="relation-label">
            {{ r.name }} · {{ r.minSources }}..{{
              r.maxSources === 0 ? "*" : "1"
            }}
            → 1
          </text>
          <title>
            {{ r.fromEntity }}({{ r.fromFields.join(", ") }}) →
            {{ r.toEntity }}({{ r.toFields.join(", ") }})
          </title>
        </g>
        <g
          v-for="e in cards"
          :key="e.index"
          data-kind="entity" :data-index="e.index"
          :transform="`translate(${e.position.x},${e.position.y})`"
          class="entity-card" :class="{invalid:invalid('entity',e.index), connecting:connectionSource===e.index}"
          role="button"
          tabindex="0"
          :aria-label="`Configure entity ${e.name}`"
          @pointerdown="start('entity',e.index,$event)"
          @click="cardClick('entity',e.index)"
          @keydown.enter.prevent="select('entity',e.index)"
          @keydown.space.prevent="select('entity',e.index)"
        >
          <rect
            width="280"
            :height="76 + e.fields.length * 28"
            rx="8"
            :class="{ selected: selection?.kind==='entity' && selection.index===e.index }"
          />
          <text x="12" y="24" class="entity-title">{{ e.name }}</text>
          <text x="12" y="44" class="entity-type">Entity · {{ e.record }}</text>
          <line x1="0" y1="55" x2="280" y2="55" stroke="#dce7eb" />
          <g v-for="(f, i) in e.fields" :key="f.name">
            <text x="12" :y="76 + i * 28" class="field-key">
              {{
                e.primaryKey.includes(f.name)
                  ? "PK"
                  : e.uniqueKeys.some((k) => k.includes(f.name))
                    ? "UK"
                    : ""
              }}
            </text>
            <text x="42" :y="76 + i * 28" class="field-name">{{ f.name }}</text>
            <text x="268" :y="76 + i * 28" text-anchor="end" class="field-type">
              {{ f.type }}
            </text>
          </g>
        </g>
        <g v-for="r in recordCards" :key="r.index" data-kind="record" :data-index="r.index"
          :transform="`translate(${r.position.x},${r.position.y})`" class="entity-card record-card"
          :class="{invalid:invalid('record',r.index)}" role="button" tabindex="0" :aria-label="`Configure record ${r.name}`"
          @pointerdown="start('record',r.index,$event)" @click="cardClick('record',r.index)"
          @keydown.enter.prevent="select('record',r.index)" @keydown.space.prevent="select('record',r.index)">
          <rect width="280" :height="76+r.fields.length*28" rx="8" :class="{selected:selection?.kind==='record' && selection.index===r.index}" />
          <text x="12" y="24" class="entity-title">{{ r.name }}</text>
          <text x="12" y="44" class="entity-type">Record type · {{ contract.entities.filter(e=>e.record===r.name).length }} entities</text>
          <line x1="0" y1="55" x2="280" y2="55" stroke="#dce7eb" />
          <g v-for="(f,i) in r.fields" :key="i"><text x="12" :y="76+i*28" class="field-name">{{ f.name }}</text><text x="268" :y="76+i*28" text-anchor="end" class="field-type">{{ f.type }}</text></g>
        </g>
      </svg>
    </div>
    <section v-if="selection && selectionTitle" ref="editor" class="canvas-editor" :style="editorStyle" role="region" :aria-label="`Configure ${selection.kind}: ${selectionTitle}`" @pointerdown.stop>
      <header><div><small>{{ selection.kind.toUpperCase() }}</small><strong>{{ selectionTitle }}</strong></div><button aria-label="Close element settings" title="Close settings (Escape)" @click="closeEditor">×</button></header>
      <div class="canvas-editor-body"><slot name="editor" /></div>
      <footer>Changes update this draft immediately · Escape closes settings.</footer>
    </section>
    </div>
    <p>
      Each source row references exactly one target. The label gives source rows
      per target: 0..* permits many, 1..1 requires exactly one. Drag entity
      cards to arrange the view. Record types are reusable field definitions; entity keys belong to each entity.
    </p>
  </section>
</template>
<style scoped>
.data-graph {
  border: 1px solid #dce6ed;
  border-radius: 9px;
  background: white;
  overflow: hidden;
  margin-bottom: 18px;
}
.data-graph.canvas-focus--expanded { display: flex; flex-direction: column; }
.canvas-stage { position:relative; height:clamp(500px,65vh,760px); min-height:0; min-width:0; }
.canvas-stage .canvas-viewport { height:100%; }
.canvas-focus--expanded .canvas-stage { flex:1; height:auto; }
.canvas-actions { display:flex; flex-wrap:wrap; gap:8px; padding:10px 14px; border-bottom:1px solid #dce6ed; align-items:center; }
.canvas-actions button,.connection-prompt button,.unresolved-links button { background:#fff; border:1px solid #c9ded9; padding:8px 12px; border-radius:6px; color:#286b60; cursor:pointer; font-size:12px; }
.canvas-actions button:disabled { opacity:.45; cursor:default; }
.canvas-actions span { font-size:11px; color:#627e90; }
.connection-prompt,.unresolved-links { padding:10px 14px; background:#eef8f4; font-size:12px; display:flex; flex-wrap:wrap; align-items:center; gap:8px; }
.canvas-editor { position:absolute; z-index:5; display:flex; flex-direction:column; box-sizing:border-box; background:#fff; border:1px solid #85b6aa; border-radius:10px; box-shadow:0 12px 38px #234e6433; overflow:hidden; min-width:0; }
.canvas-editor header { display:flex; align-items:center; justify-content:space-between; gap:10px; padding:12px 14px; background:#eef8f5; border-bottom:1px solid #d2e7df; }
.canvas-editor header small,.canvas-editor header strong { display:block; overflow-wrap:anywhere; color:#285f54; }
.canvas-editor header small { font-size:10px; margin-bottom:4px; }
.canvas-editor header strong { font-size:15px; }
.canvas-editor header button { background:transparent; border:0; font-size:24px; color:#476c60; padding:2px 8px; cursor:pointer; }
.canvas-editor-body { overflow:auto; overscroll-behavior:contain; min-height:0; padding:14px; }
.canvas-editor footer { font-size:10px; color:#698290; padding:9px 12px; background:#f8fbfc; }
.record-card rect { stroke:#8e9dc4; stroke-dasharray:5 3; fill:#fafaff; }
.entity-card.invalid rect { stroke:#c55048; stroke-width:3; }
.entity-card.connecting rect { stroke:#0c9b9b; stroke-width:4; }
g[data-kind=relationship] { cursor:pointer; }
g[data-kind=relationship]:focus { outline:none; }
g.selected-link > path:not(.link-hit),g[data-kind=relationship]:focus > path:not(.link-hit) { stroke:#15866f; stroke-width:4; }
g.invalid > path:not(.link-hit) { stroke:#c55048; }
.entity-card:focus { outline:none; }
.entity-card:focus rect { stroke:#168c78; stroke-width:3; }
@media(max-width:640px) { .canvas-stage { height:660px; } .canvas-editor-body { padding:10px; } }

.graph-tools {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  padding: 10px 16px;
  border-bottom: 1px solid #e1e9ef;
  font-size: 12px;
  color: #627e90;
}
.graph-tools strong {
  margin-right: auto;
  letter-spacing: 0.6px;
}
.graph-tools .button {
  padding: 7px;
}
.graph-scroll {
  overflow: auto;
  background: #f8fbfc;
}
.graph-scroll svg {
  display: block;
  touch-action: none;
  user-select: none;
}
.entity-card {
  cursor: pointer;
}
.entity-card rect {
  fill: white;
  stroke: #a6c7c2;
  stroke-width: 1.5;
}
.entity-card rect.selected {
  stroke: #168c78;
  stroke-width: 3;
}
.entity-title {
  font-size: 16px;
  font-weight: 600;
  fill: #34566c;
}
.entity-type,
.field-type {
  font-size: 12px;
  fill: #698494;
}
.field-name {
  font-size: 13px;
  fill: #45677b;
}
.field-key {
  font-size: 10px;
  font-weight: 700;
  fill: #248e7a;
}
.relation-label {
  font-size: 11px;
  fill: #427969;
  paint-order: stroke;
  stroke: #f8fbfc;
  stroke-width: 4px;
}
.data-graph > p {
  padding: 12px 16px;
  font-size: 12px;
  color: #627e90;
  line-height: 1.7;
  margin: 0;
}
</style>
