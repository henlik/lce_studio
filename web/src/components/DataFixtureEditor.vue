<script setup lang="ts">
import { computed, ref } from "vue";
import type { DataContract, DataFixture } from "../dataTypes";
import { typedDefault, type TypedValue } from "../behaviorTypes";
import TypedValueEditor from "./TypedValueEditor.vue";
const props = defineProps<{
  contract: DataContract;
  fixture: DataFixture;
  disabled?: boolean;
}>();
const selected = ref("");
const entity = computed(
  () =>
    props.contract.entities.find((e) => e.name === selected.value) ||
    props.contract.entities[0],
);
const fields = computed(
  () =>
    props.contract.records.find((r) => r.name === entity.value?.record)
      ?.fields || [],
);
const rows = computed(() =>
  entity.value && Array.isArray(props.fixture.rows?.[entity.value.name])
    ? props.fixture.rows[entity.value.name]
    : [],
);
const count = computed(() =>
  Object.values(props.fixture.rows || {}).reduce(
    (n, a) => n + (Array.isArray(a) ? a.length : 0),
    0,
  ),
);
function add() {
  if (!entity.value || count.value >= 256) return;
  (props.fixture.rows ||= {})[entity.value.name] ||= [];
  props.fixture.rows[entity.value.name].push(
    Object.fromEntries(fields.value.map((f) => [f.name, typedDefault(f.type)])),
  );
}
function set(index: number, name: string, value: TypedValue | undefined) {
  if (!entity.value) return;
  const row = (props.fixture.rows[entity.value.name][index] ||= {});
  if (value) row[name] = value;
  else delete row[name];
}
</script>
<template>
  <div class="fixture-editor">
    <div class="fixture-toolbar">
      <label class="field"
        >Fixture name<input
          v-model="fixture.name"
          aria-label="Data fixture name"
          :disabled="disabled" /></label
      ><label class="field"
        >Entity rows<select
          v-model="selected"
          aria-label="Fixture entity"
          :disabled="disabled"
        >
          <option v-if="!selected" value="">
            {{ contract.entities[0]?.name || "No entities" }}
          </option>
          <option v-for="e in contract.entities" :key="e.name" :value="e.name">
            {{ e.name }}
          </option>
        </select></label
      ><button
        class="button secondary"
        :disabled="disabled || !entity || count >= 256"
        @click="add"
      >
        Add fixture row
      </button>
    </div>
    <p class="help">
      {{ count }} / 256 rows in this complete snapshot. Every required field
      must be supplied. Empty entities remain empty; relationships are checked
      across the whole fixture.
    </p>
    <p
      v-if="!fixture.rows || (entity && fixture.rows[entity.name] === null)"
      class="help"
    >
      This snapshot contains a null row collection. Its invalid shape is
      preserved for validation. Add a row to replace it, or correct it in
      Fixture JSON.
    </p>
    <article v-for="(row, index) in rows" :key="index" class="fixture-row">
      <div class="row-heading">
        <strong>{{ entity?.name }} · row {{ index + 1 }}</strong
        ><button
          class="button quiet"
          :disabled="disabled"
          @click="rows.splice(index, 1)"
        >
          Remove row {{ index + 1 }}
        </button>
      </div>
      <p v-if="!row" class="help">
        Invalid null row. Enter its fields below to replace it.
      </p>
      <div class="fixture-fields">
        <TypedValueEditor
          v-for="f in fields"
          :key="f.name"
          :model-value="row?.[f.name]"
          :type="f.type"
          :label="`${entity?.name}[${index}].${f.name}`"
          :disabled="disabled"
          @update:model-value="(value) => set(index, f.name, value)"
        />
      </div>
      <p
        v-if="
          row && Object.keys(row).some((k) => !fields.some((f) => f.name === k))
        "
        class="help"
      >
        This row contains undeclared fields. Use fixture JSON to inspect and
        correct them.
      </p>
    </article>
    <p v-if="!rows.length" class="help">
      No rows for {{ entity?.name || "this contract" }}. Add a row to start a
      fixture.
    </p>
  </div>
</template>
<style scoped>
.fixture-toolbar {
  display: flex;
  gap: 14px;
  align-items: end;
  flex-wrap: wrap;
}
.fixture-toolbar .field {
  flex: 1;
  margin: 0;
  min-width: 180px;
}
.fixture-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 14px;
}
.fixture-row {
  padding: 16px;
  border: 1px solid #dbe7eb;
  border-radius: 8px;
  margin-top: 14px;
  background: #fbfdfd;
}
.row-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  color: #45667b;
  margin-bottom: 14px;
}
.row-heading .button {
  font-size: 12px;
}
.help {
  font-size: 12px;
  line-height: 1.8;
  color: #657e8e;
}
</style>
