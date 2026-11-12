<script setup lang="ts">
import { computed } from "vue";
import type { StateMachine } from "../stateTypes";
const props = defineProps<{ machine: StateMachine; selected: number }>();
const emit = defineEmits<{ select: [index: number] }>();
const height = computed(() =>
  Math.max(
    230,
    Math.max(
      props.machine.state.length,
      props.machine.commands.length,
      props.machine.effects.length,
    ) *
      76 +
      75,
  ),
);
const wires = computed(() => {
  const command = props.machine.commands[props.selected];
  if (!command) return [];
  const y = props.selected * 76 + 85;
  const reads = new Set(
    Object.values(command.bindings).flatMap((b) => (b.state ? [b.state] : [])),
  );
  const items: { path: string; kind: string }[] = [];
  props.machine.state.forEach((slot, i) => {
    const sy = i * 76 + 85;
    if (reads.has(slot.name))
      items.push({
        kind: "read",
        path: `M 250 ${sy - 6} C 305 ${sy - 6},310 ${y - 6},365 ${y - 6}`,
      });
    if (Object.hasOwn(command.writes, slot.name))
      items.push({
        kind: "write",
        path: `M 365 ${y + 7} C 310 ${y + 7},305 ${sy + 7},250 ${sy + 7}`,
      });
  });
  props.machine.effects.forEach((effect, i) => {
    if (command.effects.some((e) => e.effect === effect.name))
      items.push({
        kind: "effect",
        path: `M 595 ${y} C 650 ${y},655 ${i * 76 + 85},710 ${i * 76 + 85}`,
      });
  });
  return items;
});
</script>
<template>
  <section class="state-diagram">
    <div class="state-diagram-title">
      <strong>OWNERSHIP & COMMAND BOUNDARIES</strong
      ><span>Choose a handler to inspect its connections</span>
    </div>
    <div class="state-diagram-scroll">
      <svg
        :viewBox="`0 0 960 ${height}`"
        role="img"
        aria-label="Owned state, pure command handlers and committed effect intentions"
      >
        <defs>
          <marker
            v-for="kind in ['read', 'write', 'effect']"
            :id="`state-${kind}`"
            :key="kind"
            markerWidth="7"
            markerHeight="7"
            refX="6"
            refY="3.5"
            orient="auto"
          >
            <path
              d="M0,0 L7,3.5 L0,7"
              :fill="kind === 'write' ? '#b99061' : '#55978d'"
            />
          </marker>
        </defs>
        <text x="20" y="26" class="column">OWNED STATE</text>
        <text x="365" y="26" class="column">PURE DECISION HANDLERS</text>
        <text x="710" y="26" class="column">EFFECT INTENTIONS</text>
        <path
          v-for="(w, i) in wires"
          :key="i"
          :d="w.path"
          fill="none"
          :stroke="w.kind === 'write' ? '#b99061' : '#55978d'"
          stroke-width="1.8"
          :stroke-dasharray="w.kind === 'write' ? '5 4' : undefined"
          :marker-end="`url(#state-${w.kind})`"
        />
        <g
          v-for="(slot, i) in machine.state"
          :key="i"
          :transform="`translate(20,${52 + i * 76})`"
        >
          <rect width="230" height="61" rx="7" />
          <text x="12" y="24">{{ slot.name }}</text>
          <text x="12" y="45" class="detail">
            {{ slot.durability }} · {{ slot.type
            }}{{ slot.recordRef ? `<${slot.recordRef.record}>` : "" }}
          </text>
        </g>
        <g
          v-for="(command, i) in machine.commands"
          :key="i"
          :transform="`translate(365,${52 + i * 76})`"
          role="button"
          tabindex="0"
          :aria-label="`Inspect command ${command.name}`"
          @click="emit('select', i)"
          @keydown.enter="emit('select', i)"
        >
          <rect
            width="230"
            height="61"
            rx="7"
            :class="{ selected: i === selected }"
          />
          <text x="12" y="24">{{ command.name }}</text>
          <text x="12" y="45" class="detail">
            {{ command.decision.root.packageId || "Choose a locked handler" }}
          </text>
        </g>
        <g
          v-for="(effect, i) in machine.effects"
          :key="i"
          :transform="`translate(710,${52 + i * 76})`"
        >
          <rect width="230" height="61" rx="7" />
          <text x="12" y="24">{{ effect.name }}</text>
          <text x="12" y="45" class="detail">
            {{ effect.payload.length }} payload fields · simulated intent
          </text>
        </g>
        <text v-if="!machine.effects.length" x="710" y="86" class="detail">
          No declared effects
        </text>
      </svg>
    </div>
    <p>
      Solid arrows read owned state or stage an effect. Dashed arrows propose
      state writes. Only an accepted atomic commit publishes writes and
      intentions.
    </p>
  </section>
</template>
<style scoped>
.state-diagram {
  background: #fff;
  border: 1px solid #dce6ed;
  border-radius: 9px;
  margin: 18px 0;
  overflow: hidden;
}
.state-diagram-title {
  padding: 13px 16px;
  border-bottom: 1px solid #e1e9ed;
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 11px;
  color: #58788b;
}
.state-diagram-scroll {
  overflow: auto;
  max-height: 540px;
  background: #f9fcfc;
}
.state-diagram svg {
  display: block;
  min-width: 800px;
  width: 100%;
}
.state-diagram rect {
  fill: white;
  stroke: #abc7c4;
}
.state-diagram rect.selected {
  stroke: #188b76;
  stroke-width: 2.5;
}
.state-diagram text {
  fill: #395f74;
  font-size: 14px;
}
.state-diagram text.detail {
  font-size: 11px;
  fill: #6c8694;
}
.state-diagram text.column {
  font-size: 11px;
  letter-spacing: 1px;
}
.state-diagram g[role="button"] {
  cursor: pointer;
}
.state-diagram p {
  font-size: 12px;
  color: #6a8190;
  line-height: 1.7;
  margin: 0;
  padding: 12px 16px;
}
</style>
