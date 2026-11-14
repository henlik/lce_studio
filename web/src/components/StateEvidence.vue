<script setup lang="ts">
import ExecutionStepper from "./ExecutionStepper.vue";
import { computed, ref, watch } from "vue";
import type { StateRun } from "../stateTypes";
import StateValueMap from "./StateValueMap.vue";
const props = defineProps<{
  record: StateRun;
  stale: boolean;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "load-snapshot": [] }>();
const selected = ref(0),
  traceIndex = ref(0);
watch(
  () => props.record.id,
  () => {
    selected.value = 0;
    traceIndex.value = 0;
  },
);
watch(selected, () => (traceIndex.value = 0));
const step = computed(
  () =>
    props.record.result.steps.find((s) => s.index === selected.value) ||
    props.record.result.steps[0],
);
const trace = computed(() => step.value?.decision?.events[traceIndex.value]);
</script>
<template>
  <section class="state-evidence">
    <section class="state-panel">
      <div class="state-panel-heading">
        <div>
          <span class="section-eyebrow">IMMUTABLE STATE EXPERIMENT</span>
          <h3>
            {{
              record.result.verified
                ? record.result.passed
                  ? "Expectations passed"
                  : "Expectations failed"
                : "Unverified scenario"
            }}
          </h3>
        </div>
        <span>{{
          stale
            ? "Earlier machine or scenario"
            : "Matches current machine and scenario"
        }}</span>
      </div>
      <p v-if="stale" class="state-warning">
        The machine, initial values, ordered steps, expectations or pending JSON
        differ from this record. These results belong to its saved snapshot.
      </p>
      <p class="state-help">
        {{ record.machine.machine.name }} ·
        {{ new Date(record.createdAt).toLocaleString() }} · engine
        {{ record.result.engineVersion }} ·
        {{ record.result.operations }} primitive operations ·
        {{ record.result.invocations }} component invocations
      </p>
      <p v-if="!record.result.verified" class="state-warning">
        No independent final expectations were supplied. Completion is not a
        passing verification result.
      </p>
      <div
        v-for="expectation in record.result.expectations"
        :key="expectation.instanceId"
        class="state-verdict"
      >
        <strong
          >{{ expectation.instanceId }} ·
          {{ expectation.passed ? "PASS" : "FAIL" }}</strong
        >
        <ul>
          <li v-for="(issue, i) in expectation.issues" :key="i">
            <code>{{ issue.path }}</code> {{ issue.message }}
          </li>
        </ul>
      </div>
      <button
        class="button secondary"
        :disabled="disabled"
        @click="emit('load-snapshot')"
      >
        Load recorded state snapshot
      </button>
      <details class="state-subsection">
        <summary>Recorded identity and scenario</summary>
        <p class="state-help">Run {{ record.id }}</p>
        <code>Machine SHA-256 {{ record.result.machineHash }}</code
        ><br /><code>Scenario SHA-256 {{ record.result.scenarioHash }}</code>
        <pre>{{ JSON.stringify(record.scenario, null, 2) }}</pre>
      </details>
    </section>
    <section class="state-panel">
      <h3>Final owned state</h3>
      <div class="state-result-grid">
        <article
          v-for="instance in record.result.instances"
          :key="instance.id"
          class="state-card"
        >
          <h4>{{ instance.id }}</h4>
          <p class="state-help">
            Revision {{ instance.revision }} ·
            {{ instance.receipts.length }} durable receipts ·
            {{ instance.effects.length }} committed effect intentions
          </p>
          <StateValueMap :values="instance.state" />
          <details class="state-subsection">
            <summary>
              Historical receipts · {{ instance.receipts.length }}
            </summary>
            <article
              v-for="receipt in instance.receipts"
              :key="receipt.commandId"
              class="state-card"
            >
              <h4>
                {{ receipt.commandId }} ·
                {{ receipt.accepted ? "Accepted" : "Denied" }}
              </h4>
              <p class="state-help">
                Original reply revision {{ receipt.revision }} ·
                {{ receipt.command }}
              </p>
              <StateValueMap
                :values="receipt.outputs"
                empty="No public returned values"
              />
              <p class="state-help">
                Original effect IDs:
                {{ receipt.effectIds.join(", ") || "none" }}
              </p>
              <code>Request {{ receipt.requestHash }}</code>
            </article>
          </details>
          <details class="state-subsection">
            <summary>
              Committed effect intentions · {{ instance.effects.length }}
            </summary>
            <p class="state-help">
              These are recorded intentions, with no delivery or acknowledgement
              status.
            </p>
            <article
              v-for="effect in instance.effects"
              :key="effect.id"
              class="state-card"
            >
              <h4>{{ effect.effect }}</h4>
              <p class="state-help">
                Command {{ effect.commandId }} · revision {{ effect.revision }}
              </p>
              <code>{{ effect.id }}</code
              ><StateValueMap :values="effect.payload" />
            </article>
          </details>
        </article>
      </div>
    </section>
    <section class="state-panel">
      <h3>Ordered transition trace</h3>
      <ExecutionStepper :steps="record.result.steps" :selected="selected" @select="selected = $event" />
      <div class="state-trace-layout">
        <div class="state-step-list">
          <button
            v-for="item in record.result.steps"
            :key="item.index"
            :class="{ selected: item.index === step?.index }"
            :aria-label="`Inspect state step ${item.index + 1}`"
            @click="selected = item.index"
          >
            <strong>{{ item.index + 1 }} · {{ item.instanceId }}</strong
            ><span
              >{{ item.kind === "restart" ? "Restart" : item.commandId }} ·
              {{ item.status.replaceAll("_", " ") }}</span
            ><small
              >Revision {{ item.beforeRevision }} → {{ item.afterRevision
              }}{{ item.replyLost ? " · reply lost" : "" }}</small
            >
          </button>
        </div>
        <article v-if="step" class="state-step-detail">
          <h4>
            Step {{ step.index + 1 }} · {{ step.status.replaceAll("_", " ") }}
          </h4>
          <p class="state-help">{{ step.message }}</p>
          <p v-if="step.replyLost" class="state-warning">
            The reply was lost after commit. Owned state, receipt and effect
            intentions were committed.
          </p>
          <p v-if="step.faultApplied" class="state-help">
            The injected failure point was reached for this step.
          </p>
          <div class="state-result-grid">
            <section>
              <h4>Current state before · revision {{ step.beforeRevision }}</h4>
              <StateValueMap :values="step.before" />
            </section>
            <section>
              <h4>Current state after · revision {{ step.afterRevision }}</h4>
              <StateValueMap :values="step.after" />
            </section>
          </div>
          <section v-if="step.receipt" class="state-card">
            <h4>
              {{
                step.status === "replayed"
                  ? "Replayed historical receipt"
                  : "Committed receipt"
              }}
            </h4>
            <p class="state-help">
              Original command {{ step.receipt.commandId }} · original revision
              {{ step.receipt.revision }} ·
              {{ step.receipt.accepted ? "accepted" : "denied" }}
            </p>
            <p v-if="step.status === 'replayed'" class="state-warning">
              These are the original reply values. Replay did not re-evaluate
              the handler, restore old state or emit another effect. Current
              owned state is shown above.
            </p>
            <StateValueMap :values="step.receipt.outputs" />
            <p class="state-help">
              Original effect IDs:
              {{ step.receipt.effectIds.join(", ") || "none" }}
            </p>
          </section>
          <details v-if="step.decision" class="state-subsection">
            <summary>
              Pure handler execution · {{ step.decision.operations }} operations
            </summary>
            <label class="field"
              >Decision trace event<select
                v-model.number="traceIndex"
                aria-label="State decision trace event"
              >
                <option
                  v-for="(event, i) in step.decision.events"
                  :key="i"
                  :value="i"
                >
                  {{ event.sequence }} · {{ event.path }} ·
                  {{ event.op || event.kind }}
                </option>
              </select></label
            ><template v-if="trace"
              ><h4>{{ trace.path }} · {{ trace.kind.replaceAll("_", " ") }}</h4>
              <div class="state-result-grid">
                <section>
                  <h4>Decision inputs</h4>
                  <StateValueMap :values="trace.inputs || {}" />
                </section>
                <section>
                  <h4>Decision outputs</h4>
                  <StateValueMap :values="trace.outputs || {}" />
                </section></div
            ></template>
          </details>
          <p v-else class="state-help">
            No successful pure decision trace was produced for this step.
            Replays and revision/ID conflicts do not evaluate the handler.
          </p>
        </article>
      </div>
    </section>
  </section>
