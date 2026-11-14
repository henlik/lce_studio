<script setup lang="ts">
import type { WorkspaceDraft } from "../composables/useWorkspaceDraft";
defineProps<{ draft: WorkspaceDraft }>();
</script>
<template>
  <div class="draft-status" :class="draft.status.value" role="status">
    <span
      >{{
        draft.status.value === "saved"
          ? "✓"
          : draft.status.value === "saving"
            ? "↻"
            : "!"
      }}
      {{ draft.message.value }}</span
    >
    <small v-if="draft.updated.value">{{
      new Date(draft.updated.value).toLocaleTimeString()
    }}</small>
    <label v-if="draft.backups.value.length" class="recovery-choice"
      >Recover earlier draft<select
        :disabled="
          draft.status.value === 'saving' || draft.status.value === 'loading'
        "
        aria-label="Recover earlier draft"
        @change="
          draft.restoreBackup(($event.target as HTMLSelectElement).value);
          ($event.target as HTMLSelectElement).value = '';
        "
      >
        <option value="">Choose a saved checkpoint</option>
        <option v-for="b in draft.backups.value" :key="b.key" :value="b.key">
          {{ b.label }}
        </option>
      </select></label
    >
    <template
      v-if="draft.status.value === 'conflict' || draft.status.value === 'error'"
    >
      <button class="button secondary" @click="draft.exportRecovery">
        Export recovery copy
      </button>
      <button class="button secondary" @click="draft.loadSaved">
        Load saved draft
      </button>
      <button class="button secondary" @click="draft.keepMine">
        Save my draft (back up previous)
      </button>
    </template>
  </div>
</template>
<style scoped>
.draft-status {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 9px 14px;
  margin: 10px 0;
  border: 1px solid #cfe5db;
  background: #f1faf5;
  border-radius: 8px;
  color: #27634d;
  font-size: 12px;
}
.draft-status small {
  margin-left: auto;
}
.conflict,
.error {
  color: #912b23;
  border-color: #e9ada6;
  background: #fff3f1;
}
.saving,
.loading {
  color: #395e87;
  background: #eff6ff;
}
.recovery-choice {
  min-width: 0;
  max-width: 100%;
  flex-wrap: wrap;
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 11px;
}
.recovery-choice select {
  min-width: 0;
  max-width: min(260px, 100%);
  font-size: 11px;
}
</style>
