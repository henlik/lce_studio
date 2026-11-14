<script setup lang="ts">
import { computed, ref } from "vue";
import {
  Search,
  Plus,
  Shapes,
  Boxes,
  Network,
  Upload,
  Download,
  Share2,
} from "lucide-vue-next";
import {
  lockKey,
  referenceKey,
  type PrimitiveSpec,
  type CompositeLock,
  type ComponentBundle,
} from "../behaviorTypes";
const props = defineProps<{
  primitives: PrimitiveSpec[];
  components: CompositeLock[];
  examples: ComponentBundle[];
  disabled?: boolean;
  full?: boolean;
}>();
const emit = defineEmits<{
  add: [kind: string, key: string];
  import: [];
  export: [];
  publish: [];
}>();
const query = ref("");
const matches = (...values: string[]) =>
  values.join(" ").toLowerCase().includes(query.value.toLowerCase());
const items = computed(() => {
  const values = props.components.map((p) => ({
    key: lockKey(p),
    kind: "nested",
    name: p.package.name,
    description: p.package.description,
    version: p.package.version,
    level: p.package.designLevel || "component",
    origin: "Workspace",
    id: p.package.id,
  }));
  for (const bundle of props.examples) {
    const p = bundle.packages.find(
      (p) => lockKey(p) === referenceKey(bundle.root),
    );
    if (p && !values.some((x) => x.key === lockKey(p)))
      values.push({
        key: lockKey(p),
        kind: "example",
        name: p.package.name,
        description: p.package.description,
        version: p.package.version,
        level: p.package.designLevel || "component",
        origin: "Built-in",
        id: p.package.id,
      });
  }
  return values.filter((p) => matches(p.name, p.description, p.id));
});
function drag(e: DragEvent, kind: string, key: string) {
  if (props.disabled || props.full || !e.dataTransfer) return;
  e.dataTransfer.setData(
    "application/x-lce-component",
    JSON.stringify({ kind, key }),
  );
  e.dataTransfer.effectAllowed = "copy";
}
</script>
<template>
  <aside class="component-palette" aria-label="Component palette">
    <h3><Boxes :size="17" /> Component library</h3>
    <label class="palette-search"
      ><Search :size="15" /><input
        v-model="query"
        aria-label="Search component palette"
        placeholder="Search parts…"
    /></label>
    <p class="palette-help">
      Drag onto the canvas or select + Add. Select a placed node to configure it
      on the right.
    </p>
    <p v-if="full" class="palette-help">
      This definition has reached its 32-node limit. Extract reusable parts into
      another component.
    </p>
    <details open>
      <summary>
        <Shapes :size="15" /> Primitives
        <span>{{
          primitives.filter((p) => matches(p.name, p.op, p.description)).length
        }}</span>
      </summary>
      <article
        v-for="p in primitives.filter((p) =>
          matches(p.name, p.op, p.description),
        )"
        :key="p.op"
        class="palette-card primitive"
        :draggable="!disabled && !full"
        @dragstart="drag($event, 'primitive', p.op)"
      >
        <div>
          <b>◇ {{ p.name }}</b
          ><small>Built-in · {{ p.inputs.length }} inputs</small>
        </div>
        <p>{{ p.description }}</p>
        <button
          class="button quiet"
          :disabled="disabled || full"
          :aria-label="`Add primitive ${p.name}`"
          @click="emit('add', 'primitive', p.op)"
        >
          <Plus :size="13" /> Add
        </button>
      </article>
    </details>
    <details v-for="level in ['component', 'system']" :key="level" open>
      <summary>
        <Boxes v-if="level === 'component'" :size="15" /><Network
          v-else
          :size="15"
        />
        {{ level === "component" ? "Components" : "Systems" }}
        <span>{{ items.filter((p) => p.level === level).length }}</span>
      </summary>
      <article
        v-for="p in items.filter((p) => p.level === level)"
        :key="p.key"
        class="palette-card"
        :class="level"
        :draggable="!disabled && !full"
        @dragstart="drag($event, p.kind, p.key)"
      >
        <div>
          <b>{{ level === "system" ? "▣" : "▢" }} {{ p.name }}</b
          ><small>{{ p.origin }} · {{ p.version }}</small>
        </div>
        <p>{{ p.description }}</p>
        <button
          class="button quiet"
          :disabled="disabled || full"
          :aria-label="`Add ${p.name} ${p.version}`"
          @click="emit('add', p.kind, p.key)"
        >
          <Plus :size="13" /> Add
        </button>
      </article>
      <p v-if="!items.some((p) => p.level === level)" class="palette-help">
        {{
          level === "system"
            ? "No pure systems yet. Choose System in a definition’s design level, then publish it to this studio. Stateful orchestration belongs in Workflows."
            : "No matching components. Import a bundle or publish your definition to this studio."
        }}
      </p>
    </details>
    <details class="palette-sharing">
      <summary><Share2 :size="15" /> Reuse & sharing</summary>
      <button
        class="button secondary"
        :disabled="disabled"
        @click="emit('import')"
      >
        <Upload :size="14" /> Import bundle</button
      ><button
        class="button secondary"
        :disabled="disabled"
        @click="emit('export')"
      >
        <Download :size="14" /> Export current bundle</button
      ><button
        class="button secondary"
        :disabled="disabled"
        @click="emit('publish')"
      >
        <Share2 :size="14" /> Publish to this studio
      </button>
      <p class="palette-help">
        Publishing installs an immutable version in this studio’s catalog. Share
        the exported bundle with other engineers. A public community marketplace
        is not connected.
      </p>
    </details>
  </aside>
</template>
<style scoped>
.component-palette {
  padding: 14px;
  background: #fff;
  border: 1px solid #dbe5ec;
  border-radius: 10px;
  max-height: 78vh;
  overflow: auto;
}
.component-palette h3 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 14px;
  font-size: 14px;
}
.palette-search {
  display: flex;
  align-items: center;
  gap: 6px;
  border: 1px solid #d1dfe8;
  border-radius: 6px;
  padding: 7px;
}
.palette-search input {
  min-width: 0;
  width: 100%;
  border: 0;
  background: transparent;
}
.palette-help {
  color: #5d7586;
  font-size: 12px;
  line-height: 1.6;
}
.component-palette summary {
  display: flex;
  gap: 7px;
  align-items: center;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  padding: 12px 0;
}
.component-palette summary span {
  margin-left: auto;
  color: #708594;
}
.component-palette .palette-card {
  display: block;
  border: 1px solid #dfe8ed;
  border-left: 3px solid #15957c;
  border-radius: 7px;
  padding: 10px;
  margin-bottom: 8px;
  cursor: grab;
  background: #fbfefd;
}
.palette-card.primitive {
  border-left-color: #7957bb;
  background: #fcfaff;
}
.palette-card.system {
  border-left-color: #c78622;
  background: #fffdf7;
}
.palette-card b {
  font-size: 12px;
}
.palette-card small {
  display: block;
  font-size: 11px;
  color: #637b8e;
  margin-top: 4px;
}
.palette-card p {
  font-size: 11px;
  line-height: 1.5;
  color: #526c80;
  margin: 7px 0;
}
.palette-card button {
  padding: 4px 7px;
}
.palette-sharing > .button {
  width: 100%;
  margin: 4px 0;
  justify-content: flex-start;
}
</style>
