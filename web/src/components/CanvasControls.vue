<script setup lang="ts">
import { Maximize2, Minimize2, Minus, Plus, Scan } from "lucide-vue-next";
withDefaults(defineProps<{ label: string; zoom: number; expanded: boolean; minZoom?: number; maxZoom?: number }>(), { minZoom: 0.5, maxZoom: 4 });
const emit = defineEmits<{ zoomOut: []; zoomIn: []; fit: []; expand: [] }>();
</script>
<template>
  <div class="canvas-controls" role="group" :aria-label="`${label} view controls`">
    <button type="button" class="icon-button" :aria-label="`Zoom ${label} out`" :title="`Zoom ${label} out`" :disabled="zoom <= minZoom" @click="emit('zoomOut')"><Minus :size="15" /></button>
    <output class="canvas-zoom" :aria-label="`${label} zoom`">{{ Math.round(zoom * 100) }}%</output>
    <button type="button" class="icon-button" :aria-label="`Zoom ${label} in`" :title="`Zoom ${label} in`" :disabled="zoom >= maxZoom" @click="emit('zoomIn')"><Plus :size="15" /></button>
    <button type="button" class="canvas-view-button" :aria-label="`Fit ${label}`" :title="`Fit ${label} to the available space`" @click="emit('fit')"><Scan :size="15" /><span>Fit</span></button>
    <button type="button" class="canvas-view-button" :aria-label="`${expanded ? 'Restore' : 'Expand'} ${label}`" :aria-pressed="expanded" :title="expanded ? 'Restore canvas (Escape)' : 'Expand canvas'" @click="emit('expand')"><component :is="expanded ? Minimize2 : Maximize2" :size="15" /><span>{{ expanded ? "Restore" : "Expand" }}</span></button>
  </div>
</template>
<style scoped>
.canvas-controls { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }
.canvas-zoom { min-width: 38px; text-align: center; color: #688093; font-size: 11px; font-variant-numeric: tabular-nums; }
.canvas-view-button { display: inline-flex; gap: 6px; align-items: center; border: 1px solid #dce6ec; border-radius: 6px; background: #fff; padding: 7px 9px; color: #426477; font-size: 11px; white-space: nowrap; }
.canvas-view-button:hover { background: #edf7f4; border-color: #9ecabe; }
.canvas-view-button:focus-visible { outline: 2px solid #168d79; outline-offset: 2px; }
@media (max-width: 600px) { .canvas-view-button span { display: none; } }
</style>
