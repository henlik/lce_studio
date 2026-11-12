<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Box,
  Check,
  Database,
  GitBranch,
  LoaderCircle,
  Plus,
  RefreshCw,
  Server,
  X,
} from "lucide-vue-next";
import { api, APIError } from "../api";
import type { CatalogRecord } from "../behaviorTypes";
import type {
  ComponentPackage,
  InstalledComponent,
  PackageLock,
  Project,
  ValidationIssue,
} from "../types";

const props = defineProps<{ project: Project }>();
const emit = defineEmits<{ close: []; add: [profile: PackageLock] }>();
const tab = ref<"bundled" | "installed" | "project">("bundled");
const bundled = ref<PackageLock[]>([]);
const installed = ref<InstalledComponent[]>([]);
const selectedKey = ref("");
const busy = ref("");
const error = ref("");
const issues = ref<ValidationIssue[]>([]);
const notice = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const key = (p: ComponentPackage) => `${p.id}@${p.version}`;
const entries = computed(() =>
  tab.value === "bundled"
    ? bundled.value
    : tab.value === "installed"
      ? installed.value
      : props.project.packages || [],
);
const selected = computed(
  () =>
    entries.value.find((p) => key(p.package) === selectedKey.value) ||
    entries.value[0],
);
const installedSelected = computed(() =>
  installed.value.find(
    (p) => selected.value && key(p.package) === key(selected.value.package),
  ),
);
const lockedSelected = computed(() =>
  props.project.packages?.find(
    (p) => selected.value && key(p.package) === key(selected.value.package),
  ),
);
const projectConflict = computed(
  () =>
    !!selected.value &&
    !!lockedSelected.value &&
    lockedSelected.value.digest !== selected.value.digest,
);
const exactInstalled = computed(
  () =>
    !!selected.value &&
    installedSelected.value?.digest === selected.value.digest,
);
const bounds = computed(() =>
  selected.value
    ? [
        {
          label: "Processing · ms",
          ...selected.value.package.config.processingMs,
        },
        {
          label: "Concurrent slots",
          ...selected.value.package.config.capacity,
        },
        { label: "Memory · MiB", ...selected.value.package.config.memoryMb },
      ]
    : [],
);
function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e);
  issues.value = e instanceof APIError ? e.issues : [];
}
async function load() {
  busy.value = "load";
  error.value = "";
  issues.value = [];
  try {
    const results = await Promise.allSettled([
      api<PackageLock[]>("/components/examples"),
      api<CatalogRecord[]>("/components"),
    ]);
    if (results[0].status === "fulfilled") bundled.value = results[0].value;
    else fail(results[0].reason);
    if (results[1].status === "fulfilled")
      installed.value = results[1].value.filter(
        (p): p is InstalledComponent => p.package.schemaVersion === 1,
      );
    else fail(results[1].reason);
  } finally {
    busy.value = "";
  }
}
async function install(profile: ComponentPackage) {
  const record = await api<InstalledComponent>("/components", "POST", profile);
  installed.value = [
    ...installed.value.filter((p) => key(p.package) !== key(record.package)),
    record,
  ].sort((a, b) => key(a.package).localeCompare(key(b.package)));
  return record;
}
async function installSelected() {
  if (!selected.value || busy.value) return;
  busy.value = "install";
  error.value = "";
  issues.value = [];
  notice.value = "";
  try {
    const record = await install(selected.value.package);
    notice.value = `${record.package.id} @ ${record.package.version} installed. Existing versions are immutable.`;
  } catch (e) {
    fail(e);
  } finally {
    busy.value = "";
  }
}
async function importProfile(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file || busy.value) return;
  busy.value = "import";
  error.value = "";
  issues.value = [];
  notice.value = "";
  try {
    if (!file.name.toLowerCase().endsWith(".json"))
      throw new Error("Choose a component profile with the .json extension.");
    if (file.size > 2 * 1024 * 1024)
      throw new Error("Component profiles must be smaller than 2 MiB.");
    const record = await install(JSON.parse(await file.text()));
    tab.value = "installed";
    selectedKey.value = key(record.package);
    notice.value =
      "Profile installed. Add it to the model to embed its exact definition and digest.";
  } catch (e) {
    fail(e);
  } finally {
    busy.value = "";
    input.value = "";
  }
}
function exportProfile() {
  if (!selected.value) return;
  const p = selected.value.package;
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(p, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = `${p.id}-${p.version}.json`;
  a.click();
  URL.revokeObjectURL(url);
  notice.value =
    "Profile exported. Edit its identity, defaults and bounds, then import it to install a new version.";
}
onMounted(load);
</script>

<template>
  <div
    class="modal-backdrop catalog-backdrop"
    @click.self="!busy && emit('close')"
    @keydown.esc="!busy && emit('close')"
  >
    <section
      class="modal catalog-modal"
      role="dialog"
      aria-modal="true"
      aria-label="Component catalog"
    >
      <div class="modal-heading">
        <div>
          <span class="section-eyebrow">REUSABLE COMPONENTS</span>
          <h2>Component catalog</h2>
        </div>
        <button
          class="icon-button"
          aria-label="Close component catalog"
          :disabled="!!busy"
          @click="emit('close')"
        >
          <X :size="20" />
        </button>
      </div>
      <p class="catalog-intro">
        Profiles configure a service, database or exclusive router. Add a
        profile to embed its exact version in your project; exported projects
        and saved runs keep that definition.
      </p>
      <div class="catalog-toolbar">
        <div
          class="catalog-tabs"
          role="group"
          aria-label="Component catalog source"
        >
          <button
            :class="{ active: tab === 'bundled' }"
            :aria-pressed="tab === 'bundled'"
            @click="
              tab = 'bundled';
              selectedKey = '';
            "
          >
            Built-in profiles <span>{{ bundled.length }}</span>
          </button>
          <button
            :class="{ active: tab === 'installed' }"
            :aria-pressed="tab === 'installed'"
            @click="
              tab = 'installed';
              selectedKey = '';
            "
          >
            Installed <span>{{ installed.length }}</span>
          </button>
          <button
            :class="{ active: tab === 'project' }"
            :aria-pressed="tab === 'project'"
            @click="
              tab = 'project';
              selectedKey = '';
            "
          >
            Project locks <span>{{ project.packages?.length || 0 }}</span>
          </button>
        </div>
        <div class="catalog-tools">
          <button
            class="button secondary"
            :disabled="!!busy"
            @click="fileInput?.click()"
          >
            <ArrowUpFromLine :size="14" /> Import profile
          </button>
          <button
            class="icon-button"
            aria-label="Refresh component catalog"
            :disabled="!!busy"
            @click="load"
          >
            <RefreshCw :size="15" />
          </button>
          <input
            ref="fileInput"
            type="file"
            accept=".json,application/json"
            aria-label="Import component profile JSON"
            class="catalog-file"
            @change="importProfile"
          />
        </div>
      </div>
      <div v-if="error" class="catalog-alert" role="alert">
        <strong>{{ error }}</strong>
        <ul v-if="issues.length">
          <li v-for="issue in issues" :key="issue.path + issue.message">
            <code>{{ issue.path }}</code
            >: {{ issue.message }}
          </li>
        </ul>
      </div>
      <p v-if="notice" class="catalog-notice" role="status">
        <Check :size="15" />{{ notice }}
      </p>
      <div v-if="busy === 'load' && !entries.length" class="catalog-empty">
        <LoaderCircle :size="24" class="spin" />
        <p>Loading reusable profiles…</p>
      </div>
      <div v-else class="catalog-layout">
        <div class="catalog-list" aria-label="Available component profiles">
          <button
            v-for="entry in entries"
            :key="key(entry.package)"
            class="catalog-entry"
            :class="{ selected: selected?.digest === entry.digest }"
            :aria-label="`Select profile ${entry.package.id} version ${entry.package.version}`"
            @click="selectedKey = key(entry.package)"
          >
            <span class="catalog-kernel" :class="entry.package.kernel"
              ><Database
                v-if="entry.package.kernel === 'database'"
                :size="18" /><GitBranch
                v-else-if="entry.package.kernel === 'router'"
                :size="18" /><Server v-else :size="18"
            /></span>
            <span
              ><strong>{{ entry.package.name }}</strong
              ><small>{{ entry.package.id }}</small
              ><em
                >v{{ entry.package.version }} · {{ entry.package.kernel }}</em
              ></span
            >
          </button>
          <div v-if="!entries.length" class="catalog-empty">
            <Box :size="26" />
            <p>
              {{
                tab === "project"
                  ? "Add a reusable profile to create a portable project lock."
                  : tab === "installed"
                    ? "Import a JSON profile or install one of the built-in profiles."
                    : "No bundled profiles are available."
              }}
            </p>
          </div>
        </div>
        <article v-if="selected" class="catalog-detail">
          <div class="catalog-detail-heading">
            <span class="type-chip">{{ selected.package.kernel }} profile</span
            ><span class="catalog-capabilities">Design · Simulate</span>
          </div>
          <h3>{{ selected.package.name }}</h3>
          <code class="catalog-identity"
            >{{ selected.package.id }} @ {{ selected.package.version }}</code
          >
          <p>
            {{
              selected.package.description ||
              "A reusable configuration profile."
            }}
          </p>
          <dl class="catalog-author">
            <div>
              <dt>Author</dt>
              <dd>{{ selected.package.author }}</dd>
            </div>
            <div>
              <dt>License</dt>
              <dd>{{ selected.package.license }}</dd>
            </div>
          </dl>
          <table class="catalog-bounds">
            <thead>
              <tr>
                <th>PROPERTY</th>
                <th>DEFAULT</th>
                <th>ALLOWED</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="bound in bounds" :key="bound.label">
                <td>{{ bound.label }}</td>
                <td>{{ bound.default }}</td>
                <td>{{ bound.min }}–{{ bound.max }}</td>
              </tr>
            </tbody>
          </table>
          <p
            v-if="selected.package.kernel === 'database'"
            class="catalog-detail-note"
          >
            Deduplication defaults to
            {{ selected.package.config.deduplicate ? "on" : "off" }}; each
            database instance can change it.
          </p>
          <p
            v-if="selected.package.kernel === 'router'"
            class="catalog-detail-note"
          >
            Chooses one weighted output per logical request. Retries keep the
            same route. Branches may converge; there is no parallel fan-out or
            join.
          </p>
          <details class="catalog-digest">
            <summary>Content identity · SHA-256</summary>
            <code>{{ selected.digest }}</code>
            <p>
              A content digest identifies this definition; it is not a publisher
              signature.
            </p>
          </details>
          <p v-if="projectConflict" class="catalog-alert" role="alert">
            This project already embeds different content at the same ID and
            version. Import a new version to keep its existing components
            reproducible.
          </p>
          <div class="catalog-detail-actions">
            <button
              class="button primary"
              :disabled="
                !!busy ||
                projectConflict ||
                project.nodes.length >= 30 ||
                (!lockedSelected && (project.packages?.length || 0) >= 50)
              "
              @click="emit('add', selected)"
            >
              <Plus :size="15" /> Add to model
            </button>
            <button
              class="button secondary"
              :disabled="!!busy || exactInstalled"
              @click="installSelected"
            >
              <Check v-if="exactInstalled" :size="14" /><ArrowDownToLine
                v-else
                :size="14"
              />{{ exactInstalled ? "Installed" : "Install profile" }}
            </button>
            <button
              class="button quiet"
              :disabled="!!busy"
              @click="exportProfile"
            >
              <ArrowDownToLine :size="14" /> Export JSON
            </button>
          </div>
          <p class="catalog-detail-note">
            Adding a profile upgrades older projects to schema 3 while
            preserving the design. Defaults are copied once; existing values are
            never silently changed.
          </p>
        </article>
        <div v-else class="catalog-detail catalog-empty">
          <Box :size="30" />
          <p>Select a profile to inspect its identity and limits.</p>
        </div>
      </div>
      <div class="catalog-footer">
        Declarative simulation profiles only. No external code executes, and
        these profiles do not generate applications or deploy systems.
      </div>
    </section>
  </div>
</template>

<style scoped>
.catalog-backdrop {
  z-index: 65;
}
.catalog-modal {
  width: min(950px, calc(100vw - 32px));
  max-height: calc(100dvh - 40px);
  padding: 25px;
  overflow-y: auto;
}
.catalog-intro {
  color: #788a9b;
  font-size: 12px;
  line-height: 1.7;
  margin: 0 0 18px;
  max-width: 720px;
}
.catalog-toolbar,
.catalog-tools,
.catalog-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.catalog-toolbar {
  justify-content: space-between;
  margin-bottom: 16px;
}
.catalog-tabs {
  gap: 3px;
  background: #f1f5f8;
  padding: 4px;
  border-radius: 7px;
}
.catalog-tabs button {
  background: transparent;
  color: #7a8c9d;
  padding: 8px;
  border-radius: 5px;
  font-size: 11px;
}
.catalog-tabs button.active {
  background: white;
  color: #168875;
  box-shadow: 0 1px 5px #183b4510;
}
.catalog-tabs span {
  opacity: 0.65;
  margin-left: 3px;
}
.catalog-tools .button {
  font-size: 11px;
  padding: 8px 10px;
}
.catalog-file {
  display: none;
}
.catalog-layout {
  display: grid;
  grid-template-columns: 265px minmax(0, 1fr);
  border: 1px solid var(--line);
  border-radius: 9px;
  overflow: hidden;
}
.catalog-list {
  background: #f8fafc;
  padding: 10px;
  border-right: 1px solid var(--line);
  max-height: 530px;
  overflow: auto;
}
.catalog-entry {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  width: 100%;
  padding: 13px 10px;
  background: transparent;
  border: 1px solid transparent;
  text-align: left;
  border-radius: 7px;
  margin-bottom: 4px;
}
.catalog-entry.selected {
  background: white;
  border-color: #b5dbd4;
  box-shadow: 0 3px 9px #203a4006;
}
.catalog-entry > span:last-child {
  min-width: 0;
}
.catalog-entry strong,
.catalog-entry small,
.catalog-entry em {
  display: block;
  overflow-wrap: anywhere;
}
.catalog-entry strong {
  font-size: 12px;
  color: #364d62;
  line-height: 1.5;
}
.catalog-entry small {
  font-size: 10px;
  color: #8395a6;
  margin-top: 4px;
}
.catalog-entry em {
  font-style: normal;
  font-size: 10px;
  margin-top: 7px;
  color: #6b8e89;
}
.catalog-kernel {
  display: flex;
  padding: 7px;
  border-radius: 7px;
  color: #37917d;
  background: #e9f4f0;
}
.catalog-kernel.database {
  color: #8c72b1;
  background: #f1edf8;
}
.catalog-kernel.router {
  color: #b1843d;
  background: #fbf2df;
}
.catalog-detail {
  padding: 22px;
  min-width: 0;
}
.catalog-detail-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.catalog-capabilities {
  font-size: 10px;
  color: #6b938c;
}
.catalog-detail h3 {
  font-size: 21px;
  margin: 15px 0 8px;
  color: #2b4256;
}
.catalog-identity {
  font-size: 11px;
  color: #487a84;
  overflow-wrap: anywhere;
}
.catalog-detail > p {
  font-size: 12px;
  color: #788b9b;
  line-height: 1.7;
}
.catalog-author {
  display: flex;
  flex-wrap: wrap;
  gap: 30px;
  margin: 18px 0;
  font-size: 11px;
}
.catalog-author dt {
  color: #96a4b2;
  font-size: 10px;
}
.catalog-author dd {
  color: #566d80;
  margin: 5px 0 0;
}
.catalog-bounds {
  border-collapse: collapse;
  width: 100%;
  font-size: 11px;
  text-align: left;
}
.catalog-bounds th {
  font-size: 8px;
  letter-spacing: 0.6px;
  color: #899aaa;
  background: #f5f8fa;
  padding: 9px;
}
.catalog-bounds td {
  padding: 10px 9px;
  border-bottom: 1px solid #eef2f5;
  color: #617789;
}
.catalog-detail .catalog-detail-note {
  font-size: 10px;
}
.catalog-digest {
  color: #7a8b9d;
  font-size: 10px;
  padding: 12px 0;
}
.catalog-digest summary {
  cursor: pointer;
}
.catalog-digest code {
  display: block;
  margin-top: 10px;
  overflow-wrap: anywhere;
  line-height: 1.6;
}
.catalog-digest p {
  font-size: 10px;
  line-height: 1.6;
}
.catalog-detail-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 12px;
}
.catalog-detail-actions .button {
  font-size: 11px;
  padding: 9px 11px;
}
.catalog-alert {
  border: 1px solid #efc7bc;
  border-radius: 6px;
  background: #fff8f5;
  padding: 12px;
  color: #a66958;
  font-size: 11px;
  line-height: 1.65;
  margin-bottom: 14px;
  overflow-wrap: anywhere;
}
.catalog-alert ul {
  margin: 6px 0 0;
  padding-left: 18px;
}
.catalog-notice {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: #258672;
  background: #effaf5;
  padding: 12px;
  border-radius: 6px;
  line-height: 1.6;
}
.catalog-empty {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 7px;
  text-align: center;
  padding: 35px 20px;
  color: #92a4b3;
  font-size: 12px;
  line-height: 1.7;
  min-height: 180px;
}
.catalog-footer {
  font-size: 10px;
  color: #92a1ae;
  line-height: 1.7;
  margin-top: 15px;
}
@media (max-width: 650px) {
  .catalog-modal {
    padding: 17px;
    max-height: calc(100dvh - 20px);
  }
  .catalog-layout {
    grid-template-columns: 1fr;
  }
  .catalog-list {
    max-height: 180px;
    border-right: 0;
    border-bottom: 1px solid var(--line);
  }
  .catalog-detail {
    padding: 17px;
  }
  .catalog-detail h3 {
    font-size: 19px;
  }
  .catalog-tabs button {
    font-size: 10px;
    padding: 7px;
  }
  .catalog-toolbar {
    gap: 12px;
  }
}
</style>
