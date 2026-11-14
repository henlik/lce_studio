<script setup lang="ts">
import { focusField } from "../validationFocus";
import type { ValidationIssue } from "../types";
import { computed, ref, watch } from "vue";
import {
  cloneBehavior,
  typedDefault,
  typeKey,
  typeLabel,
  lockKey,
  type InstalledComposite,
  type BehaviorPort,
  type ValueShape,
  type ValueType,
} from "../behaviorTypes";
import {
  dataKey,
  type DataContractLock,
  type DataContractRecord,
} from "../dataTypes";
import {
  decisionRoot,
  freshStateName,
  stateUIKey,
  type StateMachine,
  type StateCommand,
  type StateSlot,
} from "../stateTypes";
import StateTypeSelect from "./StateTypeSelect.vue";
import StatePortEditor from "./StatePortEditor.vue";
import StateBindingEditor from "./StateBindingEditor.vue";
import StateDiagram from "./StateDiagram.vue";
import CanvasInspector from "./CanvasInspector.vue";
import TypedValueEditor from "./TypedValueEditor.vue";
const props = defineProps<{
  machine: StateMachine;
  contracts: DataContractLock[];
  catalog: InstalledComposite[];
  dataCatalog: DataContractRecord[];
  disabled?: boolean;
  issues?: ValidationIssue[];
}>();
const emit = defineEmits<{
  "load-handler": [index: number, key: string];
  "import-handler": [index: number];
  "embed-data": [key: string];
}>();
const settings = ref<'slots'|'effects'|'commands'|null>(null);
const selectedSlot = ref<number|null>(null), selectedEffect = ref<number|null>(null);
const editorRoot = ref<HTMLElement | null>(null);
function focusIssue(issue: ValidationIssue) {
  const command = issue.path.match(/commands\[(\d+)\]/),
    slot = issue.path.match(/state\[(\d+)\]/), effect = issue.path.match(/effects\[(\d+)\]/);
  if (command) { selected.value = Number(command[1]); settings.value='commands'; }
  if (slot) { selectedSlot.value=Number(slot[1]); settings.value='slots'; }
  if (!command && !slot && effect) { selectedEffect.value=Number(effect[1]); settings.value='effects'; }
  if (!command && !slot && !effect) settings.value=null;
  focusField(
    editorRoot.value,
    command
      ? '[aria-label="State command name"]'
      : slot
        ? `[aria-label="State slot ${Number(slot[1]) + 1} name"]`
        : effect ? `[aria-label="Effect ${Number(effect[1]) + 1} name"]` : '[aria-label="State machine ID"]',
  );
}
defineExpose({ focusIssue });
const selected = ref(0),
  handler = ref(""),
  dataSelection = ref("");
