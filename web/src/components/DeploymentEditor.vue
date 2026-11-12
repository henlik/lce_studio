<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  ArrowRight,
  Cable,
  CircleAlert,
  Cpu,
  Globe2,
  MapPin,
  Plus,
  Server,
  Trash2,
  Zap,
} from "lucide-vue-next";
import OutageWindows from "./OutageWindows.vue";
import type {
  ComponentNode,
  Host,
  HostTier,
  NetworkLink,
  Project,
} from "../types";

const props = defineProps<{
  project: Project;
  busy: boolean;
  activeHostId?: string;
  activeLinkId?: string;
  replayTimeMs?: number;
}>();
const emit = defineEmits<{ "load-example": [] }>();
const tab = ref<"placements" | "sites" | "hosts" | "links">("placements");
const mapMode = ref<"topology" | "geography">("topology");
const selectedSite = ref("");
const selectedHost = ref("");
const selectedLink = ref("");
const infra = computed(() => props.project.infrastructure);
const site = computed(() =>
  infra.value?.sites.find((s) => s.id === selectedSite.value),
);
const host = computed(() =>
  infra.value?.hosts.find((h) => h.id === selectedHost.value),
);
const link = computed(() =>
  infra.value?.links.find((l) => l.id === selectedLink.value),
);
const tiers: HostTier[] = ["device", "edge", "regional", "cloud"];
const id = (prefix: string) => `${prefix}-${crypto.randomUUID().slice(0, 8)}`;
const unplaced = computed(
  () =>
    props.project.nodes.filter(
      (n) => !infra.value?.placements.some((p) => p.nodeId === n.id),
    ).length,
);
const totalCores = computed(
  () =>
    infra.value?.hosts.reduce((sum, h) => sum + (Number(h.cpuCores) || 0), 0) ||
    0,
);
const totalMemory = computed(
  () =>
    infra.value?.hosts.reduce((sum, h) => sum + (Number(h.memoryMb) || 0), 0) ||
    0,
);
const outageCount = computed(
  () =>
    (infra.value?.hosts.reduce((sum, h) => sum + h.outages.length, 0) || 0) +
    (infra.value?.links.reduce((sum, l) => sum + l.outages.length, 0) || 0),
);
const hostPositions = computed(() => {
  const counts = new Map<string, number>();
  return new Map(
    (infra.value?.hosts || []).map((h) => {
      const row = counts.get(h.tier) || 0;
      counts.set(h.tier, row + 1);
      return [
        h.id,
        { x: 20 + Math.max(0, tiers.indexOf(h.tier)) * 225, y: 48 + row * 122 },
      ];
    }),
  );
});
const physicalLines = computed(() =>
  (infra.value?.links || []).flatMap((l) => {
    const a = hostPositions.value.get(l.sourceHostId);
    const b = hostPositions.value.get(l.targetHostId);
    if (!a || !b) return [];
    const span = Math.abs(b.x - a.x);
    if (span > 225) {
      // Longer links leave the bottom ports and bow below the host rows, so
      // they remain visible when another host occupies an intermediate tier.
      const bottom = Math.max(
        ...Array.from(hostPositions.value.values()).map((p) => p.y + 92),
      );
      const bend = bottom + 28 + (span / 225) * 20;
      return [
        {
          link: l,
          path: `M ${a.x + 95} ${a.y + 92} C ${a.x + 95} ${bend}, ${b.x + 95} ${bend}, ${b.x + 95} ${b.y + 92}`,
          bottom: bend,
        },
      ];
    }
    return [
      {
        link: l,
        path: `M ${a.x + 95} ${a.y + 40} L ${b.x + 95} ${b.y + 40}`,
        bottom: Math.max(a.y, b.y) + 92,
      },
    ];
  }),
);
const topologyHeight = computed(() =>
  Math.max(
    232,
    ...Array.from(hostPositions.value.values()).map((p) => p.y + 110),
    ...physicalLines.value.map((line) => line.bottom + 20),
  ),
);
const geoSites = computed(() => {
  const sites = infra.value?.sites || [];
  if (!sites.length) return [];
  const lats = sites.map((s) => (Number.isFinite(s.latitude) ? s.latitude : 0));
  const lons = sites.map((s) =>
    Number.isFinite(s.longitude) ? s.longitude : 0,
  );
  const minLat = Math.min(...lats),
    maxLat = Math.max(...lats);
  const minLon = Math.min(...lons),
    maxLon = Math.max(...lons);
  const lonSpan = Math.max(0.1, maxLon - minLon),
    latSpan = Math.max(0.1, maxLat - minLat);
  return sites.map((s, i) => ({
    ...s,
    x: 110 + ((lons[i] - (minLon + maxLon) / 2) / lonSpan + 0.5) * 680,
    y: 45 + (0.5 - (lats[i] - (minLat + maxLat) / 2) / latSpan) * 190,
    hostCount: infra.value!.hosts.filter((h) => h.siteId === s.id).length,
  }));
});
const geoLines = computed(() =>
  (infra.value?.links || []).flatMap((l) => {
    const a = infra.value!.hosts.find((h) => h.id === l.sourceHostId);
    const b = infra.value!.hosts.find((h) => h.id === l.targetHostId);
    const pa = geoSites.value.find((s) => s.id === a?.siteId);
    const pb = geoSites.value.find((s) => s.id === b?.siteId);
    return pa && pb && pa.id !== pb.id ? [{ id: l.id, a: pa, b: pb }] : [];
  }),
);
watch(
  () => [
    props.project.id,
    infra.value?.sites.map((s) => s.id).join(),
    infra.value?.hosts.map((h) => h.id).join(),
    infra.value?.links.map((l) => l.id).join(),
  ],
  () => {
    if (!infra.value?.sites.some((s) => s.id === selectedSite.value))
      selectedSite.value = infra.value?.sites[0]?.id || "";
    if (!infra.value?.hosts.some((h) => h.id === selectedHost.value))
      selectedHost.value = infra.value?.hosts[0]?.id || "";
    if (!infra.value?.links.some((l) => l.id === selectedLink.value))
      selectedLink.value = infra.value?.links[0]?.id || "";
  },
  { immediate: true },
);

