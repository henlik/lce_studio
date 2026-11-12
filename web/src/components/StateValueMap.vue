<script setup lang="ts">
import { valueText, type TypedValue } from "../behaviorTypes";
defineProps<{ values: Record<string, TypedValue>; empty?: string }>();
</script>
<template>
  <dl class="state-values">
    <div v-for="(value, name) in values" :key="name">
      <dt>
        {{ name }}
        <small
          >{{ value.type
          }}{{ value.recordRef ? `<${value.recordRef.record}>` : "" }}</small
        >
      </dt>
      <dd>{{ valueText(value) }}</dd>
    </div>
    <p v-if="!Object.keys(values).length">{{ empty || "No values" }}</p>
  </dl>
</template>
<style scoped>
.state-values {
  margin: 10px 0;
  display: grid;
  gap: 8px;
}
.state-values > div {
  border: 1px solid #e0eaee;
  background: #f8fbfc;
  border-radius: 6px;
  padding: 10px 12px;
  min-width: 0;
}
.state-values dt {
  font-size: 12px;
  color: #55768b;
}
.state-values small {
  font-weight: 400;
  color: #7a909d;
  margin-left: 7px;
}
.state-values dd {
  font-family: monospace;
  font-size: 12px;
  line-height: 1.7;
  color: #385d73;
  margin: 5px 0 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.state-values p {
  font-size: 12px;
  color: #738a98;
}
</style>
