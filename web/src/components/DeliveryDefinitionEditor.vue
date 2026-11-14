<script setup lang="ts">
import { focusField } from "../validationFocus";
import type { ValidationIssue } from "../types";
import { computed, ref, watch } from "vue";
import { typeKey, typeLabel } from "../behaviorTypes";
import {
  stateKey,
  stateUIKey,
  freshStateName,
  type InstalledStateMachine,
} from "../stateTypes";
import {
  instanceMachine,
  deliveryMachineKey,
  deliveryMachineRef,
  type DeliveryModel,
  type DeliveryRoute,
} from "../deliveryTypes";
import DeliveryDiagram from "./DeliveryDiagram.vue";
import CanvasInspector from "./CanvasInspector.vue";
import DeliveryPositionEditor from "./DeliveryPositionEditor.vue";
const props = defineProps<{
  model: DeliveryModel;
  machines: InstalledStateMachine[];
  disabled?: boolean;
}>();
const emit = defineEmits<{
  "embed-machine": [key: string];
  "import-machine": [];
  "instance-added": [id: string];
}>();
const settings = ref<'instances'|'routes'|null>(null);
const selectedRoute=ref<number|null>(null);
const editorRoot = ref<HTMLElement | null>(null);
function focusIssue(issue: ValidationIssue) {
  const instance = issue.path.match(/instances\[(\d+)\]/),
    route = issue.path.match(/routes\[(\d+)\]/);
  if (instance) { selected.value = props.model.instances[Number(instance[1])]?.id || ""; settings.value='instances'; }
  if (route) { selectedRoute.value=Number(route[1]); settings.value='routes'; }
  if (!instance && !route) settings.value=null;
  focusField(
    editorRoot.value,
    instance
      ? '[aria-label="Message instance ID"]'
      : route
        ? `[aria-label="Route ${Number(route[1]) + 1} name"]`
        : '[aria-label="Message model ID"]',
  );
}
defineExpose({ focusIssue });
const installed = ref(""),
  selected = ref(""),
  editorError = ref("");