watch(
  () => props.machine,
  () => {
    selected.value = 0;
    settings.value=null; selectedSlot.value=null; selectedEffect.value=null;
    handler.value = "";
  },
);
const command = computed(() => props.machine.commands[selected.value]);
const decision = computed(() =>
  command.value ? decisionRoot(command.value) : undefined,
);
const outputs = computed(() => decision.value?.behavior.outputs || []);
function addSlot() {
  settings.value='slots'; selectedSlot.value=props.machine.state.length;
  props.machine.state.push({
    name: freshStateName(
      "state",
      props.machine.state.map((s) => s.name),
    ),
    type: "integer",
    durability: "durable",
    initial: typedDefault("integer"),
  });
}
function setType(slot: StateSlot, shape: ValueShape) {
  slot.type = shape.type as ValueType;
  if (shape.recordRef) slot.recordRef = { ...shape.recordRef };
  else delete slot.recordRef;
}
function addCommand() {
  settings.value='commands';
  props.machine.commands.push({
    name: freshStateName(
      "Command",
      props.machine.commands.map((c) => c.name),
    ),
    inputs: [],
    outputs: [],
    decision: {
      schemaVersion: 1,
      root: { packageId: "", version: "", digest: "" },
      packages: [],
    },
    bindings: {},
    acceptedOutput: "",
    writes: {},
    results: {},
    effects: [],
  });
  selected.value = props.machine.commands.length - 1;
}
function addEffect() {
  settings.value='effects'; selectedEffect.value=props.machine.effects.length;
  props.machine.effects.push({
    name: freshStateName(
      "Effect",
      props.machine.effects.map((e) => e.name),
    ),
    payload: [],
  });
}
function mapping(map: Record<string, string>, key: string, e: Event) {
  const value = (e.target as HTMLSelectElement).value;
  if (value) map[key] = value;
  else delete map[key];
}
function extraKeys(map: Record<string, unknown>, ports: { name: string }[]) {
  return Object.keys(map).filter((name) => !ports.some((p) => p.name === name));
}
function enableEffect(name: string, e: Event) {
  if (!command.value) return;
  if ((e.target as HTMLInputElement).checked)
    command.value.effects.push({ effect: name, payload: {} });
  else
    command.value.effects = command.value.effects.filter(
      (b) => b.effect !== name,
    );
}
function booleanOutputs() {
  return outputs.value.filter((p) => p.type === "boolean");
}
</script>
<template>
  <div ref="editorRoot" class="state-definition">
    <div class="canvas-author-tools">
      <button class="button secondary" @click="settings='slots'; selectedSlot=null">Owned state</button>
      <button class="button secondary" @click="settings='effects'; selectedEffect=null">Effects</button>
      <button class="button secondary" @click="settings='commands'">Commands</button>
    </div>
    <StateDiagram
      :machine="machine"
      :selected="selected"
      @select="selected = $event; settings='commands'"
      @slot="selectedSlot=$event; settings='slots'" @effect="selectedEffect=$event; settings='effects'"
    >
      <CanvasInspector v-if="settings" :title="settings === 'commands' ? 'Command settings' : settings === 'slots' ? 'Owned state settings' : 'Effect settings'" @close="settings=null">
    <section v-show="settings === 'slots'" class="state-panel">
      <div class="state-panel-heading">
        <h3>Owned state</h3>
        <button
          class="button secondary"
          :disabled="disabled || machine.state.length >= 8"
          @click="addSlot"
        >
          Add state slot
        </button>
      </div>
      <p class="state-help">
        Durable slots survive an explicit simulated restart. Volatile slots
        reset to each instance's effective initial value. Each run begins from
        fresh initialization.
      </p>
      <fieldset :disabled="disabled">
        <article
          v-for="(slot, i) in machine.state" v-show="selectedSlot === null || selectedSlot === i"
          :key="stateUIKey(slot)"
          class="state-card"
        >
          <div class="state-grid">
            <label class="field"
              >Slot name<input
                v-model="slot.name"
                :aria-label="`State slot ${i + 1} name`" /></label
            ><label class="field"
              >Type<StateTypeSelect
                :model-value="slot"
                :contracts="contracts"
                :label="`State slot ${i + 1} type`"
                @update:model-value="setType(slot, $event)" /></label
            ><label class="field"
              >Restart policy<select
                v-model="slot.durability"
                :aria-label="`State slot ${i + 1} durability`"
              >
                <option value="durable">Durable · preserve on restart</option>
                <option value="volatile">Volatile · reset on restart</option>
              </select></label
            >
          </div>
          <TypedValueEditor
            :model-value="slot.initial"
            :type="slot.type"
            :record-ref="slot.recordRef"
            :contracts="contracts"
            :label="`Default ${slot.name}`"
            @update:model-value="
              (value) => {
                if (value) slot.initial = value;
              }
            "
          /><button
            class="button quiet"
            :disabled="machine.state.length <= 1"
            @click="machine.state.splice(i, 1)"
          >
            Remove state slot {{ i + 1 }}
          </button>
        </article>
      </fieldset>
    </section>
    <section v-show="settings === 'effects'" class="state-panel">
      <div class="state-panel-heading">
        <h3>Effect declarations</h3>
        <button
          class="button secondary"
          :disabled="disabled || machine.effects.length >= 8"
          @click="addEffect"
        >
          Add effect declaration
        </button>
      </div>
      <p class="state-help">
        Effects are committed intentions with typed payloads. This workspace
        does not deliver them or contact external services.
      </p>
      <fieldset :disabled="disabled">
        <article
          v-for="(effect, i) in machine.effects" v-show="selectedEffect === null || selectedEffect === i"
          :key="stateUIKey(effect)"
          class="state-card"
        >
          <label class="field"
            >Effect name<input
              v-model="effect.name"
              :aria-label="`Effect ${i + 1} name`" /></label
          ><StatePortEditor
            :ports="effect.payload"
            :contracts="contracts"
            :label="`${effect.name} payload field`"
            :disabled="disabled"
          /><button class="button quiet" @click="machine.effects.splice(i, 1)">
            Remove effect declaration {{ i + 1 }}
          </button>
        </article>
      </fieldset>
    </section>
    <section v-show="settings === 'commands'" class="state-panel">
      <div class="state-panel-heading">
        <h3>Command handlers</h3>
        <button
          class="button secondary"
          :disabled="disabled || machine.commands.length >= 8"
          @click="addCommand"
        >
          Add command
        </button>
      </div>
      <label class="field"
        >Selected command<select
          v-model.number="selected"
          aria-label="Selected state command"
        >
          <option v-for="(c, i) in machine.commands" :key="i" :value="i">
            {{ c.name }}
          </option>
        </select></label
      ><template v-if="command"
        ><fieldset :disabled="disabled">
          <p
            v-for="issue in (issues || []).filter((i) =>
              i.path.includes(`commands[${selected}]`),
            )"
            :key="issue.path"
            class="state-warning"
          >
            {{ issue.message }}
          </p>
          <label class="field"
            >Command name<input
              v-model="command.name"
              aria-label="State command name"
          /></label>
          <div class="state-actions">
            <label class="field"
              >Pure handler from Components<select
                v-model="handler"
                aria-label="State handler component"
              >
                <option value="">Select an installed exact version</option>
                <option
                  v-for="p in catalog"
                  :key="lockKey(p)"
                  :value="lockKey(p)"
                >
                  {{ p.package.id }} @ {{ p.package.version }}
                </option>
              </select></label
            ><button
              class="button secondary"
              :disabled="!handler"
              @click="emit('load-handler', selected, handler)"
            >
              Use locked handler</button
            ><button
              class="button secondary"
              @click="emit('import-handler', selected)"
            >
              Import handler bundle
            </button>
          </div>
          <p class="state-help">
            Create the pure decision in Components, test it there, then install
            or export its complete bundle. It must return a Boolean acceptance
            decision. The machine pins its exact transitive contents.
          </p>
          <p v-if="decision" class="state-lock">
            <code
              >{{ command.decision.root.packageId }} @
              {{ command.decision.root.version }} ·
              {{ command.decision.root.digest }}</code
            >
          </p>
          <p v-else class="state-warning">
            Choose or import a valid handler bundle to configure its decision
            boundary.
          </p>
          <details open class="state-subsection">
            <summary>Public command inputs and outputs</summary>
            <h4>Caller inputs</h4>
            <StatePortEditor
              :ports="command.inputs"
              :contracts="contracts"
              label="command input"
              :disabled="disabled"
            />
            <h4>Returned outputs</h4>
            <StatePortEditor
              :ports="command.outputs"
              :contracts="contracts"
              label="command output"
              :disabled="disabled"
            />
          </details>
          <template v-if="decision"
            ><h4>Decision inputs</h4>
            <StateBindingEditor
              v-for="port in decision.behavior.inputs"
              :key="stateUIKey(command) + ':' + port.name"
              :model-value="command.bindings[port.name]"
              :port="port"
              :state="machine.state"
              :inputs="command.inputs"
              :contracts="contracts"
              :label="`Handler ${port.name}`"
              :disabled="disabled"
              @update:model-value="
                (value) => {
                  if (value) command!.bindings[port.name] = value;
                  else delete command!.bindings[port.name];
                }
              "
            />
            <div
              v-for="name in extraKeys(
                command.bindings,
                decision.behavior.inputs,
              )"
              :key="name"
              class="state-warning"
            >
              Unresolved handler binding: {{ name }}
              <button
                class="button quiet"
                @click="delete command.bindings[name]"
              >
                Remove unresolved binding
              </button>
            </div>
            <label class="field"
              >Acceptance output<select
                v-model="command.acceptedOutput"
                aria-label="Command acceptance output"
              >
                <option value="">Choose a Boolean decision output</option>
                <option
                  v-if="
                    command.acceptedOutput &&
                    !booleanOutputs().some(
                      (o) => o.name === command!.acceptedOutput,
                    )
                  "
                  :value="command.acceptedOutput"
                >
                  Unresolved: {{ command.acceptedOutput }}
                </option>
                <option v-for="p in booleanOutputs()" :key="p.name">
                  {{ p.name }}
                </option>
              </select></label
            >
            <p class="state-help">
              False commits a denial receipt and returned outputs. It publishes
              no state writes or effect intentions.
            </p>
            <h4>State writes on acceptance</h4>
            <div class="state-grid">
              <label
                v-for="slot in machine.state"
                :key="slot.name"
                class="field"
                >Write {{ slot.name
                }}<select
                  :value="command.writes[slot.name] || ''"
                  :aria-label="`Write state ${slot.name}`"
                  @change="mapping(command.writes, slot.name, $event)"
                >
                  <option value="">Keep current value</option>
                  <option
                    v-if="
                      command.writes[slot.name] &&
                      !outputs.some(
                        (p) => p.name === command!.writes[slot.name],
                      )
                    "
                    :value="command.writes[slot.name]"
                  >
                    Unresolved: {{ command.writes[slot.name] }}
                  </option>
                  <option
                    v-for="p in outputs"
                    :key="p.name"
                    :value="p.name"
                    :disabled="typeKey(slot) !== typeKey(p)"
                  >
                    {{ p.name }} · {{ typeLabel(p) }}
                  </option>
                </select></label
              >
            </div>
            <div
              v-for="name in extraKeys(command.writes, machine.state)"
              :key="name"
              class="state-warning"
            >
              Unresolved written slot: {{ name }}
              <button class="button quiet" @click="delete command.writes[name]">
                Remove unresolved write
              </button>
            </div>
            <h4>Public returned values</h4>
            <div class="state-grid">
              <label
                v-for="port in command.outputs"
                :key="port.name"
                class="field"
                >Return {{ port.name
                }}<select
                  :value="command.results[port.name] || ''"
                  :aria-label="`Return ${port.name}`"
                  @change="mapping(command.results, port.name, $event)"
                >
                  <option value="">Choose a decision output</option>
                  <option
                    v-if="
                      command.results[port.name] &&
                      !outputs.some(
                        (p) => p.name === command!.results[port.name],
                      )
                    "
                    :value="command.results[port.name]"
                  >
                    Unresolved: {{ command.results[port.name] }}
                  </option>
                  <option
                    v-for="p in outputs"
                    :key="p.name"
                    :value="p.name"
                    :disabled="typeKey(port) !== typeKey(p)"
                  >
                    {{ p.name }} · {{ typeLabel(p) }}
                  </option>
                </select></label
              >
            </div>
            <div
              v-for="name in extraKeys(command.results, command.outputs)"
              :key="name"
              class="state-warning"
            >
              Unresolved returned port: {{ name }}
              <button
                class="button quiet"
                @click="delete command.results[name]"
              >
                Remove unresolved result
              </button>
            </div>
            <h4>Effects on acceptance</h4>
            <label
              v-for="effect in machine.effects"
              :key="effect.name"
              class="state-check"
              ><input
                type="checkbox"
                :checked="command.effects.some((b) => b.effect === effect.name)"
                :aria-label="`Emit ${effect.name}`"
                @change="enableEffect(effect.name, $event)"
              />
              Stage {{ effect.name }}</label
            >
            <article
              v-for="(binding, i) in command.effects"
              :key="stateUIKey(binding)"
              class="state-card"
            >
              <h4>{{ binding.effect }}</h4>
              <label class="field"
                >Additional condition<select
                  v-model="binding.whenOutput"
                  :aria-label="`${binding.effect} condition output`"
                >
                  <option :value="undefined">Always when accepted</option>
                  <option
                    v-if="
                      binding.whenOutput &&
                      !booleanOutputs().some(
                        (p) => p.name === binding.whenOutput,
                      )
                    "
                    :value="binding.whenOutput"
                  >
                    Unresolved: {{ binding.whenOutput }}
                  </option>
                  <option v-for="p in booleanOutputs()" :key="p.name">
                    {{ p.name }}
                  </option>
                </select></label
              >
              <div class="state-grid">
                <label
                  v-for="port in machine.effects.find(
                    (e) => e.name === binding.effect,
                  )?.payload || []"
                  :key="port.name"
                  class="field"
                  >Payload {{ port.name
                  }}<select
                    :value="binding.payload[port.name] || ''"
                    :aria-label="`${binding.effect} payload ${port.name}`"
                    @change="mapping(binding.payload, port.name, $event)"
                  >
                    <option value="">Choose a decision output</option>
                    <option
                      v-if="
                        binding.payload[port.name] &&
                        !outputs.some(
                          (p) => p.name === binding.payload[port.name],
                        )
                      "
                      :value="binding.payload[port.name]"
                    >
                      Unresolved: {{ binding.payload[port.name] }}
                    </option>
                    <option
                      v-for="p in outputs"
                      :key="p.name"
                      :value="p.name"
                      :disabled="typeKey(port) !== typeKey(p)"
                    >
                      {{ p.name }} · {{ typeLabel(p) }}
                    </option>
                  </select></label
                >
              </div>
              <div
                v-for="name in extraKeys(
                  binding.payload,
                  machine.effects.find((e) => e.name === binding.effect)
                    ?.payload || [],
                )"
                :key="name"
                class="state-warning"
              >
                Unresolved payload field: {{ name }}
                <button
                  class="button quiet"
                  @click="delete binding.payload[name]"
                >
                  Remove unresolved payload
                </button>
              </div>
              <button
                class="button quiet"
                @click="command.effects.splice(i, 1)"
              >
                Remove effect binding
              </button>
            </article></template
          ><button
            class="button danger"
            :disabled="machine.commands.length <= 1"
            @click="
              machine.commands.splice(selected, 1);
              selected = Math.max(0, selected - 1);
            "
          >
            Remove command
          </button>
        </fieldset></template
      >
      <p class="state-help">
        Renames and removals leave dependent references visible for correction.
        Validate before running.
      </p>
    </section>
      </CanvasInspector>
    </StateDiagram>
    <details class="state-panel">
      <summary>Machine identity and description</summary>
      <fieldset :disabled="disabled">
        <div class="state-grid">
          <label class="field"
            >Namespaced ID<input
              v-model="machine.id"
              aria-label="State machine ID" /></label
          ><label class="field"
            >Version<input
              v-model="machine.version"
              aria-label="State machine version" /></label
          ><label class="field"
            >Display name<input
              v-model="machine.name"
              aria-label="State machine name" /></label
          ><label class="field"
            >Author<input
              v-model="machine.author"
              aria-label="State machine author" /></label
          ><label class="field"
            >License<input
              v-model="machine.license"
              aria-label="State machine license" /></label
          ><label class="field"
            >Description<textarea
              v-model="machine.description"
              aria-label="State machine description"
            />
          </label>
        </div>
      </fieldset>
    </details>
    <details class="state-panel">
      <summary>
        Exact data contracts · {{ contracts.length }} available from machine and
        handlers
      </summary>
      <p class="state-help">
        Handler bundle types are available automatically. Add a machine-owned
        contract for additional state or payload types.
      </p>
      <div class="state-actions">
        <label class="field"
          >Installed data contract<select
            v-model="dataSelection"
            aria-label="State data contract"
            :disabled="disabled"
          >
            <option value="">Select an exact version</option>
            <option
              v-for="l in dataCatalog"
              :key="dataKey(l)"
              :value="dataKey(l)"
            >
              {{ l.contract.id }} @ {{ l.contract.version }}
            </option>
          </select></label
        ><button
          class="button secondary"
          :disabled="
            disabled || !dataSelection || machine.dataContracts.length >= 8
          "
          @click="emit('embed-data', dataSelection)"
        >
          Embed state data contract
        </button>
      </div>
      <div
        v-for="(l, i) in machine.dataContracts"
        :key="dataKey(l)"
        class="state-lock"
      >
        <code
          >{{ l.contract.id }} @ {{ l.contract.version }} ·
          {{ l.digest.slice(0, 12) }}</code
        ><button
          class="button quiet"
          :disabled="disabled"
          @click="machine.dataContracts.splice(i, 1)"
        >
          Remove machine lock
        </button>
      </div>
    </details>
  </div>
</template>
