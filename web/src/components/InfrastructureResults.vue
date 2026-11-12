<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  ArrowLeftRight,
  CircleAlert,
  Cpu,
  LoaderCircle,
  Zap,
} from "lucide-vue-next";
import { api } from "../api";
import type { Project, Run } from "../types";

const props = defineProps<{ run: Run; history: Run[] }>();
const selected = ref("");
const previous = ref<Run | null>(null);
const loading = ref(false);
const error = ref("");
let generation = 0;
const candidates = computed(() =>
  props.history.filter((r) => r.id !== props.run.id),
);
const fmt = (n: number, digits = 2) =>
  new Intl.NumberFormat("en", { maximumFractionDigits: digits }).format(n);
const when = (r: Run) => new Date(r.createdAt).toLocaleString();
const stable = (v: unknown) =>
  JSON.stringify(v, (_k, value) =>
    value && typeof value === "object" && !Array.isArray(value)
      ? Object.fromEntries(
          Object.entries(value).sort(([a], [b]) => a.localeCompare(b)),
        )
      : value,
  );
const application = (p: Project) => ({
  nodes: p.nodes.map((n) => ({
    id: n.id,
    type: n.type,
    config: n.config,
    component: n.component,
  })),
  edges: p.edges,
  requirements: p.requirements,
});
const infrastructure = (p: Project) =>
  p.infrastructure
    ? {
        sites: p.infrastructure.sites,
        hosts: p.infrastructure.hosts,
        links: p.infrastructure.links,
        messageBytes: p.infrastructure.messageBytes,
      }
    : null;
const differences = computed(() => {
  if (!previous.value) return [];
  const before = previous.value,
    after = props.run,
    changes: string[] = [];
  if (before.engineVersion !== after.engineVersion)
    changes.push("engine version");
  if (before.project.schemaVersion !== after.project.schemaVersion)
    changes.push("project schema");
  if (stable(before.project.packages) !== stable(after.project.packages))
    changes.push("embedded component profiles");
  if (stable(before.project.scenario) !== stable(after.project.scenario))
    changes.push("workload / retry settings");
  if (
    stable(application(before.project)) !== stable(application(after.project))
  )
    changes.push("application / requirements");
  if (
    stable(infrastructure(before.project)) !==
    stable(infrastructure(after.project))
  )
    changes.push("infrastructure / outage settings");
  if (
    stable(before.project.infrastructure?.placements) !==
    stable(after.project.infrastructure?.placements)
  )
    changes.push("placement");
  return changes;
});
function hostName(run: Run, id: string) {
  return run.project.infrastructure?.hosts.find((h) => h.id === id)?.name || id;
}
function linkName(run: Run, id: string) {
  const link = run.project.infrastructure?.links.find((l) => l.id === id);
  return link
    ? `${hostName(run, link.sourceHostId)} ↔ ${hostName(run, link.targetHostId)}`
    : id;
}
function placements(run: Run) {
  return (
    run.project.infrastructure?.placements.map(
      (p) =>
        `${run.project.nodes.find((n) => n.id === p.nodeId)?.name || p.nodeId} → ${hostName(run, p.hostId)}`,
    ) || ["Application-only model"]
  );
}
const interruptions = computed(
  () =>
    props.run.infrastructure?.hosts.reduce(
      (sum, h) => sum + h.interruptedJobs,
      0,
    ) || 0,
);
const drops = computed(
  () =>
    props.run.infrastructure?.links.reduce(
      (sum, l) => sum + l.droppedTransfers,
      0,
    ) || 0,
);
const metricRows = computed(() => {
  if (!previous.value) return [];
  const before = previous.value,
    after = props.run;
  return [
    {
      label: "Completed requests",
      a: String(before.metrics.completed),
      b: String(after.metrics.completed),
    },
    {
      label: "Failed requests",
      a: String(before.metrics.failed),
      b: String(after.metrics.failed),
    },
    {
      label: "P95 completed latency · ms",
      a: fmt(before.metrics.p95LatencyMs),
      b: fmt(after.metrics.p95LatencyMs),
    },
    {
      label: "Duplicate writes",
      a: String(before.metrics.duplicateWrites),
      b: String(after.metrics.duplicateWrites),
    },
    {
      label: "Run horizon · ms",
      a: fmt(before.metrics.durationMs),
      b: fmt(after.metrics.durationMs),
    },
    {
      label: "Modelled host energy · Wh",
      a: before.infrastructure
        ? fmt(before.infrastructure.energyWh, 6)
        : "Not modelled",
      b: after.infrastructure
        ? fmt(after.infrastructure.energyWh, 6)
        : "Not modelled",
    },
    {
      label: "Requirement checks passed",
      a: `${before.verdicts.filter((v) => v.passed).length} / ${before.verdicts.length}`,
      b: `${after.verdicts.filter((v) => v.passed).length} / ${after.verdicts.length}`,
    },
  ];
});
watch(
  () => props.run.id,
  () => {
    generation++;
    selected.value = "";
    previous.value = null;
    loading.value = false;
    error.value = "";
  },
);
watch(selected, async (id) => {
  const request = ++generation;
  previous.value = null;
  error.value = "";
  if (!id) {
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    const run = await api<Run>(`/runs/${encodeURIComponent(id)}`);
    if (request === generation) previous.value = run;
  } catch (e) {
    if (request === generation)
      error.value =
        e instanceof Error ? e.message : "Could not load comparison.";
  } finally {
    if (request === generation) loading.value = false;
  }
});
</script>