function enable() {
  const siteId = id("site"),
    hostId = id("host");
  if (props.project.schemaVersion !== 3) props.project.schemaVersion = 2;
  props.project.infrastructure = {
    sites: [{ id: siteId, name: "Local site", latitude: 0, longitude: 0 }],
    hosts: [
      {
        id: hostId,
        name: "Local edge server",
        siteId,
        tier: "edge",
        cpuCores: 4,
        memoryMb: 4096,
        idleWatts: 15,
        activeWatts: 45,
        outages: [],
      },
    ],
    links: [],
    placements: props.project.nodes.map((n) => ({ nodeId: n.id, hostId })),
    messageBytes: 1024,
  };
}
function disable() {
  if (
    !confirm(
      "Remove this infrastructure model and return to application-only simulation?",
    )
  )
    return;
  delete props.project.infrastructure;
  if (props.project.schemaVersion !== 3) {
    props.project.schemaVersion = 1;
    props.project.nodes.forEach((n) => delete n.config.memoryMb);
  }
}
function memoryBounds(node: ComponentNode) {
  const reference = node.component;
  return reference
    ? props.project.packages?.find(
        (p) =>
          p.package.id === reference.packageId &&
          p.package.version === reference.version &&
          p.digest === reference.digest,
      )?.package.config.memoryMb
    : undefined;
}
function addSite() {
  if (!infra.value || infra.value.sites.length >= 30) return;
  const s = { id: id("site"), name: "New site", latitude: 0, longitude: 0 };
  infra.value.sites.push(s);
  selectedSite.value = s.id;
}
function removeHosts(ids: string[]) {
  if (!infra.value) return;
  infra.value.hosts = infra.value.hosts.filter((h) => !ids.includes(h.id));
  infra.value.links = infra.value.links.filter(
    (l) => !ids.includes(l.sourceHostId) && !ids.includes(l.targetHostId),
  );
  infra.value.placements = infra.value.placements.filter(
    (p) => !ids.includes(p.hostId),
  );
}
function removeSite() {
  if (!infra.value || !site.value) return;
  const selected = site.value.id;
  removeHosts(
    infra.value.hosts.filter((h) => h.siteId === selected).map((h) => h.id),
  );
  infra.value.sites = infra.value.sites.filter((s) => s.id !== selected);
}
function addHost() {
  if (
    !infra.value ||
    infra.value.hosts.length >= 30 ||
    !infra.value.sites.length
  )
    return;
  const h: Host = {
    id: id("host"),
    name: "New host",
    siteId: infra.value.sites[0].id,
    tier: "edge",
    cpuCores: 2,
    memoryMb: 2048,
    idleWatts: 10,
    activeWatts: 30,
    outages: [],
  };
  infra.value.hosts.push(h);
  selectedHost.value = h.id;
}
function addLink() {
  if (!infra.value || infra.value.links.length >= 60) return;
  const hosts = infra.value.hosts;
  for (let a = 0; a < hosts.length; a++)
    for (let b = a + 1; b < hosts.length; b++) {
      if (
        infra.value.links.some(
          (l) =>
            [l.sourceHostId, l.targetHostId].includes(hosts[a].id) &&
            [l.sourceHostId, l.targetHostId].includes(hosts[b].id),
        )
      )
        continue;
      const l: NetworkLink = {
        id: id("net"),
        sourceHostId: hosts[a].id,
        targetHostId: hosts[b].id,
        latencyMs: 20,
        bandwidthKbps: 1000,
        outages: [],
      };
      infra.value.links.push(l);
      selectedLink.value = l.id;
      return;
    }
}
const canAddLink = computed(
  () =>
    !!infra.value &&
    infra.value.links.length <
      Math.min(
        60,
        (infra.value.hosts.length * (infra.value.hosts.length - 1)) / 2,
      ),
);
function assign(nodeId: string, event: Event) {
  if (!infra.value) return;
  const hostId = (event.target as HTMLSelectElement).value;
  infra.value.placements = infra.value.placements.filter(
    (p) => p.nodeId !== nodeId,
  );
  if (hostId) infra.value.placements.push({ nodeId, hostId });
}
function editHost(hostId: string) {
  selectedHost.value = hostId;
  tab.value = "hosts";
}
function editSite(siteId: string) {
  selectedSite.value = siteId;
  tab.value = "sites";
}
function editLink(linkId: string) {
  selectedLink.value = linkId;
  tab.value = "links";
}
function assignedNodes(hostId: string) {
  return props.project.nodes.filter((n) =>
    infra.value?.placements.some(
      (p) => p.nodeId === n.id && p.hostId === hostId,
    ),
  );
}
function memory(hostId: string) {
  return assignedNodes(hostId).reduce(
    (sum, n) => sum + (Number(n.config.memoryMb) || 64),
    0,
  );
}
function hostName(hostId: string) {
  return infra.value?.hosts.find((h) => h.id === hostId)?.name || hostId;
}
function siteName(siteId: string) {
  return infra.value?.sites.find((s) => s.id === siteId)?.name || siteId;
}
function down(windows: { startMs: number; endMs: number }[]) {
  return (
    props.replayTimeMs !== undefined &&
    windows.some(
      (w) => props.replayTimeMs! >= w.startMs && props.replayTimeMs! < w.endMs,
    )
  );
}
function concise(s: string, length = 23) {
  return s.length > length ? s.slice(0, length - 1) + "…" : s;
}
function serialization(l: NetworkLink) {
  return l.bandwidthKbps > 0
    ? Math.ceil(((infra.value?.messageBytes || 0) * 8) / l.bandwidthKbps)
    : "—";
}
</script>

