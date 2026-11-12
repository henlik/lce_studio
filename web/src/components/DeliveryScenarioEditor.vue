<script setup lang="ts">
import { ref, watch } from "vue";
import { cloneBehavior, typedDefault, type TypedValue } from "../behaviorTypes";
import type { DataContractLock } from "../dataTypes";
import { stateUIKey, freshStateName } from "../stateTypes";
import {
  instanceMachine,
  type DeliveryModel,
  type DeliveryScenario,
  type DeliveryStep,
  type DeliveryInitial,
} from "../deliveryTypes";
import TypedValueEditor from "./TypedValueEditor.vue";
import StateNumberEditor from "./StateNumberEditor.vue";
const props = defineProps<{
  model: DeliveryModel;
  scenario: DeliveryScenario;
  contracts: DataContractLock[];
  disabled?: boolean;
}>();
const openStep = ref(0);
watch(
  () => props.scenario,
  () => (openStep.value = 0),
);
function toggle(index: number, e: Event) {
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
function unknown(map: Record<string, unknown>, names: string[]) {
  return Object.keys(map).filter((k) => !names.includes(k));
}
function addInitial(instanceId: string) {
  props.scenario.initial.push({ instanceId, initial: {} });
}
function override(initial: DeliveryInitial, name: string, checked: boolean) {
  const slot = instanceMachine(props.model, initial.instanceId)?.state.find(
    (s) => s.name === name,
  );
  if (checked && slot) initial.initial[name] = cloneBehavior(slot.initial);
  else delete initial.initial[name];
}
function earlierAliases(index: number) {
  const aliases = new Map<string, DeliveryStep>();
  for (const step of props.scenario.steps.slice(0, index))
    if (step.kind === "dispatch" && step.message && !aliases.has(step.message))
      aliases.set(step.message, step);
  return [...aliases].map(([name, step]) => ({
    name,
    route: props.model.routes.find((r) => r.id === step.routeId),
  }));
}
function allAliases() {
  return [
    ...new Set(
      props.scenario.steps
        .filter((s) => s.kind === "dispatch")
        .map((s) => s.message || "")
        .filter(Boolean),
    ),
  ];
}
function route(step: DeliveryStep) {
  return props.model.routes.find((r) => r.id === step.routeId);
}
function command(step: DeliveryStep) {
  return instanceMachine(props.model, step.instanceId)?.commands.find(
    (c) => c.name === step.command,
  );
}
function addStep(kind: DeliveryStep["kind"]) {
  const index = props.scenario.steps.length;
  openStep.value = index;
  const instance = props.model.instances[0];
  if (kind === "restart") {
    props.scenario.steps.push({
      kind,
      instanceId: instance?.id || "",
      inputs: null,
    });
    return;
  }
  if (kind === "deliver") {
    props.scenario.steps.push({
      kind,
      message: earlierAliases(index).at(-1)?.name || "",
      inputs: null,
    });
    return;
  }
  if (kind === "dispatch") {
    const route = props.model.routes[0];
    const sourceCommandId =
      props.scenario.steps.find(
        (s) => s.kind === "command" && s.instanceId === route?.sourceInstanceId,
      )?.commandId || "command1";
    const existing = props.scenario.steps.find(
      (s) =>
        s.kind === "dispatch" &&
        s.routeId === route?.id &&
        s.sourceCommandId === sourceCommandId &&
        s.sourceMessage === undefined,
    );
    props.scenario.steps.push({
      kind,
      routeId: route?.id || "",
      // A route/source pair has exactly one alias; adding its dispatch again is a redispatch.
      message:
        existing?.message ||
        freshStateName("message", [
          ...allAliases(),
          ...props.scenario.expectedMessages.map((e) => e.name),
        ]),
      sourceCommandId,
      expectedRevision:
        existing &&
        Number.isSafeInteger(existing.expectedRevision) &&
        existing.expectedRevision! >= 0
          ? existing.expectedRevision
          : 0,
      inputs: null,
    });
    return;
  }
  const cmd = instanceMachine(props.model, instance?.id)?.commands[0];
  props.scenario.steps.push({
    kind,
    instanceId: instance?.id || "",
    commandId: freshStateName(
      "command",
      props.scenario.steps.map((s) => s.commandId || ""),
    ),
    command: cmd?.name || "",
    expectedRevision: 0,
    inputs: Object.fromEntries(
      (cmd?.inputs || []).map((p) => [
        p.name,
        typedDefault(p.type, p.recordRef, props.contracts),
      ]),
    ),
  });
}
function changeSource(step: DeliveryStep, e: Event) {
  if ((e.target as HTMLSelectElement).value === "message") {
    delete step.sourceCommandId;
    step.sourceMessage = "";
  } else {
    delete step.sourceMessage;
    step.sourceCommandId = "";
  }
}
function move(index: number, delta: number) {
  const target = index + delta;
  if (target < 0 || target >= props.scenario.steps.length) return;
  const [step] = props.scenario.steps.splice(index, 1);
  props.scenario.steps.splice(target, 0, step);
  openStep.value = target;
}
function duplicable(step: DeliveryStep) {
  return (
    !["command", "dispatch"].includes(step.kind) ||
    (Number.isSafeInteger(step.expectedRevision) && step.expectedRevision! >= 0)
  );
}
function duplicate(index: number) {
  const step = props.scenario.steps[index];
  if (!duplicable(step)) return;
  props.scenario.steps.splice(index + 1, 0, cloneBehavior(step));
  openStep.value = index + 1;
}
function addExpected(instanceId: string) {
  const machine = instanceMachine(props.model, instanceId);
  if (!machine) return;
  const initial = props.scenario.initial.find(
    (i) => i.instanceId === instanceId,
  );
  props.scenario.expected.push({
    instanceId,
    state: Object.fromEntries(
      machine.state.map((s) => [
        s.name,
        cloneBehavior(initial?.initial[s.name] || s.initial),
      ]),
    ),
    revision: 0,
    receipts: 0,
    effects: 0,
  });
}
function addMessageExpected() {
  const declaredName = allAliases().find(
    (name) => !props.scenario.expectedMessages.some((e) => e.name === name),
  );
  const name =
    declaredName ||
    freshStateName("message", [
      ...allAliases(),
      ...props.scenario.expectedMessages.map((e) => e.name),
    ]);
  props.scenario.expectedMessages.push({
    name,
    status: declaredName ? "pending" : "absent",
    attempts: 0,
  });
}
function label(step: DeliveryStep) {
  if (step.kind === "command")
    return `${step.instanceId} · ${step.command} (${step.commandId})`;
  if (step.kind === "restart") return `${step.instanceId} · restart`;
  if (step.kind === "dispatch")
    return `${step.message} · dispatch on ${step.routeId}`;
  return `${step.message} · deliver`;
}
</script>
<template>
  <div>
    <section class="state-panel">
      <label class="field"
        >Scenario name<input
          v-model="scenario.name"
          aria-label="Message scenario name"
          :disabled="disabled"
      /></label>
      <p class="state-help">
        List every command, dispatch, delivery and restart. Dispatch freezes
        recipient inputs and expected revision; a retry never silently rebases
        that revision. No queues drain automatically.
      </p>
    </section>
    <section class="state-panel">
      <h3>Per-instance initialization</h3>
      <p class="state-help">
        Every model instance needs one initialization entry. Partial overrides
        fill from its exact machine defaults; volatile slots reset to these
        effective values on restart.
      </p>
      <div class="state-actions">
        <button
          v-for="instance in model.instances.filter(
            (instance) =>
              !scenario.initial.some((i) => i.instanceId === instance.id),
          )"
          :key="instance.id"
          class="button secondary"
          :disabled="disabled"
          @click="addInitial(instance.id)"
        >
          Add initialization for {{ instance.id }}
        </button>
      </div>
      <fieldset :disabled="disabled">
        <details
          v-for="(initial, index) in scenario.initial"
          :key="stateUIKey(initial)"
          class="state-card"
          :open="index === 0"
        >
          <summary>Initial values · {{ initial.instanceId }}</summary>
          <label class="field"
            >Instance<select
              v-model="initial.instanceId"
              :aria-label="`Initialization ${index + 1} instance`"
            >
              <option
                v-if="!model.instances.some((i) => i.id === initial.instanceId)"
                :value="initial.instanceId"
              >
                Unresolved: {{ initial.instanceId }}
              </option>
              <option
                v-for="instance in model.instances"
                :key="instance.id"
                :value="instance.id"
              >
                {{ instance.id }} · {{ instance.name }}
              </option>
            </select></label
          >
          <div
            v-for="slot in instanceMachine(model, initial.instanceId)?.state ||
            []"
            :key="slot.name"
            class="delivery-override"
          >
            <label class="state-check"
              ><input
                type="checkbox"
                :checked="Object.hasOwn(initial.initial, slot.name)"
                :aria-label="`Override message ${initial.instanceId}.${slot.name}`"
                @change="
                  override(
                    initial,
                    slot.name,
                    ($event.target as HTMLInputElement).checked,
                  )
                "
              />
              Override {{ slot.name }} · {{ slot.durability }}</label
            ><TypedValueEditor
              v-if="Object.hasOwn(initial.initial, slot.name)"
              :model-value="initial.initial[slot.name]"
              :type="slot.type"
              :record-ref="slot.recordRef"
              :contracts="contracts"
              :label="`Message initial ${initial.instanceId}.${slot.name}`"
              :disabled="disabled"
              @update:model-value="put(initial.initial, slot.name, $event)"
            />
            <p v-else class="state-help">Use this owner's declared default.</p>
          </div>
          <div
            v-for="name in unknown(
              initial.initial,
              (instanceMachine(model, initial.instanceId)?.state || []).map(
                (s) => s.name,
              ),
            )"
            :key="name"
            class="state-warning"
          >
            Unresolved initial slot: {{ name }}
            <button class="button quiet" @click="delete initial.initial[name]">
              Remove unresolved override
            </button>
          </div>
          <button
            class="button quiet"
            @click="scenario.initial.splice(index, 1)"
          >
            Remove initialization entry
          </button>
        </details>
      </fieldset>
    </section>
    <section class="state-panel">
      <div class="state-panel-heading">
        <h3>Ordered message steps · {{ scenario.steps.length }} / 64</h3>
        <div class="state-actions">
          <button
            v-for="kind in [
              'command',
              'dispatch',
              'deliver',
              'restart',
            ] as const"
            :key="kind"
            class="button secondary"
            :disabled="
              disabled ||
              scenario.steps.length >= 64 ||
              (kind === 'dispatch' && !model.routes.length)
            "
            @click="addStep(kind)"
          >
            Add {{ kind }} step
          </button>
        </div>
      </div>
      <p class="state-help">
        Message aliases name explicit dispatch declarations. A downstream
        dispatch can use an earlier message alias as its source after that
        message's recipient has committed an effect, even if its acknowledgement
        was lost.
      </p>
      <fieldset :disabled="disabled">
        <details
          v-for="(step, index) in scenario.steps"
          :key="stateUIKey(step)"
          class="state-card delivery-step"
          :open="index === openStep"
          @toggle="toggle(index, $event)"
        >
          <summary>
            Step {{ index + 1 }} · {{ label(step) }}
            <small v-if="step.fault"
              >· {{ step.fault.replaceAll("_", " ") }}</small
            >
          </summary>
          <div class="state-actions">
            <button
              class="button quiet"
              :disabled="index === 0"
              :aria-label="`Move message step ${index + 1} up`"
              @click="move(index, -1)"
            >
              ↑ Up</button
            ><button
              class="button quiet"
              :disabled="index === scenario.steps.length - 1"
              :aria-label="`Move message step ${index + 1} down`"
              @click="move(index, 1)"
            >
              ↓ Down</button
            ><button
              class="button quiet"
              :disabled="scenario.steps.length >= 64 || !duplicable(step)"
              :aria-label="`Duplicate message step ${index + 1}`"
              @click="duplicate(index)"
            >
              Duplicate step</button
            ><button
              class="button quiet"
              :disabled="scenario.steps.length <= 1"
              :aria-label="`Remove message step ${index + 1}`"
              @click="scenario.steps.splice(index, 1)"
            >
              Remove step
            </button>
          </div>
          <div class="state-grid">
            <label
              v-if="step.kind === 'command' || step.kind === 'restart'"
              class="field"
              >Owner instance<select
                v-model="step.instanceId"
                :aria-label="`Message step ${index + 1} instance`"
              >
                <option
                  v-if="!model.instances.some((i) => i.id === step.instanceId)"
                  :value="step.instanceId"
                >
                  Unresolved: {{ step.instanceId || "choose owner" }}
                </option>
                <option
                  v-for="instance in model.instances"
                  :key="instance.id"
                  :value="instance.id"
                >
                  {{ instance.id }}
                </option>
              </select></label
            >
            <template v-if="step.kind === 'command'"
              ><label class="field"
                >Command<select
                  v-model="step.command"
                  :aria-label="`Message step ${index + 1} command`"
                >
                  <option v-if="!command(step)" :value="step.command">
                    Choose command: {{ step.command }}
                  </option>
                  <option
                    v-for="c in instanceMachine(model, step.instanceId)
                      ?.commands || []"
                    :key="c.name"
                  >
                    {{ c.name }}
                  </option>
                </select></label
              ><label class="field"
                >Direct command ID<input
                  v-model="step.commandId"
                  :aria-label="`Message step ${index + 1} command ID`"
                /><small>Do not use the reserved msg_ prefix.</small></label
              ><StateNumberEditor
                v-model="step.expectedRevision"
                :label="`Message step ${index + 1} expected revision`"
                :disabled="disabled"
              /><label class="field"
                >Command failure<select
                  v-model="step.fault"
                  :aria-label="`Message step ${index + 1} command failure`"
                >
                  <option :value="undefined">No failure</option>
                  <option value="before_commit">Before command commit</option>
                  <option value="after_commit_before_reply">
                    After commit · lose command reply
                  </option>
                </select></label
              ></template
            >
            <template v-if="step.kind === 'dispatch'"
              ><label class="field"
                >Route<select
                  v-model="step.routeId"
                  :aria-label="`Message step ${index + 1} route`"
                >
                  <option v-if="!route(step)" :value="step.routeId">
                    Choose route: {{ step.routeId }}
                  </option>
                  <option v-for="r in model.routes" :key="r.id" :value="r.id">
                    {{ r.id }} · {{ r.sourceInstanceId }} →
                    {{ r.targetInstanceId }}
                  </option>
                </select></label
              ><label class="field"
                >Message alias<input
                  v-model="step.message"
                  :aria-label="`Message step ${index + 1} alias`" /></label
              ><StateNumberEditor
                v-model="step.expectedRevision"
                :label="`Message step ${index + 1} frozen recipient revision`"
                :disabled="disabled"
              /><label class="field"
                >Source selector<select
                  :value="
                    step.sourceMessage !== undefined ? 'message' : 'command'
                  "
                  :aria-label="`Message step ${index + 1} source kind`"
                  @change="changeSource(step, $event)"
                >
                  <option value="command">Direct source command ID</option>
                  <option value="message">
                    Earlier message's recipient command
                  </option>
                </select></label
              ><label v-if="step.sourceMessage === undefined" class="field"
                >Source command ID<input
                  v-model="step.sourceCommandId"
                  :aria-label="`Message step ${index + 1} source command ID`"
                /><small
                  >The committed command on
                  {{ route(step)?.sourceInstanceId || "the source owner" }}; a
                  missing effect is a modeled outcome.</small
                ></label
              ><label v-else class="field"
                >Earlier source message<select
                  v-model="step.sourceMessage"
                  :aria-label="`Message step ${index + 1} source message`"
                >
                  <option value="">Choose an earlier dispatch alias</option>
                  <option
                    v-if="
                      step.sourceMessage &&
                      !earlierAliases(index).some(
                        (a) => a.name === step.sourceMessage,
                      )
                    "
                    :value="step.sourceMessage"
                  >
                    Unresolved: {{ step.sourceMessage }}
                  </option>
                  <option
                    v-for="alias in earlierAliases(index)"
                    :key="alias.name"
                    :value="alias.name"
                    :disabled="
                      alias.route?.targetInstanceId !==
                      route(step)?.sourceInstanceId
                    "
                  >
                    {{ alias.name }} · recipient
                    {{ alias.route?.targetInstanceId || "unresolved" }}
                  </option>
                </select></label
              ></template
            >
            <template v-if="step.kind === 'deliver'"
              ><label class="field"
                >Previously declared message<select
                  v-model="step.message"
                  :aria-label="`Message step ${index + 1} deliver alias`"
                >
                  <option value="">Choose an earlier dispatch alias</option>
                  <option
                    v-if="
                      step.message &&
                      !earlierAliases(index).some(
                        (a) => a.name === step.message,
                      )
                    "
                    :value="step.message"
                  >
                    Unresolved: {{ step.message }}
                  </option>
                  <option
                    v-for="alias in earlierAliases(index)"
                    :key="alias.name"
                    :value="alias.name"
                  >
                    {{ alias.name }}
                  </option>
                </select></label
              ><label class="field"
                >Delivery failure<select
                  v-model="step.fault"
                  :aria-label="`Message step ${index + 1} delivery failure`"
                >
                  <option :value="undefined">No delivery failure</option>
                  <option value="drop_before_receive">
                    Drop before receipt
                  </option>
                  <option value="before_recipient_commit">
                    Before recipient commit · roll back
                  </option>
                  <option value="after_recipient_commit_before_ack">
                    After recipient commit · lose acknowledgement
                  </option>
                </select></label
              ></template
            >
          </div>
          <div v-if="step.kind === 'command'" class="state-grid">
            <TypedValueEditor
              v-for="port in command(step)?.inputs || []"
              :key="port.name"
              :model-value="step.inputs?.[port.name]"
              :type="port.type"
              :record-ref="port.recordRef"
              :contracts="contracts"
              :label="`Message step ${index + 1} input ${port.name}`"
              :disabled="disabled"
              @update:model-value="put((step.inputs ||= {}), port.name, $event)"
            />
            <div
              v-for="name in unknown(
                step.inputs || {},
                (command(step)?.inputs || []).map((p) => p.name),
              )"
              :key="name"
              class="state-warning"
            >
              Unresolved command input: {{ name }}
              <button class="button quiet" @click="delete step.inputs![name]">
                Remove unresolved input
              </button>
            </div>
          </div>
          <p v-if="step.kind === 'restart'" class="state-help">
            Reset this owner's volatile slots. Its durable state, revision,
            receipts and effects survive. Transport messages, attempts and
            acknowledgements also survive under this experiment's
            durable-transport assumption.
          </p>
          <p v-if="step.kind === 'dispatch'" class="state-help">
            Dispatch materializes one committed intention on this route. It
            freezes the recipient request but does not execute it. Repeating
            this alias/source/route with a different frozen revision must
            conflict.
          </p>
          <p v-if="step.kind === 'deliver'" class="state-help">
            A recipient receipt acknowledges processing, even when its business
            decision is denied. Each explicit attempt counts until acknowledged;
            subsequent deliveries do no work.
          </p>
        </details>
      </fieldset>
    </section>
    <section class="state-panel">
      <h3>Independent final owner expectations</h3>
      <p class="state-help">
        Assertions are handwritten. New owner expectations start at effective
        initial values and zero counts for review; run results never populate
        them. Partial lists verify only named owners.
      </p>
      <div class="state-actions">
        <button
          v-for="instance in model.instances"
          :key="instance.id"
          class="button secondary"
          :disabled="
            disabled ||
            !instanceMachine(model, instance.id) ||
            scenario.expected.some((e) => e.instanceId === instance.id)
          "
          @click="addExpected(instance.id)"
        >
          Expect owner {{ instance.id }}
        </button>
      </div>
      <fieldset :disabled="disabled">
        <details
          v-for="(expected, index) in scenario.expected"
          :key="stateUIKey(expected)"
          class="state-card"
          :open="index === 0"
        >
          <summary>Expected owner · {{ expected.instanceId }}</summary>
          <p
            v-if="!instanceMachine(model, expected.instanceId)"
            class="state-warning"
          >
            Unresolved owner. Remove this assertion and add one for a current
            instance.
          </p>
          <div class="state-grid">
            <StateNumberEditor
              v-model="expected.revision"
              :label="`Expected message owner ${expected.instanceId} revision`"
              :disabled="disabled"
            /><StateNumberEditor
              v-model="expected.receipts"
              :label="`Expected message owner ${expected.instanceId} receipts`"
              :disabled="disabled"
            /><StateNumberEditor
              v-model="expected.effects"
              :label="`Expected message owner ${expected.instanceId} effects`"
              :disabled="disabled"
            />
          </div>
          <div class="state-grid">
            <TypedValueEditor
              v-for="slot in instanceMachine(model, expected.instanceId)
                ?.state || []"
              :key="slot.name"
              :model-value="expected.state[slot.name]"
              :type="slot.type"
              :record-ref="slot.recordRef"
              :contracts="contracts"
              :label="`Expected message owner ${expected.instanceId}.${slot.name}`"
              :disabled="disabled"
              @update:model-value="put(expected.state, slot.name, $event)"
            />
          </div>
          <div
            v-for="name in unknown(
              expected.state,
              (instanceMachine(model, expected.instanceId)?.state || []).map(
                (s) => s.name,
              ),
            )"
            :key="name"
            class="state-warning"
          >
            Unresolved expected slot: {{ name }}
            <button class="button quiet" @click="delete expected.state[name]">
              Remove unresolved expected state
            </button>
          </div>
          <button
            class="button quiet"
            @click="scenario.expected.splice(index, 1)"
          >
            Remove owner expectation
          </button>
        </details>
      </fieldset>
    </section>
    <section class="state-panel">
      <div class="state-panel-heading">
        <h3>Independent message expectations</h3>
        <button
          class="button secondary"
          :disabled="disabled || scenario.expectedMessages.length >= 32"
          @click="addMessageExpected"
        >
          Add message expectation
        </button>
      </div>
      <p class="state-help">
        Absent means no envelope was materialized and requires zero attempts.
        Pending means no acknowledgement. Acknowledged can independently assert
        accepted or denied business outcome.
      </p>
      <fieldset :disabled="disabled">
        <article
          v-for="(expected, index) in scenario.expectedMessages"
          :key="stateUIKey(expected)"
          class="state-card"
        >
          <div class="state-grid">
            <label class="field"
              >Message alias<input
                v-model="expected.name"
                :aria-label="`Expected message ${index + 1} alias`"
                :list="`expected-alias-${stateUIKey(expected)}`" /><datalist
                :id="`expected-alias-${stateUIKey(expected)}`"
              >
                <option
                  v-for="name in allAliases()"
                  :key="name"
                  :value="name"
                /></datalist></label
            ><label class="field"
              >Transport status<select
                v-model="expected.status"
                :aria-label="`Expected message ${index + 1} status`"
              >
                <option value="absent">Absent</option>
                <option value="pending">Pending acknowledgement</option>
                <option value="acknowledged">Acknowledged</option>
              </select></label
            ><StateNumberEditor
              v-model="expected.attempts"
              :label="`Expected message ${index + 1} attempts`"
              :disabled="disabled"
            /><label v-if="expected.status === 'acknowledged'" class="field"
              >Business acceptance<select
                v-model="expected.accepted"
                :aria-label="`Expected message ${index + 1} acceptance`"
              >
                <option :value="undefined">
                  Do not assert business outcome
                </option>
                <option :value="true">Accepted</option>
                <option :value="false">Denied</option>
              </select></label
            >
          </div>
          <p
            v-if="
              expected.status !== 'acknowledged' &&
              expected.accepted !== undefined
            "
            class="state-warning"
          >
            Only acknowledged messages can assert business acceptance.
            <button class="button quiet" @click="delete expected.accepted">
              Remove acceptance assertion
            </button>
          </p>
          <button
            class="button quiet"
            @click="scenario.expectedMessages.splice(index, 1)"
          >
            Remove message expectation
          </button>
        </article>
      </fieldset>
      <p
        v-if="!scenario.expected.length && !scenario.expectedMessages.length"
        class="state-warning"
      >
        This scenario has no expectations and will be recorded as unverified,
        not passing.
      </p>
    </section>
  </div>
</template>
<style scoped>
.delivery-override {
  border-top: 1px solid #e0e9ed;
  margin-top: 10px;
  padding-top: 5px;
}
.delivery-step > summary small {
  font-size: 11px;
  color: #a47c54;
}
</style>