watch(
  () => props.model,
  () => (selected.value = props.model.instances[0]?.id || ""),
);
const current = computed(
  () =>
    props.model.instances.find((i) => i.id === selected.value) ||
    props.model.instances[0],
);
function renameInstance(e: Event) {
  if (!current.value) return;
  const input = e.target as HTMLInputElement,
    value = input.value;
  if (
    !/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(value) ||
    props.model.instances.some((i) => i !== current.value && i.id === value)
  ) {
    input.value = current.value.id;
    editorError.value =
      "Instance IDs must be unique letter-first identifiers. Existing references were kept.";
    return;
  }
  current.value.id = value;
  selected.value = value;
  editorError.value = "";
}
function addInstance() {
  settings.value='instances';
  const lock = props.model.machines[0];
  if (!lock) return;
  const id = freshStateName(
    "owner",
    props.model.instances.map((i) => i.id),
  );
  props.model.instances.push({
    id,
    name: id,
    machine: deliveryMachineRef(lock),
    position: {
      x: 40 + (props.model.instances.length % 3) * 360,
      y: 90 + Math.floor(props.model.instances.length / 3) * 180,
    },
  });
  selected.value = id;
  emit("instance-added", id);
}
function setMachine(key: string) {
  const lock = props.model.machines.find((l) => stateKey(l) === key);
  if (lock && current.value) current.value.machine = deliveryMachineRef(lock);
}
function addRoute() {
  settings.value='routes'; selectedRoute.value=props.model.routes.length;
  const source = props.model.instances[0],
    target = props.model.instances[1] || source;
  if (!source || !target) return;
  const effect = instanceMachine(props.model, source.id)?.effects[0],
    command = instanceMachine(props.model, target.id)?.commands[0],
    id = freshStateName(
      "route",
      props.model.routes.map((r) => r.id),
    );
  props.model.routes.push({
    id,
    name: id,
    sourceInstanceId: source.id,
    effect: effect?.name || "",
    targetInstanceId: target.id,
    command: command?.name || "",
    inputs: {},
  });
}
function sourceEffect(route: DeliveryRoute) {
  return instanceMachine(props.model, route.sourceInstanceId)?.effects.find(
    (e) => e.name === route.effect,
  );
}
function targetCommand(route: DeliveryRoute) {
  return instanceMachine(props.model, route.targetInstanceId)?.commands.find(
    (c) => c.name === route.command,
  );
}
function mapInput(route: DeliveryRoute, name: string, e: Event) {
  const value = (e.target as HTMLSelectElement).value;
  if (value) route.inputs[name] = value;
  else delete route.inputs[name];
}
</script>
<template>
  <div ref="editorRoot">
    <p v-if="editorError" class="state-warning" role="alert">
      {{ editorError }}
    </p>
    <div class="canvas-author-tools">
      <button class="button secondary" @click="settings='instances'">Instances</button>
      <button class="button secondary" @click="settings='routes'; selectedRoute=null">Routes</button>
    </div>
    <DeliveryDiagram
      :model="model"
      :selected="current?.id || ''"
      :disabled="disabled"
      @select="selected = $event; settings='instances'"
      @move="
        (id, position) => {
          const instance = model.instances.find((i) => i.id === id);
          if (instance && !disabled) instance.position = position;
        }
      "
      @route="selectedRoute=$event; settings='routes'"
    >
      <CanvasInspector v-if="settings" :title="settings === 'instances' ? 'Message instance settings' : 'Message route settings'" @close="settings=null">
    <section v-show="settings === 'instances'" class="state-panel">
      <div class="state-panel-heading">
        <h3>Pinned instances</h3>
        <button
          class="button secondary"
          :disabled="
            disabled || !model.machines.length || model.instances.length >= 8
          "
          @click="addInstance"
        >
          Add message instance
        </button>
      </div>
      <label class="field"
        >Selected instance<select
          v-model="selected"
          aria-label="Selected message instance"
        >
          <option v-if="!selected && model.instances[0]" value="">
            {{ model.instances[0].id }}
          </option>
          <option
            v-for="instance in model.instances"
            :key="stateUIKey(instance)"
            :value="instance.id"
          >
            {{ instance.id }} · {{ instance.name }}
          </option>
        </select></label
      >
      <fieldset v-if="current" :disabled="disabled">
        <div class="state-grid">
          <label class="field"
            >Instance ID<input
              :value="current.id"
              aria-label="Message instance ID"
              @change="renameInstance" /></label
          ><label class="field"
            >Display name<input
              v-model="current.name"
              aria-label="Message instance name" /></label
          ><label class="field"
            >Exact owner machine<select
              :value="deliveryMachineKey(current.machine)"
              aria-label="Message instance machine"
              @change="setMachine(($event.target as HTMLSelectElement).value)"
            >
              <option
                v-if="
                  !model.machines.some(
                    (l) => stateKey(l) === deliveryMachineKey(current!.machine),
                  )
                "
                :value="deliveryMachineKey(current.machine)"
              >
                Unresolved: {{ current.machine.machineId }}
              </option>
              <option
                v-for="lock in model.machines"
                :key="stateKey(lock)"
                :value="stateKey(lock)"
              >
                {{ lock.machine.id }} @ {{ lock.machine.version }} ·
                {{ lock.digest.slice(0, 10) }}
              </option>
            </select></label
          ><DeliveryPositionEditor
            v-model="current.position.x"
            :identity="stateUIKey(current)"
            label="Message instance X"
            :disabled="disabled"
          /><DeliveryPositionEditor
            v-model="current.position.y"
            :identity="stateUIKey(current)"
            label="Message instance Y"
            :disabled="disabled"
          />
        </div>
        <button
          class="button quiet"
          :disabled="model.instances.length <= 1"
          @click="
            model.instances = model.instances.filter((i) => i !== current);
            selected = model.instances[0]?.id || '';
          "
        >
          Remove selected instance
        </button>
      </fieldset>
      <p class="state-help">
        Renames, removals and machine changes retain scenario values and
        references for explicit correction. Positions are finite decimal
        coordinates within ±1,000,000.
      </p>
    </section>
    <section v-show="settings === 'routes'" class="state-panel">
      <div class="state-panel-heading">
        <h3>Typed effect-to-command routes</h3>
        <button
          class="button secondary"
          :disabled="
            disabled || !model.instances.length || model.routes.length >= 16
          "
          @click="addRoute"
        >
          Add message route
        </button>
      </div>
      <p class="state-help">
        A route maps each recipient command input to a source effect payload of
        the same exact type. Use pure components inside an owner for
        adaptations. Fanout needs distinct targets or commands.
      </p>
      <fieldset :disabled="disabled">
        <details
          v-for="(route, index) in model.routes" v-show="selectedRoute === null || selectedRoute === index"
          :key="stateUIKey(route)"
          class="state-card"
          :open="selectedRoute === index || index === 0"
        >
          <summary>
            {{ route.name || route.id }} · {{ route.sourceInstanceId }} →
            {{ route.targetInstanceId }}
          </summary>
          <div class="state-grid">
            <label class="field"
              >Route ID<input
                v-model="route.id"
                :aria-label="`Route ${index + 1} ID`" /></label
            ><label class="field"
              >Display name<input
                v-model="route.name"
                :aria-label="`Route ${index + 1} name`" /></label
            ><label class="field"
              >Source instance<select
                v-model="route.sourceInstanceId"
                :aria-label="`Route ${index + 1} source`"
              >
                <option
                  v-if="
                    !model.instances.some(
                      (i) => i.id === route.sourceInstanceId,
                    )
                  "
                  :value="route.sourceInstanceId"
                >
                  Unresolved: {{ route.sourceInstanceId }}
                </option>
                <option
                  v-for="instance in model.instances"
                  :key="instance.id"
                  :value="instance.id"
                >
                  {{ instance.id }}
                </option>
              </select></label
            ><label class="field"
              >Committed source effect<select
                v-model="route.effect"
                :aria-label="`Route ${index + 1} effect`"
              >
                <option v-if="!sourceEffect(route)" :value="route.effect">
                  Choose effect: {{ route.effect }}
                </option>
                <option
                  v-for="effect in instanceMachine(
                    model,
                    route.sourceInstanceId,
                  )?.effects || []"
                  :key="effect.name"
                >
                  {{ effect.name }}
                </option>
              </select></label
            ><label class="field"
              >Recipient instance<select
                v-model="route.targetInstanceId"
                :aria-label="`Route ${index + 1} target`"
              >
                <option
                  v-if="
                    !model.instances.some(
                      (i) => i.id === route.targetInstanceId,
                    )
                  "
                  :value="route.targetInstanceId"
                >
                  Unresolved: {{ route.targetInstanceId }}
                </option>
                <option
                  v-for="instance in model.instances"
                  :key="instance.id"
                  :value="instance.id"
                >
                  {{ instance.id }}
                </option>
              </select></label
            ><label class="field"
              >Recipient command<select
                v-model="route.command"
                :aria-label="`Route ${index + 1} command`"
              >
                <option v-if="!targetCommand(route)" :value="route.command">
                  Choose command: {{ route.command }}
                </option>
                <option
                  v-for="command in instanceMachine(
                    model,
                    route.targetInstanceId,
                  )?.commands || []"
                  :key="command.name"
                >
                  {{ command.name }}
                </option>
              </select></label
            >
          </div>
          <h4>Typed payload mapping</h4>
          <div class="state-grid">
            <label
              v-for="port in targetCommand(route)?.inputs || []"
              :key="port.name"
              class="field"
              >Recipient {{ port.name }} · {{ typeLabel(port)
              }}<select
                :value="route.inputs[port.name] || ''"
                :aria-label="`Route ${index + 1} input ${port.name}`"
                @change="mapInput(route, port.name, $event)"
              >
                <option value="">Choose source payload</option>
                <option
                  v-if="
                    route.inputs[port.name] &&
                    !sourceEffect(route)?.payload.some(
                      (p) => p.name === route.inputs[port.name],
                    )
                  "
                  :value="route.inputs[port.name]"
                >
                  Unresolved: {{ route.inputs[port.name] }}
                </option>
                <option
                  v-for="source in sourceEffect(route)?.payload || []"
                  :key="source.name"
                  :value="source.name"
                  :disabled="typeKey(source) !== typeKey(port)"
                >
                  {{ source.name }} · {{ typeLabel(source) }}
                </option>
              </select></label
            >
          </div>
          <div
            v-for="name in Object.keys(route.inputs).filter(
              (name) =>
                !targetCommand(route)?.inputs.some((p) => p.name === name),
            )"
            :key="name"
            class="state-warning"
          >
            Unresolved recipient input: {{ name }}
            <button class="button quiet" @click="delete route.inputs[name]">
              Remove unresolved mapping
            </button>
          </div>
          <button class="button quiet" @click="model.routes.splice(index, 1)">
            Remove route {{ index + 1 }}
          </button>
        </details>
      </fieldset>
    </section>
      </CanvasInspector>
    </DeliveryDiagram>
    <details class="state-panel">
      <summary>Message model identity and description</summary>
      <fieldset :disabled="disabled">
        <div class="state-grid">
          <label class="field"
            >Namespaced ID<input
              v-model="model.id"
              aria-label="Message model ID" /></label
          ><label class="field"
            >Version<input
              v-model="model.version"
              aria-label="Message model version" /></label
          ><label class="field"
            >Display name<input
              v-model="model.name"
              aria-label="Message model name" /></label
          ><label class="field"
            >Author<input
              v-model="model.author"
              aria-label="Message model author" /></label
          ><label class="field"
            >License<input
              v-model="model.license"
              aria-label="Message model license" /></label
          ><label class="field"
            >Description<textarea
              v-model="model.description"
              aria-label="Message model description"
            />
          </label>
        </div>
      </fieldset>
    </details>
    <details class="state-panel"><summary>Owner machine dependencies</summary>
      <div class="state-panel-heading">
        <h3>Exact state machine locks · {{ model.machines.length }} / 4</h3>
        <button
          class="button secondary"
          :disabled="disabled || model.machines.length >= 4"
          @click="emit('import-machine')"
        >
          Import state machine lock
        </button>
      </div>
      <div class="state-actions">
        <label class="field"
          >Installed state machine<select
            v-model="installed"
            aria-label="Message owner machine"
            :disabled="disabled"
          >
            <option value="">Choose an installed exact version</option>
            <option
              v-for="lock in machines"
              :key="stateKey(lock)"
              :value="stateKey(lock)"
            >
              {{ lock.machine.id }} @ {{ lock.machine.version }}
            </option>
          </select></label
        ><button
          class="button secondary"
          :disabled="disabled || !installed || model.machines.length >= 4"
          @click="emit('embed-machine', installed)"
        >
          Embed owner machine
        </button>
      </div>
      <p class="state-help">
        Build and test each owner in State, then pin its complete definition
        here. Installation of this model does not install its dependencies into
        other catalogs.
      </p>
      <p class="state-help">
        The browser rejects exact locks containing negative-zero coordinates to
        preserve their hashes. Retain those artifacts through the CLI, or
        publish a new version with ordinary zero coordinates.
      </p>
      <div
        v-for="(lock, i) in model.machines"
        :key="stateKey(lock)"
        class="state-lock"
      >
        <code
          >{{ lock.machine.id }} @ {{ lock.machine.version }} ·
          {{ lock.digest }}</code
        ><button
          class="button quiet"
          :disabled="disabled"
          @click="model.machines.splice(i, 1)"
        >
          Remove machine lock
        </button>
      </div>
    </details>
  </div>
</template>
