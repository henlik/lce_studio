<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { DeliveryRun } from "../deliveryTypes";
import StateValueMap from "./StateValueMap.vue";
const props = defineProps<{
  record: DeliveryRun;
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
const trace = computed(
  () => step.value?.state?.decision?.events[traceIndex.value],
);
</script>
<template>
  <div class="delivery-evidence">
    <section class="state-panel">
      <div class="state-panel-heading">
        <div>
          <span class="section-eyebrow">IMMUTABLE MESSAGE EXPERIMENT</span>
          <h3>
            {{
              record.result.verified
                ? record.result.passed
                  ? "Message expectations passed"
                  : "Message expectations failed"
                : "Unverified message scenario"
            }}
          </h3>
        </div>
        <span>{{
          stale
            ? "Earlier model or scenario"
            : "Matches current model and scenario"
        }}</span>
      </div>
      <p v-if="stale" class="state-warning">
        Model, layout, initial values, ordered steps, assertions or pending JSON
        differ from this saved snapshot. Its evidence has not changed.
      </p>
      <p class="state-help">
        {{ record.model.model.name }} ·
        {{ new Date(record.createdAt).toLocaleString() }} · engine
        {{ record.result.engineVersion }} ·
        {{ record.result.operations }} operations ·
        {{ record.result.invocations }} component invocations
      </p>
      <p v-if="!record.result.verified" class="state-warning">
        No independent owner or message assertions were supplied. Completion is
        unverified.
      </p>
      <div
        v-for="expected in record.result.expectations"
        :key="expected.instanceId"
        class="delivery-verdict"
      >
        <strong
          >Owner {{ expected.instanceId }} ·
          {{ expected.passed ? "PASS" : "FAIL" }}</strong
        >
        <ul>
          <li v-for="(issue, i) in expected.issues" :key="i">
            <code>{{ issue.path }}</code> {{ issue.message }}
          </li>
        </ul>
      </div>
      <div
        v-for="expected in record.result.messageExpectations"
        :key="expected.name"
        class="delivery-verdict"
      >
        <strong
          >Message {{ expected.name }} ·
          {{ expected.passed ? "PASS" : "FAIL" }}</strong
        >
        <ul>
          <li v-for="(issue, i) in expected.issues" :key="i">
            <code>{{ issue.path }}</code> {{ issue.message }}
          </li>
        </ul>
      </div>
      <button
        class="button secondary"
        :disabled="disabled"
        @click="emit('load-snapshot')"
      >
        Load recorded message snapshot
      </button>
      <details class="state-subsection">
        <summary>Recorded identities and scenario</summary>
        <code>Run {{ record.id }}</code
        ><br /><code>Model SHA-256 {{ record.result.modelHash }}</code
        ><br /><code>Scenario SHA-256 {{ record.result.scenarioHash }}</code>
        <pre>{{ JSON.stringify(record.scenario, null, 2) }}</pre>
      </details>
    </section>
    <section class="state-panel">
      <h3>Frozen messages & acknowledgements</h3>
      <p class="state-help">
        Transport acknowledgement records recipient processing. Business
        acceptance is the separate Boolean in the historical receipt. Frozen
        requests are never rebased to the recipient's current revision.
      </p>
      <div class="delivery-result-grid">
        <article
          v-for="message in record.result.messages"
          :key="message.id"
          class="state-card"
        >
          <h4>{{ message.name }}</h4>
          <div class="delivery-outcomes">
            <span
              ><small>TRANSPORT</small
              ><strong>{{ message.status }}</strong></span
            ><span
              ><small>BUSINESS</small
              ><strong>{{
                message.acknowledgement
                  ? message.acknowledgement.accepted
                    ? "accepted"
                    : "denied"
                  : "No acknowledgement yet"
              }}</strong></span
            >
          </div>
          <p class="state-help">
            {{ message.attempts }} delivery attempts ·
            {{ message.sourceInstanceId }} → {{ message.targetInstanceId }} ·
            route {{ message.routeId }}
          </p>
          <p class="state-help">
            Frozen command {{ message.command }} · expected revision
            {{ message.expectedRevision }}
          </p>
          <StateValueMap :values="message.inputs" />
          <details class="state-subsection">
            <summary>Frozen source and message identity</summary>
            <code>{{ message.id }}</code>
            <p class="state-help">
              Source command {{ message.sourceCommandId }} · effect
              {{ message.effect }}
            </p>
            <code>Intent {{ message.sourceIntentId }}</code>
          </details>
          <section v-if="message.acknowledgement" class="state-subsection">
            <h4>Historical acknowledgement receipt</h4>
            <p class="state-help">
              Original revision {{ message.acknowledgement.revision }} ·
              {{ message.acknowledgement.accepted ? "accepted" : "denied" }}.
              These outputs are the original reply, not the recipient's current
              state.
            </p>
            <StateValueMap :values="message.acknowledgement.outputs" />
            <p class="state-help">
              Original effect IDs:
              {{ message.acknowledgement.effectIds.join(", ") || "none" }}
            </p>
          </section>
        </article>
      </div>
      <p v-if="!record.result.messages.length" class="state-help">
        No envelope was materialized in this experiment.
      </p>
    </section>
    <section class="state-panel">
      <h3>Final machine-owned state</h3>
      <div class="delivery-result-grid">
        <article
          v-for="instance in record.result.instances"
          :key="instance.id"
          class="state-card"
        >
          <h4>{{ instance.id }}</h4>
          <p class="state-help">
            Current revision {{ instance.revision }} ·
            {{ instance.receipts.length }} receipts ·
            {{ instance.effects.length }} committed intentions
          </p>
          <StateValueMap :values="instance.state" />
          <details class="state-subsection">
            <summary>Owned receipts and effect intentions</summary>
            <article
              v-for="receipt in instance.receipts"
              :key="receipt.commandId"
              class="state-card"
            >
              <h4>{{ receipt.commandId }}</h4>
              <p class="state-help">
                Historical revision {{ receipt.revision }} ·
                {{ receipt.accepted ? "accepted" : "denied" }}
              </p>
              <StateValueMap :values="receipt.outputs" /><code
                >Request {{ receipt.requestHash }}</code
              >
            </article>
            <article
              v-for="effect in instance.effects"
              :key="effect.id"
              class="state-card"
            >
              <h4>Effect {{ effect.effect }}</h4>
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
      <h3>Ordered dispatch & delivery trace</h3>
      <div class="delivery-trace-layout">
        <div class="delivery-step-list">
          <button
            v-for="item in record.result.steps"
            :key="item.index"
            :class="{ selected: item.index === step?.index }"
            :aria-label="`Inspect message step ${item.index + 1}`"
            @click="selected = item.index"
          >
            <strong>{{ item.index + 1 }} · {{ item.kind }}</strong
            ><span
              >{{ item.messageName || item.instanceId }} ·
              {{ item.status.replaceAll("_", " ") }}</span
            ><small v-if="item.messageName"
              >Attempts {{ item.beforeAttempts }} →
              {{ item.afterAttempts }}</small
            >
          </button>
        </div>
        <article v-if="step" class="delivery-step-detail">
          <h4>
            Step {{ step.index + 1 }} · {{ step.status.replaceAll("_", " ") }}
          </h4>
          <p class="state-help">{{ step.message }}</p>
          <div v-if="step.messageName" class="delivery-outcomes">
            <span
              ><small>TRANSPORT</small
              ><strong
                >{{ step.beforeStatus || "absent" }} →
                {{ step.afterStatus || "absent" }}</strong
              ></span
            ><span
              ><small>ATTEMPTS</small
              ><strong
                >{{ step.beforeAttempts }} → {{ step.afterAttempts }}</strong
              ></span
            >
          </div>
          <p v-if="step.faultApplied" class="state-warning">
            This explicit failure injection reached its defined point.
          </p>
          <code v-if="step.messageId">{{ step.messageId }}</code
          ><template v-if="step.state"
            ><h4>
              Owner protocol · {{ step.state.status.replaceAll("_", " ") }}
            </h4>
            <p class="state-help">{{ step.state.message }}</p>
            <div class="delivery-result-grid">
              <section>
                <h4>
                  Current state before · revision
                  {{ step.state.beforeRevision }}
                </h4>
                <StateValueMap :values="step.state.before" />
              </section>
              <section>
                <h4>
                  Current state after · revision {{ step.state.afterRevision }}
                </h4>
                <StateValueMap :values="step.state.after" />
              </section>
            </div>
            <section v-if="step.state.receipt" class="state-card">
              <h4>
                Historical command receipt ·
                {{ step.state.receipt.accepted ? "accepted" : "denied" }}
              </h4>
              <p class="state-help">
                Original revision {{ step.state.receipt.revision }}. Receipt
                existence does not imply transport acknowledgement:
                acknowledgement loss can leave the message pending.
              </p>
              <StateValueMap :values="step.state.receipt.outputs" />
              <p v-if="step.state.status === 'replayed'" class="state-warning">
                The original receipt was replayed without restoring historical
                state, rerunning the decision or emitting another effect.
              </p>
            </section>
            <details v-if="step.state.decision" class="state-subsection">
              <summary>
                Pure decision trace ·
                {{ step.state.decision.operations }} operations
              </summary>
              <label class="field"
                >Decision event<select
                  v-model.number="traceIndex"
                  aria-label="Message decision trace event"
                >
                  <option
                    v-for="(event, i) in step.state.decision.events"
                    :key="i"
                    :value="i"
                  >
                    {{ event.sequence }} · {{ event.path }} ·
                    {{ event.op || event.kind }}
                  </option>
                </select></label
              ><template v-if="trace"
                ><h4>
                  {{ trace.path }} · {{ trace.kind.replaceAll("_", " ") }}
                </h4>
                <div class="delivery-result-grid">
                  <section>
                    <h4>Inputs</h4>
                    <StateValueMap :values="trace.inputs || {}" />
                  </section>
                  <section>
                    <h4>Outputs</h4>
                    <StateValueMap :values="trace.outputs || {}" />
                  </section></div
              ></template></details
          ></template>
          <p v-else class="state-help">
            No owner protocol execution occurred at this step. Dispatch and
            dropped delivery do not evaluate a recipient.
          </p>
        </article>
      </div>
    </section>
  </div>
</template>
<style scoped>
.delivery-result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}
.delivery-outcomes {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  margin: 12px 0;
}
.delivery-outcomes > span {
  display: grid;
  gap: 5px;
  border: 1px solid #dde9e7;
  background: #f1f8f5;
  padding: 10px 12px;
  border-radius: 6px;
  flex: 1;
  min-width: 150px;
  color: #457462;
  font-size: 12px;
}
.delivery-outcomes small {
  font-size: 10px;
  letter-spacing: 0.6px;
}
.delivery-trace-layout {
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
  gap: 20px;
}
.delivery-step-list {
  display: grid;
  align-content: start;
  gap: 7px;
  max-height: 700px;
  overflow: auto;
}
.delivery-step-list button {
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
.delivery-step-list button.selected {
  background: #e8f5ee;
  border-color: #80b7a2;
}
.delivery-step-list small {
  font-size: 10px;
  color: #748f9b;
}
.delivery-step-detail {
  min-width: 0;
}
.delivery-evidence code,
.delivery-evidence h4 {
  overflow-wrap: anywhere;
}
.delivery-evidence code {
  font-size: 11px;
}
.delivery-evidence pre {
  max-height: 350px;
  overflow: auto;
  background: #f7fafb;
  padding: 14px;
  font-size: 11px;
  line-height: 1.6;
}
.delivery-verdict {
  padding: 10px 0;
  font-size: 12px;
  color: #52748a;
}
.delivery-verdict ul {
  line-height: 1.8;
  margin-bottom: 0;
}
@media (max-width: 900px) {
  .delivery-trace-layout {
    grid-template-columns: 1fr;
  }
  .delivery-step-list {
    max-height: 250px;
  }
  .delivery-result-grid {
    grid-template-columns: 1fr;
  }
}
</style>
