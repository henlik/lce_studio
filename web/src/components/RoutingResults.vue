<script setup lang="ts">
import { computed } from "vue";
import { GitBranch } from "lucide-vue-next";
import type { Run } from "../types";
const props = defineProps<{ run: Run }>();
const routers = computed(() =>
  props.run.project.nodes
    .filter((n) => n.type === "router")
    .map((node) => {
      const outputs = props.run.project.edges.filter(
        (e) => e.source === node.id,
      );
      const events = props.run.events.filter(
        (e) => e.kind === "route_selected" && e.nodeId === node.id,
      );
      const totalWeight = outputs.reduce(
        (sum, e) => sum + (e.routeWeight || 1),
        0,
      );
      return {
        node,
        requests: new Set(events.map((e) => e.requestId)).size,
        rows: outputs.map((edge) => {
          const selected = events.filter((e) => e.edgeId === edge.id);
          return {
            edge,
            target:
              props.run.project.nodes.find((n) => n.id === edge.target)?.name ||
              edge.target,
            requests: new Set(selected.map((e) => e.requestId)).size,
            attempts: selected.length,
            share: new Intl.NumberFormat("en", {
              maximumFractionDigits: 1,
            }).format((100 * (edge.routeWeight || 1)) / totalWeight),
          };
        }),
      };
    }),
);
</script>

<template>
  <section v-if="routers.length" class="routing-evidence">
    <div class="routing-heading">
      <div>
        <span class="section-eyebrow">EXCLUSIVE ROUTING EVIDENCE</span>
        <h3>One selected path per request</h3>
      </div>
      <GitBranch :size="19" />
    </div>
    <p>
      Selections recorded in this run’s trace. Request counts are unique at each
      router; selection events also include retries. A selection does not imply
      successful completion.
    </p>
    <div
      v-for="router in routers"
      :key="router.node.id"
      class="router-evidence"
    >
      <div class="router-evidence-label">
        <strong>{{ router.node.name }}</strong
        ><span>{{ router.requests }} requests reached this router</span>
      </div>
      <div class="routing-table-scroll">
        <table>
          <thead>
            <tr>
              <th>SELECTED OUTPUT</th>
              <th>WEIGHT</th>
              <th>WEIGHT SHARE</th>
              <th>UNIQUE REQUESTS</th>
              <th>SELECTION EVENTS</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in router.rows" :key="row.edge.id">
              <td>
                {{ row.target }}<small>{{ row.edge.id }}</small>
              </td>
              <td>{{ row.edge.routeWeight || 1 }}</td>
              <td>{{ row.share }}%</td>
              <td>{{ row.requests }}</td>
              <td>{{ row.attempts }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <p class="routing-footnote">
      Weights describe relative selection probability, not exact sample counts.
      Routes are deterministic for the recorded seed and request ID, and retries
      use the same route. Branches execute exclusively; convergence does not
      synchronize parallel work.
    </p>
  </section>
</template>

<style scoped>
.routing-evidence {
  border: 1px solid #e2e9ed;
  border-radius: 9px;
  background: #fff;
  padding: 21px;
  margin-bottom: 20px;
}
.routing-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: #b29458;
}
.routing-heading h3 {
  color: #3b5266;
  font-size: 16px;
  margin: 7px 0 0;
}
.routing-evidence > p {
  color: #8a9baa;
  font-size: 11px;
  line-height: 1.7;
  margin: 12px 0 17px;
}
.router-evidence {
  margin-top: 16px;
}
.router-evidence-label {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  margin-bottom: 10px;
}
.router-evidence-label strong {
  color: #566e80;
  font-size: 12px;
}
.router-evidence-label span {
  color: #9ba8b4;
  font-size: 10px;
}
.routing-table-scroll {
  overflow: auto;
}
table {
  border-collapse: collapse;
  width: 100%;
  font-size: 11px;
  text-align: left;
  white-space: nowrap;
}
th {
  font-size: 8px;
  letter-spacing: 0.5px;
  color: #94a2ae;
  background: #f7f9fa;
  padding: 10px;
}
td {
  padding: 12px 10px;
  color: #61788b;
  border-bottom: 1px solid #edf1f4;
}
td small {
  display: block;
  font-size: 9px;
  color: #a0aeb9;
  margin-top: 5px;
}
.routing-evidence .routing-footnote {
  font-size: 10px;
  margin-bottom: 0;
}
@media (max-width: 650px) {
  .routing-evidence {
    padding: 16px;
  }
  .router-evidence-label {
    align-items: flex-start;
    flex-direction: column;
    gap: 5px;
  }
}
</style>
