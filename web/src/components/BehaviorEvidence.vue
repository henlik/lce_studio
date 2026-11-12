<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  CheckCircle2,
  CircleAlert,
  FileCheck2,
  Layers3,
} from "lucide-vue-next";
import {
  valueText,
  type BehaviorTestResult,
  type EvaluationRecord,
} from "../behaviorTypes";
const props = defineProps<{
  record: EvaluationRecord | null;
  stale: boolean;
  testResult: BehaviorTestResult | null;
  testsStale: boolean;
  busy?: boolean;
}>();
const emit = defineEmits<{ "load-snapshot": []; "capture-test": [] }>();
const selectedSequence = ref(0);
watch(
  () => props.record?.id,
  () => (selectedSequence.value = 0),
);
const selectedEvent = computed(() =>
  props.record?.result.events.find(
    (e) => e.sequence === selectedSequence.value,
  ),
);
</script>
<template>
  <section v-if="record" class="behavior-evidence">
    <div class="evidence-heading">
      <div>
        <span class="section-eyebrow">PERSISTED EVALUATION</span>
        <h3>Recorded outputs & execution trace</h3>
      </div>
      <span class="evidence-status" :class="{ stale }">{{
        stale ? "Earlier draft or inputs" : "Matches current draft & inputs"
      }}</span>
    </div>
    <p v-if="stale" class="evidence-warning">
      <CircleAlert :size="14" />The definition or input values differ from this
      saved evaluation. These outputs belong to its immutable snapshot.
    </p>
    <div class="evaluation-identity">
      <strong
        >{{ record.result.root.packageId }} @
        {{ record.result.root.version }}</strong
      ><span>{{ new Date(record.createdAt).toLocaleString() }}</span
      ><code
        >Engine {{ record.result.engineVersion }} · bundle
        {{ record.result.bundleHash.slice(0, 12) }}</code
      >
    </div>
    <div class="behavior-output-grid">
      <div
        v-for="(value, name) in record.result.outputs"
        :key="name"
        class="behavior-output"
      >
        <span
          >{{ name }} <small>{{ value.type }}</small></span
        ><strong>{{ valueText(value) }}</strong>
      </div>
      <div class="evaluation-counts">
        <span
          ><strong>{{ record.result.operations }}</strong> primitive
          executions</span
        ><span
          ><strong>{{ record.result.invocations }}</strong> component
          invocations</span
        >
      </div>
    </div>
    <p class="evidence-caption">
      A boolean <code>false</code> is a computed value, not an execution error.
      This pure evaluator reports no simulated latency or hardware cost.
    </p>
    <div class="evidence-actions">
      <button
        class="button secondary"
        :disabled="busy"
        @click="emit('load-snapshot')"
      >
        <Layers3 :size="14" /> Load recorded snapshot</button
      ><button
        class="button secondary"
        :disabled="stale || busy"
        @click="emit('capture-test')"
      >
        <FileCheck2 :size="14" /> Capture outputs as test
      </button>
    </div>
    <details class="snapshot-inputs">
      <summary>Recorded inputs & exact identity</summary>
      <dl>
        <div v-for="(value, name) in record.inputs" :key="name">
          <dt>{{ name }} · {{ value.type }}</dt>
          <dd>{{ valueText(value) }}</dd>
        </div>
      </dl>
      <code>Evaluation {{ record.id }}</code
      ><code>Root SHA-256 {{ record.result.root.digest }}</code
      ><code>Bundle SHA-256 {{ record.result.bundleHash }}</code>
    </details>
    <div class="trace-layout">
      <div class="behavior-trace">
        <table>
          <thead>
            <tr>
              <th>#</th>
              <th>PATH</th>
              <th>STEP</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="event in record.result.events"
              :key="event.sequence"
              :class="{ selected: selectedSequence === event.sequence }"
            >
              <td>{{ event.sequence }}</td>
              <td>
                <button
                  :aria-label="`Inspect trace event ${event.sequence} ${event.path}`"
                  @click="selectedSequence = event.sequence"
                >
                  {{ event.path }}
                </button>
              </td>
              <td>
                {{ event.kind.replaceAll("_", " ")
                }}<small>{{ event.op || event.component?.packageId }}</small>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="trace-detail">
        <template v-if="selectedEvent"
          ><span class="section-eyebrow"
            >STEP {{ selectedEvent.sequence }}</span
          >
          <h4>{{ selectedEvent.path }}</h4>
          <p>
            {{
              selectedEvent.op ||
              selectedEvent.component?.packageId ||
              selectedEvent.kind
            }}
          </p>
          <template
            v-for="(values, heading) in {
              Inputs: selectedEvent.inputs,
              Outputs: selectedEvent.outputs,
            }"
            :key="heading"
            ><strong>{{ heading }}</strong>
            <dl>
              <div v-for="(value, name) in values" :key="name">
                <dt>{{ name }} · {{ value.type }}</dt>
                <dd>{{ valueText(value) }}</dd>
              </div>
            </dl></template
          ></template
        >
        <p v-else>
          Select a trace step to inspect its typed inputs and outputs. Nested
          call paths identify the exact instance being executed.
        </p>
      </div>
    </div>
  </section>
  <section v-if="testResult" class="behavior-test-results">
    <div class="evidence-heading">
      <div>
        <span class="section-eyebrow">DEFINITION TESTS</span>
        <h3>
          {{
            testResult.cases.length
              ? `${testResult.cases.filter((c) => c.passed).length} / ${testResult.cases.length} passed`
              : "No test cases defined"
          }}
        </h3>
      </div>
      <span
        class="evidence-status"
        :class="{ stale: testsStale || !testResult.passed }"
        >{{
          testsStale
            ? "Definition has changed"
            : !testResult.cases.length
              ? "Not verified"
              : testResult.passed
                ? "All cases passed"
                : "Cases failed"
        }}</span
      >
    </div>
    <p v-if="testsStale" class="evidence-caption">
      These verdicts describe the definition tested earlier. Run the tests again
      for the current draft.
    </p>
    <div
      v-for="test in testResult.cases"
      :key="test.name"
      class="behavior-case-result"
    >
      <div>
        <CheckCircle2 v-if="test.passed" :size="16" /><CircleAlert
          v-else
          :size="16"
        /><strong>{{ test.name }}</strong
        ><span>{{ test.passed ? "PASS" : "FAIL" }}</span>
      </div>
      <p v-if="test.error" class="test-error">{{ test.error }}</p>
      <table>
        <thead>
          <tr>
            <th>OUTPUT</th>
            <th>EXPECTED</th>
            <th>ACTUAL</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(value, name) in test.expected" :key="name">
            <td>{{ name }}</td>
            <td>
              {{ valueText(value) }} <small>{{ value.type }}</small>
            </td>
            <td>
              {{ valueText(test.actual?.[name]) }}
              <small>{{ test.actual?.[name]?.type }}</small>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="evidence-caption">
      Test verdicts are shown for this session. Successful individual
      evaluations are stored separately in evaluation history.
    </p>
  </section>
