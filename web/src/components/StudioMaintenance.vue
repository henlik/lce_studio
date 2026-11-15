<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { ArchiveRestore, Download, RotateCcw, ShieldCheck, X } from 'lucide-vue-next';
import { APIError, api, pauseStudioRequests, resumeStudioRequests } from '../api';
import { captureWorkspaceDrafts, resumeWorkspaceDrafts } from '../composables/useWorkspaceDraft';
import { lockBrowser, pendingMaintenance, updateMaintenance, finishBrowser, type MaintenanceLock, type StudioBackup } from '../draftStorage';
const props = defineProps<{open: boolean}>();
const emit = defineEmits<{ 'update:open': [value: boolean] }>();
interface Summary { id: string; createdAt: string; rows: number; drafts: number }
interface Status { epoch: string; action: string; backupId: string; restoreId: string }
const dialog = ref<HTMLDialogElement>(), busy = ref(''), error = ref(''), notice = ref('');
const history = ref<Summary[]>([]), retained = ref(''), confirmation = ref('');
const safety = ref<StudioBackup>(), target = ref<StudioBackup>(), lock = ref<MaintenanceLock>();
const mode = ref<'reset'|'restore'>('reset');
const committed = ref(false);
const word = computed(() => mode.value === 'reset' ? 'RESET' : 'RESTORE');
const rowCount = (b?: StudioBackup) => b ? Object.values(JSON.parse(b.database) as Record<string, unknown[]>).reduce((n,rows) => n+rows.length,0) : 0;
function download(b: StudioBackup) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(b,null,2)], {type: 'application/json'}));
  const a = document.createElement('a'); a.href = url; a.download = `lce-studio-${b.createdAt.replaceAll(':','-')}-${b.id.slice(0,8)}.json`; a.click();
  setTimeout(() => URL.revokeObjectURL(url), 10000);
}
async function run(label: string, work: () => Promise<void>) {
  if (busy.value) return;
  busy.value = label; error.value = ''; notice.value = '';
  try { await work(); } catch (e) { error.value = e instanceof Error ? e.message : String(e); }
  finally { busy.value = ''; }
}
async function refresh() { history.value = await api<Summary[]>('/studio/backups'); }
async function initialize() {
  await nextTick(); dialog.value?.showModal();
  await run('Loading backups…', async () => {
    lock.value = await pendingMaintenance();
    if (lock.value) {
      await pauseStudioRequests(); await captureWorkspaceDrafts();
      if (lock.value.backupId) safety.value = await api<StudioBackup>(`/studio/backup?id=${encodeURIComponent(lock.value.backupId)}`);
      if (lock.value.action) { mode.value = lock.value.action; target.value = lock.value.target; notice.value = 'An interrupted operation needs recovery. Resume it to finish both server and browser storage.'; }
      else notice.value = 'A previous backup session is pending. Continue with its backup or cancel to resume editing.';
    }
    await refresh();
  });
}
watch(() => props.open, value => { if (value) void initialize(); });
onMounted(async () => {
  try { if (await pendingMaintenance()) emit('update:open',true); } catch { /* Normal backup action will explain unavailable storage. */ }
  if (props.open) await initialize();
});
async function close() {
  if (busy.value || lock.value?.action || committed.value) return;
  await run('Resuming studio…', async () => {
    if (lock.value) await updateMaintenance(undefined,lock.value.token);
    lock.value = undefined; safety.value = undefined; target.value = undefined; confirmation.value = '';
    resumeStudioRequests(); resumeWorkspaceDrafts();
    dialog.value?.close(); emit('update:open',false);
  });
}
async function prepare() {
  await run('Creating backup…', async () => {
    await pauseStudioRequests(); await nextTick();
    const live = await captureWorkspaceDrafts();
    if (lock.value) { await updateMaintenance(undefined,lock.value.token); lock.value = undefined; }
    safety.value = undefined; confirmation.value = '';
    const status = await api<Status>('/studio/status');
    const snapshot = await lockBrowser(status.epoch,live); lock.value = snapshot.lock;
    safety.value = await api<StudioBackup>('/studio/backup','POST',{epoch: status.epoch,browser: snapshot.browser});
    lock.value = {...lock.value,backupId: safety.value.id};
    await updateMaintenance({...lock.value},lock.value.token);
    download(safety.value); await refresh();
    notice.value = 'Backup retained on the server. Download requested; keep the JSON file outside this studio for recovery.';
  });
}
async function choose(b: StudioBackup) {
  await api('/studio/validate-backup','POST',{backup: b});
  target.value = b; mode.value = 'restore'; confirmation.value = '';
  notice.value = 'Backup validated. Create a backup of the current studio before replacing it.';
}
async function upload(event: Event) {
  const input = event.target as HTMLInputElement, file = input.files?.[0]; input.value = '';
  if (!file) return;
  await run('Validating backup…',async () => {
    if (file.size > 128*1024*1024) throw new Error('Backup exceeds the 128 MiB limit.');
    await choose(JSON.parse(await file.text()));
  });
}
async function selectRetained() {
  await run('Validating backup…',async () => { await choose(await api<StudioBackup>(`/studio/backup?id=${encodeURIComponent(retained.value)}`)); });
}
async function apply(recover = false) {
  if (!lock.value?.backupId || (!recover && confirmation.value !== word.value)) return;
  await run(recover ? 'Recovering operation…' : `${mode.value === 'reset' ? 'Resetting' : 'Restoring'} studio…`,async () => {
    if (!lock.value) return;
    const operation: MaintenanceLock = {...lock.value,action: lock.value.action || mode.value, target: lock.value.target || (mode.value === 'restore' ? target.value : undefined)};
    // Persist intent before sending. A retry uses the same safety backup ID and
    // is idempotent on the server, even if the first response was lost.
    await updateMaintenance(operation,operation.token); lock.value = operation;
    let state: Status;
    try {
      state = await api<Status>(`/studio/${operation.action}`,'POST',{epoch: operation.epoch,backupId: operation.backupId,confirmation: operation.action === 'reset' ? 'RESET' : 'RESTORE',...(operation.target ? {backup: operation.target} : {})});
    } catch (e) {
      // A server rejection (stale snapshot, invalid restore) can be cancelled.
      // A transport failure is ambiguous: keep the intent for safe retry.
      if (e instanceof APIError && e.status >= 400 && e.status < 500) {
        lock.value = {...operation,action: undefined,target: undefined};
        await updateMaintenance({...lock.value},operation.token);
        confirmation.value = '';
        throw e;
      }
      const status = await api<Status>('/studio/status').catch(() => undefined);
      if (status && status.backupId === operation.backupId && status.action === operation.action && (!operation.target || status.restoreId === operation.target.id)) state = status;
      else throw e;
    }
    if (state.backupId !== operation.backupId || state.action !== operation.action || (operation.target && state.restoreId !== operation.target.id)) throw new Error('Server operation does not match this recovery. Keep the backup and reload.');
    committed.value = true;
    await finishBrowser(operation,operation.target?.browser);
    window.location.reload();
  });
}
</script>