<template>
  <section class="deployment-workspace">
    <div v-if="!infra" class="deployment-welcome">
      <span class="deployment-emblem"><Globe2 :size="30" /></span>
      <span class="section-eyebrow">FROM APPLICATION TO INFRASTRUCTURE</span>
      <h3>Give your design a place to run.</h3>
      <p>
        Place components on hosts. Set processing resources, physical links,
        locations, and availability. Then compare what happens when the
        environment changes.
      </p>
      <div class="deployment-welcome-actions">
        <button class="button primary" @click="enable" :disabled="busy">
          <Plus :size="16" /> Enable deployment model</button
        ><button
          class="button secondary"
          @click="emit('load-example')"
          :disabled="busy"
        >
          <Globe2 :size="16" /> Open continuum example
        </button>
      </div>
      <div class="deployment-capabilities">
        <span><Cpu :size="17" /> Shared CPU & memory budgets</span
        ><span><Cable :size="17" /> Bandwidth & delay</span
        ><span><Zap :size="17" /> Power interruptions</span>
      </div>
    </div>
    <template v-else>
      <div class="deployment-overview">
        <div>
          <span class="section-eyebrow">DEPLOYMENT MODEL</span>
          <p>
            {{ infra.hosts.length }} hosts across {{ infra.sites.length }} sites
            <span>·</span> {{ totalCores }} CPU cores <span>·</span>
            {{ totalMemory.toLocaleString() }} MiB
          </p>
        </div>
        <div class="deployment-overview-actions">
          <span class="deployment-badge" :class="{ warning: unplaced }">{{
            unplaced ? `${unplaced} unplaced` : "All components placed"
          }}</span
          ><button
            class="button quiet"
            @click="emit('load-example')"
            :disabled="busy"
          >
            Load example <ArrowRight :size="14" />
          </button>
        </div>
      </div>
      <div class="deployment-topology">
        <div class="deployment-map-toolbar">
          <div>
            <Globe2 :size="15" /><strong>{{
              mapMode === "topology" ? "Physical topology" : "Site coordinates"
            }}</strong
            ><span
              v-if="replayTimeMs !== undefined"
              class="deployment-replay-time"
              >Replay · {{ replayTimeMs }} ms</span
            >
          </div>
          <div
            class="deployment-toggle"
            role="group"
            aria-label="Infrastructure visualization"
          >
            <button
              :class="{ active: mapMode === 'topology' }"
              :aria-pressed="mapMode === 'topology'"
              @click="mapMode = 'topology'"
            >
              Topology</button
            ><button
              :class="{ active: mapMode === 'geography' }"
              :aria-pressed="mapMode === 'geography'"
              @click="mapMode = 'geography'"
            >
              Geography
            </button>
          </div>
        </div>
        <div class="deployment-svg-scroll">
          <svg
            v-if="mapMode === 'topology'"
            class="deployment-svg"
            :viewBox="`0 0 900 ${topologyHeight}`"
            role="img"
            aria-label="Hosts and direct physical network links"
          >
            <defs>
              <pattern
                id="deployment-grid"
                width="20"
                height="20"
                patternUnits="userSpaceOnUse"
              >
                <circle cx="1" cy="1" r="0.7" fill="#d5e0e9" />
              </pattern>
            </defs>
            <rect
              width="900"
              :height="topologyHeight"
              fill="url(#deployment-grid)"
            />
            <text
              v-for="(tier, i) in tiers"
              :key="tier"
              :x="25 + i * 225"
              y="25"
              class="topology-tier"
            >
              {{ tier.toUpperCase() }}
            </text>
            <g
              v-for="line in physicalLines"
              :key="line.link.id"
              class="physical-link"
              :class="{
                selected: line.link.id === activeLinkId,
                down: down(line.link.outages),
              }"
              @click="editLink(line.link.id)"
              role="button"
              tabindex="0"
              :aria-label="`Edit link ${hostName(line.link.sourceHostId)} to ${hostName(line.link.targetHostId)}`"
              @keydown.enter="editLink(line.link.id)"
              @keydown.space.prevent="editLink(line.link.id)"
            >
              <title>
                {{ hostName(line.link.sourceHostId) }} ↔
                {{ hostName(line.link.targetHostId) }} ·
                {{ line.link.bandwidthKbps }} kbit/s ·
                {{ line.link.latencyMs }} ms
              </title>
              <path :d="line.path" class="physical-link-hit" />
              <path :d="line.path" class="physical-link-line" />
            </g>
            <g
              v-for="h in infra.hosts"
              :key="h.id"
              :transform="`translate(${hostPositions.get(h.id)?.x}, ${hostPositions.get(h.id)?.y})`"
              class="physical-host"
              :class="[
                h.tier,
                { active: h.id === activeHostId, down: down(h.outages) },
              ]"
              role="button"
              tabindex="0"
              :aria-label="`Edit host ${h.name}`"
              @click="editHost(h.id)"
              @keydown.enter="editHost(h.id)"
              @keydown.space.prevent="editHost(h.id)"
            >
              <title>
                {{ h.name }} · {{ siteName(h.siteId) }} ·
                {{
                  assignedNodes(h.id)
                    .map((n) => n.name)
                    .join(", ") || "No components placed"
                }}
              </title>
              <rect width="190" height="92" rx="9" class="host-box" />
              <rect
                x="0"
                y="0"
                width="5"
                height="92"
                rx="2"
                class="host-accent"
              />
              <text x="16" y="24" class="host-title">
                {{ concise(h.name) }}
              </text>
              <text x="16" y="43" class="host-resources">
                {{ h.cpuCores }} cores · {{ h.memoryMb }} MiB
              </text>
              <text x="16" y="63" class="host-site">
                {{ concise(siteName(h.siteId), 27) }}
              </text>
              <text x="16" y="80" class="host-apps">
                {{ assignedNodes(h.id).length }} components
                <tspan v-if="h.outages.length">
                  · {{ h.outages.length }} power outages
                </tspan>
              </text>
              <circle
                v-if="down(h.outages)"
                cx="174"
                cy="19"
                r="4"
                fill="#d9654a"
              />
            </g>
            <text
              v-if="!infra.hosts.length"
              x="450"
              y="120"
              text-anchor="middle"
              class="topology-empty"
            >
              Add hosts to define your infrastructure.
            </text>
          </svg>
          <svg
            v-else
            class="deployment-svg geography-svg"
            viewBox="0 0 900 310"
            role="img"
            aria-label="Relative geographic positions of sites"
          >
            <defs>
              <pattern
                id="geography-grid"
                width="50"
                height="50"
                patternUnits="userSpaceOnUse"
              >
                <path
                  d="M 50 0 L 0 0 0 50"
                  fill="none"
                  stroke="#dce8ee"
                  stroke-width="0.8"
                />
              </pattern>
            </defs>
            <rect width="900" height="310" fill="url(#geography-grid)" />
            <text x="18" y="26" class="topology-tier">N ↑</text>
            <line
              v-for="l in geoLines"
              :key="l.id"
              :x1="l.a.x"
              :y1="l.a.y"
              :x2="l.b.x"
              :y2="l.b.y"
              stroke="#93b5c4"
              stroke-width="1.7"
              stroke-dasharray="5 4"
            />
            <g
              v-for="s in geoSites"
              :key="s.id"
              :transform="`translate(${s.x}, ${s.y})`"
              class="geo-site"
              role="button"
              tabindex="0"
              :aria-label="`Edit site ${s.name}`"
              @click="editSite(s.id)"
              @keydown.enter="editSite(s.id)"
              @keydown.space.prevent="editSite(s.id)"
            >
              <circle r="15" fill="#0f97881a" />
              <circle r="6" fill="#0f9788" stroke="white" stroke-width="2" />
              <text y="31" text-anchor="middle" class="host-title">
                {{ concise(s.name, 25) }}
              </text>
              <text y="48" text-anchor="middle" class="host-resources">
                {{ s.latitude }}°, {{ s.longitude }}° · {{ s.hostCount }} hosts
              </text>
            </g>
            <text
              v-if="!geoSites.length"
              x="450"
              y="140"
              text-anchor="middle"
              class="topology-empty"
            >
              Add a site and its coordinates.
            </text>
          </svg>
        </div>
        <div class="deployment-map-caption">
          <span>{{
            mapMode === "topology"
              ? "Select a host or link to edit it. Physical links are bidirectional."
              : "Coordinates scaled to this project’s extent. No basemap; geography does not infer latency."
          }}</span
          ><span>{{ outageCount }} configured outages</span>
        </div>
      </div>

      <div class="deployment-editor-body">
        <nav class="deployment-tabs" aria-label="Deployment settings">
          <button
            :class="{ active: tab === 'placements' }"
            @click="tab = 'placements'"
          >
            <Server :size="15" /> Placement
            <span
              >{{ infra.placements.length }}/{{ project.nodes.length }}</span
            >
          </button>
          <button :class="{ active: tab === 'sites' }" @click="tab = 'sites'">
            <MapPin :size="15" /> Sites <span>{{ infra.sites.length }}</span>
          </button>
          <button :class="{ active: tab === 'hosts' }" @click="tab = 'hosts'">
            <Cpu :size="15" /> Hosts <span>{{ infra.hosts.length }}</span>
          </button>
          <button :class="{ active: tab === 'links' }" @click="tab = 'links'">
            <Cable :size="15" /> Links <span>{{ infra.links.length }}</span>
          </button>
        </nav>
        <div class="deployment-settings">
          <template v-if="tab === 'placements'">
            <div class="deployment-section-heading">
              <div>
                <h3>Application placement</h3>
                <p>
                  Choose a host for each component. Memory is reserved; CPU
                  cores are shared while processing.
                </p>
              </div>
              <label class="field message-size"
                >Message payload · bytes<input
                  v-model.number="infra.messageBytes"
                  type="number"
                  min="1"
                  max="10485760"
                  step="1"
                  aria-label="Message payload bytes"
                  :disabled="busy"
              /></label>
            </div>
            <div class="placement-table-wrap">
              <table class="placement-table">
                <thead>
                  <tr>
                    <th>APPLICATION COMPONENT</th>
                    <th>HOST</th>
                    <th>MEMORY · MiB</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="n in project.nodes" :key="n.id">
                    <td>
                      <strong>{{ n.name }}</strong
                      ><small>{{ n.type }}</small>
                    </td>
                    <td>
                      <select
                        :value="
                          infra.placements.find((p) => p.nodeId === n.id)
                            ?.hostId || ''
                        "
                        :aria-label="`Host for ${n.name}`"
                        :disabled="busy"
                        @change="assign(n.id, $event)"
                      >
                        <option value="">Choose a host…</option>
                        <option
                          v-for="h in infra.hosts"
                          :key="h.id"
                          :value="h.id"
                        >
                          {{ h.name }} · {{ h.tier }}
                        </option>
                      </select>
                    </td>
                    <td>
                      <input
                        v-model.number="n.config.memoryMb"
                        type="number"
                        :min="memoryBounds(n)?.min ?? 0"
                        :max="memoryBounds(n)?.max ?? 1048576"
                        step="1"
                        placeholder="64"
                        :aria-label="`Memory MiB for ${n.name}`"
                        :disabled="busy"
                      />
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p class="deployment-help">
              An omitted or zero memory reservation uses 64 MiB. Components on
              different hosts need a direct physical link; multi-hop routing is
              not yet supported.
            </p>
            <div class="host-budgets">
              <div
                v-for="h in infra.hosts"
                :key="h.id"
                class="host-budget"
                :class="{ exceeded: memory(h.id) > h.memoryMb }"
              >
                <span
                  ><strong>{{ h.name }}</strong
                  ><CircleAlert v-if="memory(h.id) > h.memoryMb" :size="14"
                /></span>
                <div class="memory-track">
                  <i
                    :style="{
                      width: `${Math.min(100, Math.max(0, (memory(h.id) / Math.max(1, h.memoryMb)) * 100))}%`,
                    }"
                  />
                </div>
                <small
                  >{{ memory(h.id).toLocaleString() }} /
                  {{ h.memoryMb.toLocaleString() }} MiB reserved</small
                >
              </div>
            </div>
          </template>

          <template v-else-if="tab === 'sites'">
            <div class="deployment-section-heading">
              <div>
                <h3>Geographic sites</h3>
                <p>
                  Locations describe your deployment. Link properties control
                  network behaviour.
                </p>
              </div>
              <button
                class="button secondary"
                @click="addSite"
                :disabled="busy || infra.sites.length >= 30"
              >
                <Plus :size="14" /> Add site
              </button>
            </div>
            <label v-if="infra.sites.length" class="field resource-picker"
              >Selected site<select
                v-model="selectedSite"
                aria-label="Selected site"
              >
                <option v-for="s in infra.sites" :key="s.id" :value="s.id">
                  {{ s.name }}
                </option>
              </select></label
            >
            <div v-if="site" class="resource-fields">
              <label class="field wide"
                >Site name<input
                  v-model="site.name"
                  maxlength="120"
                  :disabled="busy" /></label
              ><label class="field"
                >Latitude · degrees<input
                  v-model.number="site.latitude"
                  type="number"
                  min="-90"
                  max="90"
                  step="any"
                  :disabled="busy" /></label
              ><label class="field"
                >Longitude · degrees<input
                  v-model.number="site.longitude"
                  type="number"
                  min="-180"
                  max="180"
                  step="any"
                  :disabled="busy"
              /></label>
              <div class="resource-delete wide">
                <code>{{ site.id }}</code
                ><button
                  class="button quiet danger"
                  @click="removeSite"
                  :disabled="busy"
                >
                  <Trash2 :size="14" /> Delete site and its hosts
                </button>
                <p>
                  Also removes those hosts’ links and placements. Application
                  components remain in the design.
                </p>
              </div>
            </div>
            <div v-else class="deployment-empty">
              Add a site before adding physical hosts.
            </div>
          </template>

          <template v-else-if="tab === 'hosts'">
            <div class="deployment-section-heading">
              <div>
                <h3>Compute hosts</h3>
                <p>
                  Define shared capacity, power consumption, and scheduled power
                  interruptions.
                </p>
              </div>
              <button
                class="button secondary"
                @click="addHost"
                :disabled="
                  busy || !infra.sites.length || infra.hosts.length >= 30
                "
              >
                <Plus :size="14" /> Add host
              </button>
            </div>
            <label v-if="infra.hosts.length" class="field resource-picker"
              >Selected host<select
                v-model="selectedHost"
                aria-label="Selected host"
              >
                <option v-for="h in infra.hosts" :key="h.id" :value="h.id">
                  {{ h.name }}
                </option>
              </select></label
            >
            <template v-if="host"
              ><div class="resource-fields">
                <label class="field wide"
                  >Host name<input
                    v-model="host.name"
                    maxlength="120"
                    :disabled="busy" /></label
                ><label class="field"
                  >Site<select v-model="host.siteId" :disabled="busy">
                    <option v-for="s in infra.sites" :key="s.id" :value="s.id">
                      {{ s.name }}
                    </option>
                  </select></label
                ><label class="field"
                  >Continuum tier<select v-model="host.tier" :disabled="busy">
                    <option v-for="tier in tiers" :key="tier" :value="tier">
                      {{ tier }}
                    </option>
                  </select></label
                ><label class="field"
                  >CPU cores<input
                    v-model.number="host.cpuCores"
                    type="number"
                    min="1"
                    max="128"
                    step="1"
                    :disabled="busy" /></label
                ><label class="field"
                  >Memory capacity · MiB<input
                    v-model.number="host.memoryMb"
                    type="number"
                    min="1"
                    max="1048576"
                    step="1"
                    :disabled="busy" /></label
                ><label class="field"
                  >Idle power · W<input
                    v-model.number="host.idleWatts"
                    type="number"
                    min="0"
                    max="1000000"
                    step="any"
                    :disabled="busy" /></label
                ><label class="field"
                  >Fully active power · W<input
                    v-model.number="host.activeWatts"
                    type="number"
                    :min="host.idleWatts"
                    max="1000000"
                    step="any"
                    :disabled="busy"
                /></label>
              </div>
              <OutageWindows
                v-model="host.outages"
                label="Power outages"
                :disabled="busy"
              />
              <div class="resource-delete">
                <code>{{ host.id }}</code
                ><button
                  class="button quiet danger"
                  @click="removeHosts([host.id])"
                  :disabled="busy"
                >
                  <Trash2 :size="14" /> Delete host
                </button>
                <p>
                  Removes its links and placements. Its application components
                  become unplaced.
                </p>
              </div></template
            >
            <div v-else class="deployment-empty">
              Add a site, then add a host to run your components.
            </div>
          </template>

          <template v-else>
            <div class="deployment-section-heading">
              <div>
                <h3>Physical network links</h3>
                <p>
                  One shared transmission channel per link, serving both
                  directions in FIFO order.
                </p>
              </div>
              <button
                class="button secondary"
                @click="addLink"
                :disabled="busy || !canAddLink"
              >
                <Plus :size="14" /> Add link
              </button>
            </div>
            <label v-if="infra.links.length" class="field resource-picker"
              >Selected link<select
                v-model="selectedLink"
                aria-label="Selected network link"
              >
                <option v-for="l in infra.links" :key="l.id" :value="l.id">
                  {{ hostName(l.sourceHostId) }} ↔
                  {{ hostName(l.targetHostId) }}
                </option>
              </select></label
            >
            <template v-if="link"
              ><div class="resource-fields">
                <label class="field"
                  >First host<select
                    v-model="link.sourceHostId"
                    :disabled="busy"
                  >
                    <option v-for="h in infra.hosts" :key="h.id" :value="h.id">
                      {{ h.name }}
                    </option>
                  </select></label
                ><label class="field"
                  >Second host<select
                    v-model="link.targetHostId"
                    :disabled="busy"
                  >
                    <option v-for="h in infra.hosts" :key="h.id" :value="h.id">
                      {{ h.name }}
                    </option>
                  </select></label
                ><label class="field"
                  >One-way latency · ms<input
                    v-model.number="link.latencyMs"
                    type="number"
                    min="0"
                    max="60000"
                    step="1"
                    :disabled="busy" /></label
                ><label class="field"
                  >Bandwidth · kbit/s<input
                    v-model.number="link.bandwidthKbps"
                    type="number"
                    min="1"
                    max="10000000"
                    step="1"
                    :disabled="busy"
                /></label>
              </div>
              <p class="deployment-help">
                Each {{ infra.messageBytes.toLocaleString() }}-byte message
                needs {{ serialization(link) }} ms of serialization, plus
                physical latency, application-link delay, and scenario jitter.
              </p>
              <OutageWindows
                v-model="link.outages"
                label="Network outages"
                :disabled="busy"
              />
              <div class="resource-delete">
                <code>{{ link.id }}</code
                ><button
                  class="button quiet danger"
                  @click="
                    infra.links = infra.links.filter((l) => l.id !== link!.id)
                  "
                  :disabled="busy"
                >
                  <Trash2 :size="14" /> Delete link
                </button>
              </div></template
            >
            <div v-else class="deployment-empty">
              Add at least two hosts to connect them. Components on the same
              host need no physical link.
            </div>
          </template>
        </div>
      </div>
      <div class="deployment-assumptions">
        <Zap :size="17" />
        <p>
          <strong>Model assumptions.</strong> Power is estimated from CPU
          activity over the run horizon. Scheduled outages interrupt work;
          durable writes remain. Battery state, migration, storage-device
          energy, and geographic latency inference are not modelled in v0.3.
        </p>
        <button class="button quiet" @click="disable" :disabled="busy">
          Remove infrastructure
        </button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.deployment-workspace {
  padding: 22px 28px 28px;
  background: #f4f7fa;
  flex: 1;
  min-width: 0;
}
.deployment-welcome {
  background: white;
  border: 1px solid var(--line);
  border-radius: 12px;
  text-align: center;
  padding: 55px 25px;
  max-width: 1000px;
  margin: 0 auto;
}
.deployment-emblem {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  background: #e9f6f3;
  color: #0e9a89;
  border: 1px solid #ceebe4;
  border-radius: 17px;
  margin: 0 auto 25px;
}
.deployment-welcome .section-eyebrow {
  display: block;
  font-size: 9px;
}
.deployment-welcome h3 {
  font-size: 25px;
  letter-spacing: -0.5px;
  margin: 12px 0;
}
.deployment-welcome p {
  max-width: 540px;
  margin: 0 auto 25px;
  line-height: 1.85;
  color: #76869a;
}
.deployment-welcome-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  flex-wrap: wrap;
}
.deployment-capabilities {
  display: flex;
  justify-content: center;
  gap: 24px;
  flex-wrap: wrap;
  margin-top: 36px;
  padding-top: 22px;
  border-top: 1px solid #edf1f5;
  font-size: 11px;
  color: #73879b;
}
.deployment-capabilities span {
  display: flex;
  align-items: center;
  gap: 7px;
}
.deployment-overview {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.deployment-overview .section-eyebrow {
  font-size: 9px;
}
.deployment-overview p {
  font-size: 12px;
  color: #536b81;
  margin: 7px 0 0;
}
.deployment-overview p span {
  padding: 0 8px;
  color: #a2b3c2;
}
.deployment-overview-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}
.deployment-badge {
  background: #e7f4ef;
  color: #238a74;
  border: 1px solid #cce8dc;
  padding: 5px 9px;
  border-radius: 5px;
  font-size: 10px;
  white-space: nowrap;
}
.deployment-badge.warning {
  background: #fff4e7;
  color: #a26c28;
  border-color: #f1dfc4;
}
.deployment-topology {
  border: 1px solid #dce5ed;
  border-radius: 9px;
  background: #f9fbfd;
  overflow: hidden;
  box-shadow: 0 4px 12px #19344c04;
}
.deployment-map-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 12px 17px;
  border-bottom: 1px solid #e4ebf0;
  background: #fff;
}
.deployment-map-toolbar > div:first-child {
  display: flex;
  gap: 8px;
  align-items: center;
  color: #637c92;
  font-size: 11px;
}
.deployment-map-toolbar strong {
  color: #304d65;
  font-size: 12px;
}
.deployment-replay-time {
  color: #0e9788;
  font-size: 10px;
  border-left: 1px solid var(--line);
  padding-left: 10px;
}
.deployment-toggle {
  display: flex;
  background: #edf2f6;
  border: 1px solid #e1e8ee;
  border-radius: 6px;
  padding: 3px;
}
.deployment-toggle button {
  padding: 5px 10px;
  color: #7f90a0;
  font-size: 10px;
  border-radius: 4px;
  background: transparent;
}
.deployment-toggle button.active {
  background: #fff;
  color: #276657;
  box-shadow: 0 1px 4px #19384f17;
}
.deployment-svg-scroll {
  overflow: auto;
  max-height: 450px;
}
.deployment-svg {
  display: block;
  width: 100%;
  min-width: 650px;
}
.topology-tier {
  font-size: 10px;
  letter-spacing: 2px;
  fill: #a0aebc;
  font-weight: 600;
}
.physical-link {
  cursor: pointer;
}
.physical-link-hit {
  fill: none;
  stroke: transparent;
  stroke-width: 17;
}
.physical-link-line {
  fill: none;
  stroke: #9aafbf;
  stroke-width: 2;
}
.physical-link.selected .physical-link-line {
  stroke: #0ba38b;
  stroke-width: 4;
}
.physical-link.down .physical-link-line {
  stroke: #d48e77;
  stroke-dasharray: 5 5;
}
.physical-link:focus .physical-link-line {
  stroke: #1d8a9f;
  stroke-width: 4;
}
.physical-host,
.geo-site {
  cursor: pointer;
  outline: none;
}
.host-box {
  fill: white;
  stroke: #d6e1e9;
  stroke-width: 1.3;
  filter: drop-shadow(0 2px 3px #20436508);
}
.host-accent {
  fill: #309c8f;
}
.physical-host.device .host-accent {
  fill: #7d9fc1;
}
.physical-host.regional .host-accent {
  fill: #987eb2;
}
.physical-host.cloud .host-accent {
  fill: #6c84b9;
}
.physical-host.active .host-box,
.physical-host:focus .host-box {
  stroke: #0aa78c;
  stroke-width: 3;
}
.physical-host.down .host-box {
  fill: #fff5ef;
  stroke: #d8a18d;
}
.host-title {
  font-size: 12px;
  fill: #2e465d;
  font-weight: 600;
}
.host-resources {
  font-size: 10px;
  fill: #72879b;
}
.host-site {
  font-size: 10px;
  fill: #7b8fa1;
}
.host-apps {
  font-size: 9px;
  fill: #0e8e7c;
}
.topology-empty {
  font-size: 13px;
  fill: #91a0af;
}
.geo-site:focus circle:first-child {
  stroke: #127d91;
  stroke-width: 2;
}
.deployment-map-caption {
  display: flex;
  justify-content: space-between;
  gap: 15px;
  padding: 10px 16px;
  font-size: 10px;
  color: #8a9cad;
  border-top: 1px solid #e4ecf1;
  background: #fff;
}
.deployment-map-caption > span:last-child {
  white-space: nowrap;
}
.deployment-editor-body {
  margin-top: 20px;
  border: 1px solid var(--line);
  border-radius: 9px;
  background: #fff;
  overflow: hidden;
}
.deployment-tabs {
  display: flex;
  gap: 4px;
  padding: 8px 13px 0;
  border-bottom: 1px solid var(--line);
  background: #fbfcfd;
}
.deployment-tabs button {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #8392a2;
  background: none;
  padding: 12px 13px 14px;
  border-bottom: 2px solid transparent;
  font-size: 12px;
}
.deployment-tabs button.active {
  color: #178e7d;
  border-bottom-color: #0d9c88;
}
.deployment-tabs button span {
  font-size: 9px;
  background: #edf2f6;
  border-radius: 4px;
  padding: 2px 5px;
  color: #8d9daa;
}
.deployment-settings {
  padding: 22px 24px 24px;
}
.deployment-section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 19px;
}
.deployment-section-heading h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 7px;
}
.deployment-section-heading p {
  font-size: 11px;
  line-height: 1.7;
  color: #8393a3;
  margin: 0;
  max-width: 580px;
}
.deployment-section-heading > .button {
  white-space: nowrap;
}
.message-size {
  max-width: 165px;
  margin: 0;
  flex-shrink: 0;
  font-size: 10px;
}
.resource-picker {
  max-width: 550px;
  margin-bottom: 20px;
}
.deployment-settings input,
.deployment-settings select {
  width: 100%;
  border: 1px solid #dce5ed;
  border-radius: 5px;
  background: #fbfcfd;
  color: #38536b;
  padding: 9px 10px;
  font-size: 12px;
  min-width: 0;
}
.placement-table-wrap {
  overflow: auto;
}
.placement-table {
  width: 100%;
  border-collapse: collapse;
  min-width: 520px;
}
.placement-table th {
  text-align: left;
  font-size: 9px;
  letter-spacing: 0.6px;
  color: #9aa8b6;
  padding: 0 10px 12px;
}
.placement-table td {
  padding: 12px 10px;
  border-top: 1px solid #edf1f5;
}
.placement-table th:first-child,
.placement-table td:first-child {
  padding-left: 0;
}
.placement-table td:first-child {
  width: 35%;
}
.placement-table td:nth-child(2) {
  width: 45%;
}
.placement-table td strong {
  display: block;
  color: #3c5367;
  font-size: 12px;
  font-weight: 550;
}
.placement-table td small {
  display: block;
  font-size: 10px;
  color: #96a6b4;
  margin-top: 5px;
}
.placement-table td input {
  max-width: 130px;
}
.deployment-help {
  font-size: 11px;
  color: #8a9aab;
  line-height: 1.7;
  margin: 13px 0 0;
}
.host-budgets {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 13px;
  margin-top: 20px;
}
.host-budget {
  border: 1px solid #e1ebe9;
  background: #f8fcfa;
  border-radius: 6px;
  padding: 12px;
}
.host-budget > span {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: #548576;
}
.host-budget strong {
  font-weight: 550;
  font-size: 11px;
}
.host-budget small {
  font-size: 9px;
  color: #7a9a8f;
}
.memory-track {
  height: 4px;
  border-radius: 4px;
  background: #e1ede8;
  margin: 11px 0 7px;
  overflow: hidden;
}
.memory-track i {
  display: block;
  height: 100%;
  background: #54b79b;
  border-radius: 4px;
}
.host-budget.exceeded {
  border-color: #eed1bd;
  background: #fff7ef;
}
.host-budget.exceeded > span {
  color: #b17449;
}
.host-budget.exceeded .memory-track i {
  background: #cf8653;
}
.resource-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 17px;
  max-width: 780px;
}
.resource-fields .field {
  margin: 0;
  font-size: 11px;
  gap: 8px;
}
.resource-fields .wide {
  grid-column: 1/-1;
}
.resource-delete {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 16px;
  border-top: 1px solid #edf1f5;
  padding-top: 13px;
}
.resource-delete code {
  font-size: 10px;
  color: #a0adba;
  overflow-wrap: anywhere;
}
.resource-delete p {
  width: 100%;
  font-size: 10px;
  color: #99a7b5;
  line-height: 1.6;
  margin: 0;
}
.danger {
  color: #b6745c !important;
}
.deployment-empty {
  border: 1px dashed #dfe7ee;
  background: #fafcfd;
  padding: 25px;
  text-align: center;
  color: #93a2af;
  font-size: 12px;
  border-radius: 7px;
}
.deployment-assumptions {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 19px 3px 0;
  color: #8295a6;
}
.deployment-assumptions > svg {
  flex-shrink: 0;
  margin-top: 2px;
  color: #6f9f98;
}
.deployment-assumptions p {
  font-size: 10px;
  line-height: 1.85;
  margin: 0;
  flex: 1;
}
.deployment-assumptions p strong {
  color: #607b8e;
  font-weight: 550;
}
.deployment-assumptions > .button {
  font-size: 10px;
  white-space: nowrap;
  flex-shrink: 0;
  padding: 4px 7px;
}
@media (max-width: 1100px) {
  .deployment-workspace {
    padding: 20px;
  }
  .deployment-overview {
    align-items: flex-start;
  }
  .deployment-overview-actions {
    flex-direction: column;
    align-items: flex-end;
  }
  .deployment-settings {
    padding: 19px;
  }
  .deployment-assumptions {
    flex-wrap: wrap;
  }
  .deployment-assumptions > .button {
    margin-left: 27px;
  }
}
@media (max-width: 650px) {
  .deployment-workspace {
    padding: 14px 12px 20px;
  }
  .deployment-welcome {
    padding: 35px 18px;
  }
  .deployment-welcome h3 {
    font-size: 21px;
  }
  .deployment-capabilities {
    flex-direction: column;
    align-items: center;
    gap: 12px;
  }
  .deployment-welcome-actions .button {
    width: 100%;
    justify-content: center;
  }
  .deployment-overview {
    flex-direction: column;
    gap: 10px;
  }
  .deployment-overview-actions {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    width: 100%;
  }
  .deployment-overview p {
    font-size: 10px;
  }
  .deployment-overview p span {
    padding: 0 4px;
  }
  .deployment-map-toolbar {
    padding: 10px;
    gap: 7px;
    flex-wrap: wrap;
  }
  .deployment-map-toolbar strong {
    font-size: 11px;
  }
  .deployment-map-caption {
    padding: 9px 10px;
    font-size: 9px;
    line-height: 1.6;
  }
  .deployment-tabs {
    padding: 6px 5px 0;
    gap: 0;
    overflow: auto;
  }
  .deployment-tabs button {
    font-size: 10px;
    padding: 10px 9px;
    gap: 5px;
    white-space: nowrap;
  }
  .deployment-tabs button span {
    display: none;
  }
  .deployment-settings {
    padding: 17px 13px;
  }
  .deployment-section-heading {
    align-items: flex-start;
    flex-wrap: wrap;
    gap: 13px;
  }
  .deployment-section-heading .message-size {
    max-width: none;
    width: 100%;
  }
  .deployment-section-heading h3 {
    font-size: 14px;
  }
  .deployment-section-heading p {
    font-size: 10px;
  }
  .resource-fields {
    gap: 13px;
  }
  .resource-fields .field {
    font-size: 10px;
  }
  .host-budgets {
    grid-template-columns: 1fr 1fr;
  }
  .deployment-assumptions p {
    font-size: 9px;
  }
  .resource-delete {
    gap: 5px;
  }
  .resource-delete code {
    font-size: 9px;
  }
}
</style>