<template>
  <section v-if="run.infrastructure" class="infrastructure-evidence">
    <div class="infra-result-heading">
      <div>
        <span class="section-eyebrow">INFRASTRUCTURE EVIDENCE</span>
        <h3>Placement, availability & energy</h3>
      </div>
      <span class="snapshot-label"
        >Recorded model · {{ run.modelHash.slice(0, 8) }}</span
      >
    </div>
    <div class="infra-metric-strip">
      <div>
        <Zap :size="17" /><span
          ><strong
            >{{ fmt(run.infrastructure.energyWh, 6) }} <small>Wh</small></strong
          ><em>Modelled host energy</em></span
        >
      </div>
      <div>
        <Cpu :size="17" /><span
          ><strong>{{ interruptions }}</strong
          ><em>Interrupted processing jobs</em></span
        >
      </div>
      <div>
        <ArrowLeftRight :size="17" /><span
          ><strong>{{ drops }}</strong
          ><em>Dropped link transfers</em></span
        >
      </div>
    </div>
    <details class="infra-details">
      <summary>
        Host and link measurements
        <span
          >{{ run.infrastructure.hosts.length }} hosts ·
          {{ run.infrastructure.links.length }} links</span
        >
      </summary>
      <div class="infra-table-scroll">
        <table class="infra-evidence-table">
          <thead>
            <tr>
              <th>HOST</th>
              <th>UPTIME · ms</th>
              <th>BUSY CORE-TIME · ms</th>
              <th>INTERRUPTED JOBS</th>
              <th>ENERGY · Wh</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="h in run.infrastructure.hosts" :key="h.hostId">
              <td>{{ hostName(run, h.hostId) }}</td>
              <td>{{ fmt(h.uptimeMs) }}</td>
              <td>{{ fmt(h.busyCoreMs) }}</td>
              <td>{{ h.interruptedJobs }}</td>
              <td>{{ fmt(h.energyWh, 6) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="run.infrastructure.links.length" class="infra-table-scroll">
        <table class="infra-evidence-table">
          <thead>
            <tr>
              <th>PHYSICAL LINK</th>
              <th>ATTEMPTED TRANSFERS</th>
              <th>DELIVERED BYTES</th>
              <th>DROPPED TRANSFERS</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="l in run.infrastructure.links" :key="l.linkId">
              <td>{{ linkName(run, l.linkId) }}</td>
              <td>{{ l.transfers }}</td>
              <td>{{ fmt(l.bytesTransferred, 0) }}</td>
              <td>{{ l.droppedTransfers }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </details>
    <p class="infra-result-note">
      Uncalibrated estimates over this run’s horizon, including configured
      outage transitions. Energy includes all defined hosts, even when
      unassigned, and excludes network/storage-device power and batteries.
    </p>
  </section>
  <section class="run-comparison">
    <div class="comparison-controls">
      <div>
        <h3><ArrowLeftRight :size="17" /> Compare recorded runs</h3>
        <p>
          Compare immutable snapshots. Current editor changes do not alter
          either result.
        </p>
      </div>
      <label class="field"
        >Baseline run<select
          v-model="selected"
          aria-label="Baseline run for comparison"
          :disabled="!candidates.length"
        >
          <option value="">
            {{
              candidates.length
                ? "Choose a recorded run…"
                : "Run another experiment to compare"
            }}
          </option>
          <option v-for="r in candidates" :key="r.id" :value="r.id">
            {{ when(r) }} · {{ r.metrics.completed }}/{{
              r.metrics.requests
            }}
            complete · {{ r.id.slice(-8) }}
          </option>
        </select></label
      >
    </div>
    <p v-if="loading" class="comparison-loading" role="status">
      <LoaderCircle :size="15" class="spin" /> Loading recorded model…
    </p>
    <p v-if="error" class="comparison-error" role="alert">
      <CircleAlert :size="16" /> {{ error }}
    </p>
    <template v-if="previous">
      <div class="comparison-snapshots">
        <div v-for="(r, index) in [previous, run]" :key="r.id">
          <span class="section-eyebrow">{{
            index === 0 ? "BASELINE" : "SELECTED RUN"
          }}</span
          ><strong>{{ r.project.name }}</strong
          ><small>{{ when(r) }} · Engine {{ r.engineVersion }}</small
          ><code>{{ r.modelHash.slice(0, 16) }}</code>
          <ul>
            <li v-for="p in placements(r)" :key="p">{{ p }}</li>
          </ul>
        </div>
      </div>
      <p class="comparison-differences">
        <CircleAlert :size="15" /><span
          ><strong>{{
            differences.length ? "Changed inputs:" : "Same execution inputs."
          }}</strong>
          {{
            differences.length
              ? differences.join(", ") +
                ". Results may reflect all of these changes."
              : "Names and diagram positions may still differ."
          }}</span
        >
      </p>
      <div class="infra-table-scroll">
        <table class="comparison-table">
          <thead>
            <tr>
              <th>MEASURE</th>
              <th>BASELINE</th>
              <th>SELECTED RUN</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in metricRows" :key="row.label">
              <td>{{ row.label }}</td>
              <td>{{ row.a }}</td>
              <td :class="{ changed: row.a !== row.b }">{{ row.b }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="infra-result-note">
        P95 describes completed requests only; compare completion counts
        alongside latency. Energy depends on the run horizon as well as
        placement.
      </p>
    </template>
  </section>
</template>

<style scoped>
.infrastructure-evidence,
.run-comparison {
  background: #fff;
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 22px;
}
.infra-result-heading {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}
.infra-result-heading .section-eyebrow {
  font-size: 8px;
}
.infra-result-heading h3 {
  font-size: 15px;
  font-weight: 550;
  margin: 7px 0 0;
}
.snapshot-label {
  font:
    10px Consolas,
    monospace;
  color: #8c9dab;
  background: #f4f7fa;
  border: 1px solid #e8edf2;
  padding: 6px 9px;
  border-radius: 5px;
}
.infra-metric-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
  padding: 22px 0 18px;
  margin-top: 5px;
}
.infra-metric-strip > div {
  display: flex;
  align-items: flex-start;
  gap: 11px;
}
.infra-metric-strip svg {
  color: #54a392;
  margin-top: 4px;
}
.infra-metric-strip strong {
  display: block;
  color: #33586c;
  font-weight: 500;
  font-size: 23px;
  letter-spacing: -0.5px;
}
.infra-metric-strip strong small {
  font-size: 12px;
  color: #8ba4b1;
  font-weight: 400;
}
.infra-metric-strip em {
  display: block;
  font-style: normal;
  font-size: 10px;
  color: #8d9eaa;
  margin-top: 5px;
}
.infra-details {
  border-top: 1px solid #e8eef3;
  padding-top: 12px;
}
.infra-details summary {
  cursor: pointer;
  font-size: 11px;
  color: #58758a;
  padding: 4px 0;
}
.infra-details summary span {
  font-size: 10px;
  color: #93a6b4;
  margin-left: 8px;
}
.infra-table-scroll {
  overflow: auto;
}
.infra-evidence-table,
.comparison-table {
  border-collapse: collapse;
  width: 100%;
  font-size: 11px;
  margin-top: 14px;
  min-width: 510px;
}
.infra-evidence-table th,
.comparison-table th {
  text-align: left;
  font-size: 8px;
  letter-spacing: 0.4px;
  font-weight: 500;
  color: #91a2b1;
  padding: 10px 12px;
  background: #f7f9fb;
  white-space: nowrap;
}
.infra-evidence-table td,
.comparison-table td {
  border-bottom: 1px solid #edf1f5;
  padding: 11px 12px;
  color: #5e7588;
}
.infra-evidence-table td:first-child {
  font-weight: 500;
  color: #405f75;
}
.infra-result-note {
  font-size: 10px;
  color: #92a2b0;
  line-height: 1.8;
  margin: 14px 0 0;
}
.comparison-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 24px;
}
.comparison-controls h3 {
  display: flex;
  align-items: center;
  gap: 9px;
  font-size: 14px;
  font-weight: 550;
  margin: 0 0 8px;
}
.comparison-controls h3 svg {
  color: #66a49a;
}
.comparison-controls p {
  font-size: 10px;
  color: #94a3b2;
  line-height: 1.7;
  margin: 0;
}
.comparison-controls .field {
  width: 330px;
  max-width: 50%;
  margin: 0;
  flex-shrink: 0;
  font-size: 10px;
}
.comparison-controls select {
  width: 100%;
  font-size: 11px;
}
.comparison-loading,
.comparison-error {
  display: flex;
  gap: 8px;
  align-items: center;
  margin: 17px 0 0;
  font-size: 11px;
  color: #6d8d9d;
}
.comparison-error {
  color: #b16c50;
}
.comparison-snapshots {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 15px;
  margin-top: 20px;
}
.comparison-snapshots > div {
  padding: 15px;
  border: 1px solid #e5ecef;
  border-radius: 6px;
  background: #fafcfd;
  min-width: 0;
}
.comparison-snapshots .section-eyebrow {
  font-size: 8px;
}
.comparison-snapshots strong {
  display: block;
  color: #416175;
  font-size: 12px;
  margin: 8px 0;
}
.comparison-snapshots small {
  display: block;
  font-size: 9px;
  color: #90a0b0;
}
.comparison-snapshots code {
  display: block;
  color: #99aab7;
  font-size: 9px;
  margin-top: 7px;
}
.comparison-snapshots ul {
  padding-left: 15px;
  margin: 12px 0 0;
  color: #7c93a3;
  font-size: 10px;
  line-height: 1.9;
  overflow-wrap: anywhere;
}
.comparison-differences {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 10px;
  line-height: 1.8;
  color: #8b8d73;
  padding: 11px 13px;
  margin: 13px 0 0;
  background: #faf8ee;
  border: 1px solid #eeead6;
  border-radius: 5px;
}
.comparison-differences > svg {
  flex-shrink: 0;
  margin-top: 1px;
}
.comparison-differences strong {
  font-weight: 550;
}
.comparison-table td.changed {
  color: #127e70;
  font-weight: 600;
  background: #f1faf7;
}
@media (max-width: 850px) {
  .comparison-controls {
    flex-wrap: wrap;
    gap: 13px;
  }
  .comparison-controls .field {
    max-width: none;
    width: 100%;
  }
  .infra-metric-strip {
    gap: 10px;
  }
  .infra-metric-strip strong {
    font-size: 19px;
  }
  .infra-metric-strip em {
    font-size: 9px;
  }
  .infra-result-heading {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .snapshot-label {
    font-size: 9px;
  }
}
@media (max-width: 600px) {
  .infrastructure-evidence,
  .run-comparison {
    padding: 15px;
    margin-bottom: 16px;
  }
  .infra-metric-strip {
    grid-template-columns: 1fr;
    gap: 17px;
  }
  .infra-metric-strip > div {
    align-items: center;
  }
  .infra-metric-strip strong {
    font-size: 21px;
  }
  .infra-metric-strip em {
    margin-top: 2px;
  }
  .comparison-snapshots {
    grid-template-columns: 1fr;
  }
  .infra-result-heading h3 {
    font-size: 14px;
  }
  .infra-details summary span {
    display: block;
    margin: 7px 0 0;
  }
  .comparison-controls h3 {
    font-size: 13px;
  }
}
</style>