</template>
<style scoped>
.behavior-evidence,
.behavior-test-results {
  background: white;
  border: 1px solid #e0e8ee;
  border-radius: 9px;
  padding: 20px;
  margin-top: 18px;
}
.evidence-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.evidence-heading h3 {
  font-size: 16px;
  color: #365368;
  margin: 7px 0 0;
}
.evidence-status {
  font-size: 12px;
  background: #edf7f1;
  color: #4d967a;
  padding: 7px 10px;
  border-radius: 5px;
}
.evidence-status.stale {
  color: #a48147;
  background: #fcf5e6;
}
.evidence-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  line-height: 1.7;
  color: #ab874d;
  background: #fff9ed;
  padding: 11px;
  border-radius: 5px;
}
.evaluation-identity {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  margin: 17px 0;
  font-size: 12px;
  color: #657e8f;
}
.evaluation-identity strong {
  color: #617f8c;
  font-weight: 500;
}
.evaluation-identity code {
  font-size: 12px;
  width: 100%;
}
.behavior-output-grid {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.behavior-output {
  border: 1px solid #d7e8df;
  background: #f7fcf9;
  border-radius: 7px;
  padding: 14px;
  flex: 1;
  min-width: 170px;
  overflow-wrap: anywhere;
}
.behavior-output > span {
  display: flex;
  justify-content: space-between;
  gap: 25px;
  font-size: 12px;
  color: #6e8c80;
}
.behavior-output small {
  font-size: 12px;
  color: #657e8f;
}
.behavior-output strong {
  display: block;
  color: #2e806b;
  font-family: monospace;
  font-size: 22px;
  margin-top: 10px;
  white-space: pre-wrap;
}
.evaluation-counts {
  display: grid;
  align-content: center;
  gap: 12px;
  font-size: 12px;
  color: #657e8f;
  padding: 12px;
}
.evaluation-counts strong {
  font-size: 17px;
  color: #627e8d;
  margin-right: 5px;
}
.evidence-caption {
  font-size: 12px;
  line-height: 1.75;
  color: #657e8f;
}
.evidence-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 13px 0;
}
.evidence-actions .button {
  font-size: 12px;
  padding: 8px 11px;
}
.snapshot-inputs {
  padding: 12px 0;
  color: #7c939f;
  font-size: 12px;
}
.snapshot-inputs summary {
  cursor: pointer;
}
.snapshot-inputs > code {
  display: block;
  font-size: 12px;
  margin-top: 8px;
  overflow-wrap: anywhere;
}
.snapshot-inputs dl {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
dl div {
  background: #f7fafb;
  border: 1px solid #ebf0f3;
  padding: 8px;
  border-radius: 4px;
}
dt {
  font-size: 12px;
  color: #657e8f;
}
dd {
  font-size: 12px;
  color: #58778b;
  margin: 5px 0 0;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}
.trace-layout {
  display: grid;
  grid-template-columns: 1.5fr 1fr;
  border: 1px solid #e4ebf0;
  border-radius: 7px;
  overflow: hidden;
  margin-top: 10px;
}
.behavior-trace {
  max-height: 350px;
  overflow: auto;
}
.behavior-trace table,
.behavior-case-result table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 12px;
}
th {
  padding: 9px;
  color: #657e8f;
  background: #f5f8fa;
  font-size: 12px;
  letter-spacing: 0.6px;
  position: sticky;
  top: 0;
}
td {
  padding: 10px;
  border-bottom: 1px solid #edf2f5;
  color: #758d9f;
}
td small {
  display: block;
  font-size: 12px;
  color: #657e8f;
  margin-top: 4px;
}
.behavior-trace button {
  color: #577f8f;
  font-family: monospace;
  font-size: 12px;
  background: none;
  padding: 0;
  text-align: left;
  overflow-wrap: anywhere;
}
.behavior-trace .selected {
  background: #eef9f5;
}
.trace-detail {
  border-left: 1px solid #e4ebf0;
  padding: 14px;
  max-height: 350px;
  overflow: auto;
}
.trace-detail h4 {
  font-size: 12px;
  color: #688596;
  margin: 9px 0;
  overflow-wrap: anywhere;
}
.trace-detail > p {
  font-size: 12px;
  color: #657e8f;
  line-height: 1.8;
  overflow-wrap: anywhere;
}
.trace-detail > strong {
  font-size: 12px;
  color: #657e8f;
}
.trace-detail dl {
  display: grid;
  gap: 5px;
}
.behavior-case-result {
  border: 1px solid #e8eef2;
  border-radius: 6px;
  overflow: hidden;
  margin-top: 12px;
}
.behavior-case-result > div {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  font-size: 12px;
  color: #629581;
  background: #f8fbfa;
}
.behavior-case-result > div > span {
  margin-left: auto;
  font-size: 12px;
}
.test-error {
  font-size: 12px;
  color: #b4775f;
  padding: 0 12px;
  line-height: 1.6;
}
@media (max-width: 700px) {
  .trace-layout {
    grid-template-columns: 1fr;
  }
  .trace-detail {
    border-left: 0;
    border-top: 1px solid #e4ebf0;
  }
  .evidence-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .behavior-evidence,
  .behavior-test-results {
    padding: 15px;
  }
}
</style>
