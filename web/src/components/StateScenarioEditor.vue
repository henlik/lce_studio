<script setup lang="ts">
import { ref, watch } from "vue";
import { cloneBehavior, typedDefault, type TypedValue } from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import {
  freshStateName,
  stateUIKey,
  type StateMachine,
  type StateScenario,
  type StateStep,
  type StateInstance,
} from "../stateTypes";
import TypedValueEditor from "./TypedValueEditor.vue";
import StateNumberEditor from "./StateNumberEditor.vue";
const props = defineProps<{
  machine: StateMachine;
  scenario: StateScenario;
  contracts: DataContractLock[];
  disabled?: boolean;
}>();
const openStep = ref(0);
watch(
  () => props.scenario,
  () => (openStep.value = 0),
);
function toggleStep(index: number, e: Event) {
  if ((e.target as HTMLDetailsElement).open) openStep.value = index;
  else if (openStep.value === index) openStep.value = -1;
}
function put(
  map: Record<string, TypedValue>,
  name: string,
  value?: TypedValue,
) {
  if (value) map[name] = value;
  else delete map[name];
}
function addInstance() {
  const id = freshStateName(
    "instance",
    props.scenario.instances.map((i) => i.id),
  );
  props.scenario.instances.push({ id, name: id, initial: {} });
}
function override(instance: StateInstance, name: string, checked: boolean) {
  const slot = props.machine.state.find((s) => s.name === name);
  if (checked && slot) instance.initial[name] = cloneBehavior(slot.initial);
  else delete instance.initial[name];
}
function addStep(kind: "command" | "restart") {
  openStep.value = props.scenario.steps.length;
  const instanceId = props.scenario.instances[0]?.id || "";
  if (kind === "restart") {
    props.scenario.steps.push({ kind, instanceId, inputs: null });
    return;
  }
  const command = props.machine.commands[0];
  props.scenario.steps.push({
    kind,
    instanceId,
    commandId: freshStateName(
      "command",
      props.scenario.steps.map((s) => s.commandId || ""),
    ),
    command: command?.name || "",
    expectedRevision: 0,
    inputs: Object.fromEntries(
      (command?.inputs || []).map((p) => [
        p.name,
        typedDefault(p.type, p.recordRef, props.contracts),
      ]),
    ),
  });
}
function move(index: number, delta: number) {
  const target = index + delta;
  if (target < 0 || target >= props.scenario.steps.length) return;
  const [step] = props.scenario.steps.splice(index, 1);
  props.scenario.steps.splice(target, 0, step);
  openStep.value = target;
}
function stepCommand(step: StateStep) {
  return props.machine.commands.find((c) => c.name === step.command);
}
function addExpectation(instance: StateInstance) {
  props.scenario.expected.push({
    instanceId: instance.id,
    state: Object.fromEntries(
      props.machine.state.map((s) => [
        s.name,
        cloneBehavior(instance.initial[s.name] || s.initial),
      ]),
    ),
    revision: 0,
    receipts: 0,
    effects: 0,
  });
}
function unknown(map: Record<string, unknown>, names: string[]) {
  return Object.keys(map).filter((k) => !names.includes(k));
}
</script>
<template>
  <div class="state-scenario">
    <section class="state-panel">
      <label class="field"
        >Scenario name<input
          v-model="scenario.name"
          aria-label="State scenario name"
          :disabled="disabled"
      /></label>
      <p class="state-help">
        Each run initializes independent instances, then executes this list in
        order. Reorder steps deliberately: revisions, receipts and cached
        replies depend on that order.
      </p>
    </section>
    <section class="state-panel">
      <div class="state-panel-heading">
        <h3>Instances & initial overrides</h3>
        <button
          class="button secondary"
          :disabled="disabled || scenario.instances.length >= 8"
          @click="addInstance"
        >
          Add state instance
        </button>
      </div>
      <fieldset :disabled="disabled">
        <article
          v-for="(instance, i) in scenario.instances"
          :key="stateUIKey(instance)"
          class="state-card"
        >
          <div class="state-grid">
            <label class="field"
              >Instance ID<input
                v-model="instance.id"
                :aria-label="`Instance ${i + 1} ID`" /></label
            ><label class="field"
              >Display name<input
                v-model="instance.name"
                :aria-label="`Instance ${i + 1} name`"
            /></label>
          </div>
          <div
            v-for="slot in machine.state"
            :key="slot.name"
            class="state-override"
          >
            <label class="state-check"
              ><input
                type="checkbox"
                :checked="Object.hasOwn(instance.initial, slot.name)"
                :aria-label="`Override ${instance.id}.${slot.name}`"
                @change="
                  override(
                    instance,
                    slot.name,
                    ($event.target as HTMLInputElement).checked,
                  )
                "
              />
              Override {{ slot.name }}
              <small>{{ slot.durability }}</small></label
            ><TypedValueEditor
              v-if="Object.hasOwn(instance.initial, slot.name)"
              :model-value="instance.initial[slot.name]"
              :type="slot.type"
              :record-ref="slot.recordRef"
              :contracts="contracts"
              :label="`Initial ${instance.id}.${slot.name}`"
              :disabled="disabled"
              @update:model-value="put(instance.initial, slot.name, $event)"
            />
            <p v-else class="state-help">
              Use the machine default. A volatile slot returns to this effective
              initial value on restart.
            </p>
          </div>
          <div
            v-for="name in unknown(
              instance.initial,
              machine.state.map((s) => s.name),
            )"
            :key="name"
            class="state-warning"
          >
            Unresolved initial slot: {{ name }}
            <button class="button quiet" @click="delete instance.initial[name]">
              Remove unresolved override
            </button>
          </div>
          <button
            class="button quiet"
            :disabled="scenario.instances.length <= 1"
            @click="scenario.instances.splice(i, 1)"
          >
            Remove instance {{ i + 1 }}
          </button>
        </article>
      </fieldset>
    </section>
    <section class="state-panel">
      <div class="state-panel-heading">
        <h3>Ordered steps · {{ scenario.steps.length }} / 64</h3>
        <div class="state-actions">
          <button
            class="button secondary"
            :disabled="
              disabled ||
              scenario.steps.length >= 64 ||
              !machine.commands.length
            "
            @click="addStep('command')"
          >
            Add command step</button
          ><button
            class="button secondary"
            :disabled="disabled || scenario.steps.length >= 64"
            @click="addStep('restart')"
          >
            Add restart step
          </button>
        </div>
      </div>
      <p class="state-help">
        Expected revision rejects a stale writer: two new requests at the same
        revision cannot both change the state. Scenarios simulate an explicit
        ordering; they do not run threads in parallel. A repeated command ID
        replays its original receipt only when command, expected revision and
        inputs match. Faults apply only when a new commit reaches their
        injection point.
      </p>
      <fieldset :disabled="disabled">
        <details
          v-for="(step, index) in scenario.steps"
          :key="stateUIKey(step)"
          class="state-card state-step-card"
          :open="index === openStep"
          @toggle="toggleStep(index, $event)"
        >
          <summary>
            Step {{ index + 1 }} · {{ step.instanceId }} ·
            {{
              step.kind === "restart"
                ? "Restart"
                : `${step.command} (${step.commandId})`
            }}
            <small v-if="step.fault"
              >· {{ step.fault.replaceAll("_", " ") }}</small
            >
          </summary>
          <div class="state-panel-heading">
            <h4>
              Step {{ index + 1 }} ·
              {{
                step.kind === "restart" ? "Restart instance" : "Submit command"
              }}
            </h4>
            <div class="state-actions">
              <button
                class="button quiet"
                :disabled="index === 0"
                :aria-label="`Move step ${index + 1} up`"
                @click="move(index, -1)"
              >
                ↑ Up</button
              ><button
                class="button quiet"
                :disabled="index === scenario.steps.length - 1"
                :aria-label="`Move step ${index + 1} down`"
                @click="move(index, 1)"
              >
                ↓ Down</button
              ><button
                v-if="step.kind === 'command'"
                class="button quiet"
                :disabled="
                  scenario.steps.length >= 64 ||
                  !Number.isSafeInteger(step.expectedRevision) ||
                  step.expectedRevision! < 0
                "
                :aria-label="`Duplicate step ${index + 1}`"
                @click="
                  scenario.steps.splice(index + 1, 0, cloneBehavior(step));
                  openStep = index + 1;
                "
              >
                Duplicate for retry</button
              ><button
                class="button quiet"
                :disabled="scenario.steps.length <= 1"
                :aria-label="`Remove step ${index + 1}`"
                @click="scenario.steps.splice(index, 1)"
              >
                Remove
              </button>
            </div>
          </div>
          <div class="state-grid">
            <label class="field"
              >Target instance<select
                v-model="step.instanceId"
                :aria-label="`Step ${index + 1} instance`"
              >
                <option
                  v-if="
                    !scenario.instances.some((i) => i.id === step.instanceId)
                  "
                  :value="step.instanceId"
                >
                  Unresolved: {{ step.instanceId || "choose instance" }}
                </option>
                <option
                  v-for="instance in scenario.instances"
                  :key="instance.id"
                  :value="instance.id"
                >
                  {{ instance.name }} · {{ instance.id }}
                </option>
              </select></label
            ><template v-if="step.kind === 'command'"
              ><label class="field"
                >Command<select
                  v-model="step.command"
                  :aria-label="`Step ${index + 1} command`"
                >
                  <option
                    v-if="
                      !machine.commands.some((c) => c.name === step.command)
                    "
                    :value="step.command"
                  >
                    Unresolved: {{ step.command || "choose command" }}
                  </option>
                  <option v-for="c in machine.commands" :key="c.name">
                    {{ c.name }}
                  </option>
                </select></label
              ><label class="field"
                >Command ID<input
                  v-model="step.commandId"
                  :aria-label="`Step ${index + 1} command ID`" /></label
              ><StateNumberEditor
                v-model="step.expectedRevision"
                :label="`Step ${index + 1} expected revision`"
                :disabled="disabled"
              /><label class="field"
                >Failure injection<select
                  v-model="step.fault"
                  :aria-label="`Step ${index + 1} failure`"
                >
                  <option :value="undefined">No injected failure</option>
                  <option value="before_commit">
                    Before commit · roll back staged work
                  </option>
                  <option value="after_commit_before_reply">
                    After commit · lose reply
                  </option>
                </select></label
              ></template
            >
          </div>
          <p v-if="step.kind === 'restart'" class="state-help">
            Preserve durable state, current revision, receipts and effect
            intentions. Reset only this instance's volatile slots to their
            effective initial values.
          </p>
          <template v-else
            ><p v-if="step.fault === 'before_commit'" class="state-help">
              This aborts staged writes, receipt and effects. It does not
              restart the instance; add a separate restart step if needed.
            </p>
            <p
              v-if="step.fault === 'after_commit_before_reply'"
              class="state-help"
            >
              The transaction commits but its reply is lost. A matching retry
              can retrieve the receipt without producing a second effect.
            </p>
            <div class="state-grid">
              <TypedValueEditor
                v-for="port in stepCommand(step)?.inputs || []"
                :key="port.name"
                :model-value="step.inputs?.[port.name]"
                :type="port.type"
                :record-ref="port.recordRef"
                :contracts="contracts"
                :label="`Step ${index + 1} input ${port.name}`"
                :disabled="disabled"
                @update:model-value="
                  put((step.inputs ||= {}), port.name, $event)
                "
              />
            </div>
            <div
              v-for="name in unknown(
                step.inputs || {},
                (stepCommand(step)?.inputs || []).map((p) => p.name),
              )"
              :key="name"
              class="state-warning"
            >
              Unresolved command input: {{ name }}
              <button class="button quiet" @click="delete step.inputs![name]">
                Remove unresolved input
              </button>
            </div></template
          >
        </details>
      </fieldset>
    </section>
    <section class="state-panel">
      <div class="state-panel-heading">
        <h3>Independent final expectations</h3>
        <span>{{
          scenario.expected.length
            ? "Verification enabled"
            : "Unverified until expectations are added"
        }}</span>
      </div>
      <p class="state-help">
        Enter the final state, revision, receipt count and effect count you
        expect. New expectations start from initial values and zero counts for
        you to review; they never copy run results.
      </p>
      <div class="state-actions">
        <button
          v-for="instance in scenario.instances"
          :key="instance.id"
          class="button secondary"
          :disabled="
            disabled ||
            scenario.expected.some((e) => e.instanceId === instance.id)
          "
          @click="addExpectation(instance)"
        >
          Add expectation for {{ instance.id }}
        </button>
      </div>
      <fieldset :disabled="disabled">
        <article
          v-for="(expected, i) in scenario.expected"
          :key="stateUIKey(expected)"
          class="state-card"
        >
          <h4>Expected final instance · {{ expected.instanceId }}</h4>
          <p
            v-if="!scenario.instances.some((x) => x.id === expected.instanceId)"
            class="state-warning"
          >
            This expectation names an unresolved instance. Remove it and add an
            expectation for a current instance.
          </p>
          <div class="state-grid">
            <StateNumberEditor
              v-model="expected.revision"
              :label="`Expected ${expected.instanceId} revision`"
              :disabled="disabled"
            /><StateNumberEditor
              v-model="expected.receipts"
              :label="`Expected ${expected.instanceId} receipts`"
              :disabled="disabled"
            /><StateNumberEditor
              v-model="expected.effects"
              :label="`Expected ${expected.instanceId} effects`"
              :disabled="disabled"
            />
          </div>
          <div class="state-grid">
            <TypedValueEditor
              v-for="slot in machine.state"
              :key="slot.name"
              :model-value="expected.state[slot.name]"
              :type="slot.type"
              :record-ref="slot.recordRef"
              :contracts="contracts"
              :label="`Expected ${expected.instanceId}.${slot.name}`"
              :disabled="disabled"
              @update:model-value="put(expected.state, slot.name, $event)"
            />
          </div>
          <div
            v-for="name in unknown(
              expected.state,
              machine.state.map((s) => s.name),
            )"
            :key="name"
            class="state-warning"
          >
            Unresolved expected slot: {{ name }}
            <button class="button quiet" @click="delete expected.state[name]">
              Remove unresolved expected state
            </button>
          </div>
          <button class="button quiet" @click="scenario.expected.splice(i, 1)">
            Remove expectation for {{ expected.instanceId }}
          </button>
        </article>
      </fieldset>
    </section>
  </div>
</template>
<style scoped>
.state-step-card > summary {
  cursor: pointer;
  color: #4f7589;
  font-size: 13px;
  line-height: 1.8;
}
.state-step-card > summary small {
  font-size: 11px;
  color: #a97d54;
}
.state-step-card[open] > summary {
  margin-bottom: 16px;
}
.state-override {
  border-top: 1px solid #e1e9ed;
  padding: 10px 0;
}
.state-override .state-check small {
  margin-left: auto;
  color: #6a8692;
}
</style>