<template>
  <dialog ref="dialog" class="studio-maintenance" aria-labelledby="maintenance-title" @cancel.prevent="close">
    <header class="maintenance-header"><div><span class="maintenance-eyebrow">STUDIO STORAGE</span><h2 id="maintenance-title">Backup &amp; reset</h2></div><button class="icon-button" aria-label="Close backup and reset" :disabled="!!busy || !!lock?.action || committed" @click="close"><X :size="20"/></button></header>
    <div class="maintenance-body" :aria-busy="!!busy">
      <p class="maintenance-scope"><strong>This affects the entire studio on this server.</strong> Projects, designs, installed packages, component definitions, tests and execution history are included, along with this browser’s saved drafts, recovery checkpoints, open workspaces in this tab and ribbon settings.</p>
      <p class="maintenance-note">Built-in examples, application code, credentials and retained backups stay. Save or export unsaved edits in other tabs or browsers first; their open buffers cannot be captured here.</p>
      <div class="maintenance-tabs" role="group" aria-label="Storage action"><button class="button" :class="{primary: mode === 'reset'}" :disabled="!!busy || !!lock?.action" @click="mode = 'reset'; confirmation = ''"><RotateCcw :size="16"/> Backup / reset</button><button class="button" :class="{primary: mode === 'restore'}" :disabled="!!busy || !!lock?.action" @click="mode = 'restore'; confirmation = ''"><ArchiveRestore :size="16"/> Restore</button></div>
      <section v-if="mode === 'restore'" class="maintenance-card"><h3>Choose a backup to restore</h3><label class="maintenance-label">Upload backup JSON<input type="file" accept="application/json,.json" :disabled="!!busy || !!lock?.action" @change="upload"/></label><label class="maintenance-label">Retained backups (latest 100)<select v-model="retained" :disabled="!!busy || !!lock?.action"><option value="">Select a backup</option><option v-for="b in history" :key="b.id" :value="b.id">{{new Date(b.createdAt).toLocaleString()}} · {{b.rows}} rows · {{b.drafts}} drafts · {{b.id.slice(0,8)}}</option></select></label><button class="button" :disabled="!retained || !!busy || !!lock?.action" @click="selectRetained">Use selected backup</button><div v-if="target" class="backup-ready"><ShieldCheck :size="18"/><span>Validated · {{new Date(target.createdAt).toLocaleString()}} · {{rowCount(target)}} rows · {{target.browser.drafts.length}} drafts</span><button class="button quiet" :disabled="!!busy" @click="download(target)">Download selected</button></div></section>
      <section class="maintenance-card"><h3>1. Back up the current studio</h3><p>The server must retain a complete backup before reset or restore can proceed. Creating a backup does not clear anything.</p><button class="button primary" :disabled="!!busy || !!lock?.action" @click="prepare"><Download :size="16"/>{{safety ? 'Create fresh backup' : 'Create backup'}}</button><div v-if="safety" class="backup-ready"><ShieldCheck :size="18"/><span>Retained · {{rowCount(safety)}} rows · {{safety.browser.drafts.length}} drafts<br/><small>{{safety.id}}</small></span><button class="button quiet" :disabled="!!busy" @click="download(safety)">Download again</button></div></section>
      <section v-if="!lock?.action" class="maintenance-card maintenance-danger"><h3>2. {{mode === 'reset' ? 'Reset to an empty studio' : 'Replace the studio with the selected backup'}}</h3><p>{{mode === 'reset' ? 'All application records and this browser’s drafts will be cleared. The studio will reload empty.' : 'The selected backup replaces the current application records and this browser’s drafts. The studio will reload with the restored data.'}}</p><label class="maintenance-label">Type {{word}} to confirm<input v-model="confirmation" :aria-label="`Type ${word} to confirm`" :disabled="!safety || !!busy" autocomplete="off" spellcheck="false"/></label><button class="button danger" :disabled="!safety || !!busy || confirmation !== word || (mode === 'restore' && !target)" @click="apply()">{{mode === 'reset' ? 'Reset studio' : 'Restore studio'}}</button></section>
      <section v-else class="maintenance-card maintenance-danger"><h3>Finish interrupted {{lock.action}}</h3><p>Editing is paused until server and browser storage agree. Your safety backup remains available.</p><button class="button primary" :disabled="!!busy" @click="apply(true)">Resume recovery</button></section>
      <p v-if="busy" class="maintenance-status" role="status"><span class="maintenance-spinner"/>{{busy}}</p><p v-if="notice" class="maintenance-status" role="status">{{notice}}</p><p v-if="error" class="maintenance-error" role="alert">{{error}}</p>
    </div><footer class="maintenance-footer"><span>Backups contain your studio data. Store downloaded files securely.</span><button class="button" :disabled="!!busy || !!lock?.action || committed" @click="close">Done</button></footer>
  </dialog>