</template>
<style scoped>
.state-result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 16px;
}
.state-trace-layout {
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
  gap: 20px;
}
.state-step-list {
  display: grid;
  align-content: start;
  gap: 7px;
  max-height: 700px;
  overflow: auto;
}
.state-step-list button {
  display: grid;
  gap: 6px;
  text-align: left;
  padding: 12px;
  color: #52758a;
  background: #f8fbfc;
  border: 1px solid #dfe9ee;
  border-radius: 6px;
  font-size: 12px;
}
.state-step-list button.selected {
  background: #e8f5ee;
  border-color: #80b7a2;
}
.state-step-list small {
  font-size: 10px;
  color: #748f9b;
}
.state-step-detail {
  min-width: 0;
}
.state-evidence code {
  font-size: 11px;
  overflow-wrap: anywhere;
}
.state-evidence pre {
  max-height: 350px;
  overflow: auto;
  background: #f7fafb;
  padding: 14px;
  font-size: 11px;
  line-height: 1.6;
}
.state-verdict {
  padding: 12px 0;
  font-size: 12px;
  color: #52748a;
}
.state-verdict ul {
  line-height: 1.8;
  margin-bottom: 0;
}
@media (max-width: 900px) {
  .state-trace-layout {
    grid-template-columns: 1fr;
  }
  .state-step-list {
    max-height: 250px;
  }
  .state-result-grid {
    grid-template-columns: 1fr;
  }
}
</style>
