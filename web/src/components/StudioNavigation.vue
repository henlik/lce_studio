<script setup lang="ts">
import {computed, ref, watch} from 'vue';
import {Compass, PencilRuler, Radio, ShieldCheck, ChevronDown} from 'lucide-vue-next';
import {blueprint, capabilityLocation, workspaceLocation, statusLabels, type StudioView, type PlaneID} from '../studioBlueprint';

const props = defineProps<{view: StudioView; selectedCapability: string; collapsed: boolean}>();
const emit = defineEmits<{select: [id: string]; blueprint: []; expand: []; projects: []; history: []}>();
const active = computed(() => props.view === 'blueprint' ? props.selectedCapability : workspaceLocation(props.view));
const openPlanes = ref<string[]>(['engineering']);
const openGroups = ref<string[]>(['design']);
const icons = {engineering: PencilRuler, runtime: Radio, foundations: ShieldCheck};
watch(active, id => {
  const location = id && capabilityLocation(id);
  if (!location) return;
  if (!openPlanes.value.includes(location.plane.id)) openPlanes.value.push(location.plane.id);
  if (!openGroups.value.includes(location.group.id)) openGroups.value.push(location.group.id);
}, {immediate: true});
function togglePlane(id: PlaneID) {
  if (props.collapsed) {
    emit('expand');
    if (!openPlanes.value.includes(id)) openPlanes.value.push(id);
    return;
  }
  openPlanes.value = openPlanes.value.includes(id) ? openPlanes.value.filter(v => v !== id) : [...openPlanes.value, id];
}
function toggleGroup(id: string) {
  openGroups.value = openGroups.value.includes(id) ? openGroups.value.filter(v => v !== id) : [...openGroups.value, id];
}
</script>

<template>
  <nav class="studio-navigation" aria-label="Studio planes and workspaces">
    <button class="blueprint-link" :class="{active:view === 'blueprint' && !selectedCapability}" :aria-current="view === 'blueprint' && !selectedCapability ? 'page' : undefined" title="Product blueprint" aria-label="Product blueprint" @click="emit('blueprint')">
      <Compass :size="18"/><span v-if="!collapsed">Product blueprint</span>
    </button>
    <section v-for="plane in blueprint" :key="plane.id" class="plane">
      <button class="plane-toggle" :aria-expanded="!collapsed && openPlanes.includes(plane.id)" :aria-controls="'nav-'+plane.id" :title="plane.label" :aria-label="plane.label" @click="togglePlane(plane.id)">
        <component :is="icons[plane.id]" :size="18"/><span v-if="!collapsed">{{plane.label}}</span><ChevronDown v-if="!collapsed" :size="13" class="chevron" :class="{open:openPlanes.includes(plane.id)}"/>
      </button>
      <div v-show="!collapsed && openPlanes.includes(plane.id)" :id="'nav-'+plane.id">
        <div v-for="group in plane.groups" :key="group.id" class="nav-group">
          <div v-if="group.id==='design'" class="design-entry"><button class="group-toggle" @click="emit('select','system-model')">Design</button><button class="group-toggle group-disclosure" aria-label="Expand or collapse Design navigation" :aria-expanded="openGroups.includes(group.id)" :aria-controls="'nav-group-'+group.id" @click="toggleGroup(group.id)"><ChevronDown :size="12" class="chevron" :class="{open:openGroups.includes(group.id)}"/></button></div>
          <button v-else class="group-toggle" :aria-expanded="openGroups.includes(group.id)" :aria-controls="'nav-group-'+group.id" @click="toggleGroup(group.id)"><span>{{group.label}}</span><ChevronDown :size="12" class="chevron" :class="{open:openGroups.includes(group.id)}"/></button>
          <ul v-show="openGroups.includes(group.id)" :id="'nav-group-'+group.id">
            <li v-for="item in group.capabilities" :key="item.id">
              <button class="workspace-link" :class="{active:active===item.id}" :aria-current="active===item.id ? 'page' : undefined" :title="item.label+' — '+statusLabels[item.status]" @click="emit('select',item.id)">
                <span>{{item.label}}</span><small :class="item.status" :aria-label="statusLabels[item.status]">{{item.status === 'available' ? '●' : statusLabels[item.status]}}</small>
              </button>
            </li>
            <template v-if="group.id === 'simulation'">
              <li><button class="workspace-link" @click="emit('projects')">Open simulation projects</button></li>
              <li><button class="workspace-link" @click="emit('history')">Simulation history</button></li>
            </template>
          </ul>
        </div>
      </div>
    </section>
    <p v-if="!collapsed" class="nav-hint">Planned items open their blueprint. Runtime controls are not connected.</p>
  </nav>
</template>

<style scoped>
.design-entry{display:flex;min-width:0}.design-entry .group-disclosure{width:32px;flex:none;justify-content:center}.design-entry .chevron{margin-left:0}

.studio-navigation{margin-top:20px;min-width:0}.studio-navigation button{display:flex;align-items:center;gap:8px;width:100%;text-align:left;color:#b5c6d7;background:transparent;border:0;border-radius:6px;cursor:pointer;min-width:0;font:inherit;font-size:12px;padding:10px 7px}.studio-navigation button:hover{background:#23394b;color:#fff}.studio-navigation button:focus-visible{outline:2px solid #68dbc2;outline-offset:-2px}.studio-navigation button.active{background:#23394b;color:#8ce2d3;box-shadow:inset 3px 0 #4acbb3}.studio-navigation svg{flex-shrink:0}.plane{margin-top:12px}.studio-navigation .plane-toggle{font-weight:650;font-size:11px}.chevron{margin-left:auto;transform:rotate(-90deg)}.chevron.open{transform:none}.studio-navigation .group-toggle{font-size:11px;color:#8ea8bd;padding:9px 8px}.nav-group{margin-left:7px;border-left:1px solid #304458;padding-left:5px}.nav-group ul{list-style:none;padding:0;margin:0}.studio-navigation .workspace-link{font-size:11px;padding:9px 7px;align-items:flex-start;line-height:1.4}.workspace-link>span{flex:1;min-width:0;overflow-wrap:anywhere}.workspace-link small{font-size:8px;flex-shrink:0;margin-top:2px}.available{color:#65d3b7}.partial{color:#f0ca80}.planned{color:#9aaec4}.nav-hint{font-size:10px;line-height:1.6;color:#91a7b9;margin:15px 8px}.blueprint-link{margin-bottom:14px}
</style>