</template>
<style scoped>
.studio-maintenance{color:var(--ink,#18232f);background:#f5f7fa;border:1px solid #c7d1dc;border-radius:14px;padding:0;width:min(720px,calc(100vw - 24px));max-width:calc(100vw - 24px);max-height:calc(100dvh - 24px);box-shadow:0 24px 90px #13283b55;overflow:auto;margin:auto}
.studio-maintenance[open]{display:flex;flex-direction:column;overflow:hidden}
.studio-maintenance::backdrop{background:#0b1b2db3;backdrop-filter:blur(3px)}
.maintenance-header,.maintenance-footer{flex:none;display:flex;justify-content:space-between;align-items:center;gap:16px;padding:18px 22px;background:#fff}
.maintenance-header{border-bottom:1px solid #dce2e9}.maintenance-header h2{margin:4px 0 0;font-size:23px;letter-spacing:-.5px}.maintenance-eyebrow{font-size:10px;letter-spacing:1.8px;font-weight:750;color:#527286}.maintenance-body{padding:20px 22px;display:grid;gap:16px;min-height:0;overflow:auto}.maintenance-body p{margin:0;line-height:1.6;font-size:13px}.maintenance-note,.maintenance-card p{color:#526575}.maintenance-tabs{display:flex;flex-wrap:wrap;gap:8px}.maintenance-tabs .button{flex:1;justify-content:center}.maintenance-card{background:#fff;border:1px solid #d8e1e8;border-radius:10px;padding:18px;display:grid;gap:13px;min-width:0}.maintenance-card h3{font-size:15px;margin:0}.maintenance-card>.button{justify-self:start}.maintenance-label{display:grid;gap:7px;font-size:12px;font-weight:650;min-width:0}.maintenance-label input,.maintenance-label select{width:100%;min-width:0;box-sizing:border-box;padding:9px;border:1px solid #b8c7d3;border-radius:6px;background:#fff;color:inherit}.backup-ready{display:flex;align-items:center;flex-wrap:wrap;gap:10px;padding:12px;background:#eff8f5;border-radius:6px;color:#245b49;font-size:12px;overflow-wrap:anywhere}.backup-ready span{flex:1;min-width:130px}.backup-ready small{font-size:10px}.maintenance-danger{border-color:#e3c9c4}.maintenance-danger .danger{background:#a6382d;color:#fff;border-color:#a6382d}.maintenance-status{display:flex;gap:8px;align-items:center;color:#255a78}.maintenance-error{padding:12px;background:#fff0ec;border-radius:6px;color:#a42d24;overflow-wrap:anywhere}.maintenance-footer{border-top:1px solid #dce2e9;font-size:11px;color:#526575}.maintenance-footer span{max-width:440px}.maintenance-spinner{width:14px;height:14px;flex:none;border:2px solid #b5c8d3;border-top-color:#255a78;border-radius:50%;animation:maintenance-spin .8s linear infinite}.button:disabled{opacity:.45;cursor:not-allowed}@keyframes maintenance-spin{to{transform:rotate(360deg)}}@media(max-width:520px){.maintenance-header,.maintenance-body,.maintenance-footer{padding:16px}.maintenance-card{padding:14px}.maintenance-header h2{font-size:20px}.maintenance-tabs .button{font-size:12px}.maintenance-footer{gap:12px}}
</style>
