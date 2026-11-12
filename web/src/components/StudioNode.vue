<script setup lang="ts">
import { Handle, Position } from "@vue-flow/core";
import {
  Monitor,
  Server,
  Database,
  GitBranch,
  ShieldCheck,
} from "lucide-vue-next";
import type { ComponentNode } from "../types";
defineProps<{
  data: { node: ComponentNode; active: boolean };
  selected?: boolean;
}>();
</script>
<template>
  <div
    class="studio-node"
    :class="[data.node.type, { selected, active: data.active }]"
  >
    <Handle
      v-if="data.node.type !== 'client'"
      type="target"
      :position="Position.Left"
    />
    <div class="node-top">
      <span class="node-icon"
        ><Monitor v-if="data.node.type === 'client'" :size="19" /><Server
          v-else-if="data.node.type === 'service'"
          :size="19" /><GitBranch
          v-else-if="data.node.type === 'router'"
          :size="19" /><Database v-else :size="19" /></span
      ><span class="node-type">{{ data.node.type }}</span
      ><span class="node-dot" />
    </div>
    <strong>{{ data.node.name }}</strong>
    <div
      v-if="data.node.component"
      class="node-profile"
      :title="`${data.node.component.packageId} @ ${data.node.component.version}`"
    >
      {{ data.node.component.packageId }} · v{{ data.node.component.version }}
    </div>
    <div class="node-meta" v-if="data.node.type === 'client'">
      Request source <span>OUT</span>
    </div>
    <div class="node-meta" v-else>
      <span
        >{{ data.node.config.processingMs }} ms ·
        {{ data.node.config.capacity }} slots</span
      ><ShieldCheck
        v-if="data.node.type === 'database' && data.node.config.deduplicate"
        :size="14"
      />
    </div>
    <div v-if="data.node.type === 'router'" class="node-route-hint">
      ONE WEIGHTED OUTPUT
    </div>
    <Handle
      v-if="data.node.type !== 'database'"
      type="source"
      :position="Position.Right"
    />
  </div>
</template>
