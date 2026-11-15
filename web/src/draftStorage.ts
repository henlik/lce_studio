// IndexedDB transactions fence autosave across tabs during backup/reset/restore.
export interface DraftRecord { key: string; revision: number; updated: string; data: string; label?: string }
export interface BrowserBackup { drafts: DraftRecord[]; preferences: Record<string, string> }
export interface StudioBackup { format: string; id: string; createdAt: string; epoch: string; database: string; databaseHash: string; browser: BrowserBackup; digest: string }
export interface MaintenanceLock { token: string; epoch: string; backupId?: string; action?: 'reset' | 'restore'; target?: StudioBackup; applied?: boolean }
interface Control { key: 'control'; generation: string; lock?: MaintenanceLock }
let database: Promise<IDBDatabase> | undefined;
export let draftsPaused = false;
export let maintenanceReloading = false;
const writes = new Set<Promise<unknown>>();
export function pauseDrafts() { draftsPaused = true; }
export function resumeDrafts() { draftsPaused = false; }
export async function drainDraftWrites() { await Promise.allSettled([...writes]); }
function db() {
  return database ||= new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open('lce-studio-drafts', 2);
    request.onupgradeneeded = () => {
      if (!request.result.objectStoreNames.contains('drafts')) request.result.createObjectStore('drafts', {keyPath: 'key'});
      if (!request.result.objectStoreNames.contains('meta')) request.result.createObjectStore('meta', {keyPath: 'key'});
    };
    request.onblocked = () => { database = undefined; reject(new Error('Close older studio tabs, then retry. Draft storage is being upgraded.')); };
    request.onsuccess = () => { request.result.onversionchange = () => { request.result.close(); database = undefined; }; resolve(request.result); };
    request.onerror = () => { database = undefined; reject(request.error); };
  });
}
function control(value?: Control): Control { return value || {key: 'control', generation: 'initial'}; }
export async function readDraftState(key: string): Promise<{record?: DraftRecord; generation: string}> {
  const connection = await db();
  return new Promise((resolve,reject) => {
    const tx = connection.transaction(['drafts','meta'], 'readonly');
    const row = tx.objectStore('drafts').get(key), meta = tx.objectStore('meta').get('control');
    tx.oncomplete = () => resolve({record: row.result, generation: control(meta.result).generation});
    tx.onabort = tx.onerror = () => reject(tx.error);
  });
}
export async function readDraft(key: string) { return (await readDraftState(key)).record; }
export function commitDraft(key: string, revision: number, data: string, archive = false, generation = 'initial'): Promise<DraftRecord> {
  const operation = (async () => {
    if (draftsPaused) throw new Error('Studio maintenance is in progress.');
    const connection = await db();
    return new Promise<DraftRecord>((resolve,reject) => {
      const tx = connection.transaction(['drafts','meta'], 'readwrite'), store = tx.objectStore('drafts');
      let error = '';
      const result = {key, revision: revision+1, data, updated: new Date().toISOString()};
      const meta = tx.objectStore('meta').get('control');
      meta.onsuccess = () => {
        const state = control(meta.result);
        if (state.lock || state.generation !== generation) { error = 'Studio was reset, restored, or paused in another tab. Export your edits and reload.'; tx.abort(); return; }
        const req = store.get(key);
        req.onsuccess = () => {
          const previous = req.result as DraftRecord | undefined;
          if ((previous?.revision || 0) !== revision) { error = 'draft-conflict'; tx.abort(); return; }
          if (archive && previous) store.put({...previous, key: `${key}:backup:${crypto.randomUUID()}`});
          store.put(result);
        };
      };
      tx.oncomplete = () => resolve(result);
      tx.onabort = tx.onerror = () => reject(new Error(error || tx.error?.message || 'Draft storage is unavailable.'));
    });
  })();
  writes.add(operation); void operation.then(() => writes.delete(operation), () => writes.delete(operation));
  return operation;
}
export async function draftBackups(key: string): Promise<DraftRecord[]> {
  const connection = await db();
  return new Promise((resolve,reject) => {
    const tx = connection.transaction('drafts', 'readonly'), req = tx.objectStore('drafts').getAll();
    tx.oncomplete = () => resolve((req.result as DraftRecord[]).filter(r => r.key.startsWith(`${key}:backup:`)).sort((a,b) => b.updated.localeCompare(a.updated)));
    tx.onabort = tx.onerror = () => reject(tx.error);
  });
}
export async function pendingMaintenance(): Promise<MaintenanceLock | undefined> {
  const connection = await db();
  return new Promise((resolve,reject) => {
    const tx = connection.transaction('meta','readonly'), req = tx.objectStore('meta').get('control');
    tx.oncomplete = () => resolve(control(req.result).lock);
    tx.onabort = tx.onerror = () => reject(tx.error);
  });
}
export async function lockBrowser(epoch: string, live: {key: string; data: string}[]): Promise<{lock: MaintenanceLock; browser: BrowserBackup}> {
  const preferences: Record<string,string> = {};
  for (let i = 0; i < localStorage.length; i++) { const k = localStorage.key(i)!; if (k.startsWith('studio.ribbon.')) preferences[k] = localStorage.getItem(k)!; }
  const connection = await db();
  return new Promise((resolve,reject) => {
    const tx = connection.transaction(['drafts','meta'],'readwrite'), meta = tx.objectStore('meta');
    const req = meta.get('control'), rows = tx.objectStore('drafts').getAll();
    const lock: MaintenanceLock = {token: crypto.randomUUID(), epoch}; let error = '';
    req.onsuccess = () => {
      const state = control(req.result);
      if (state.lock) { error = 'A backup/reset is already pending. Reopen Backup & reset to recover it.'; tx.abort(); return; }
      meta.put({...state, lock});
    };
    tx.oncomplete = () => {
      const drafts: DraftRecord[] = rows.result;
      // Unsaved buffers in THIS tab are part of the backup, including conflict
      // recovery. Preserve the stored version as a checkpoint if it differs.
      for (const current of live) {
        const index = drafts.findIndex(d => d.key === current.key), previous = drafts[index];
        if (previous?.data === current.data) continue;
        if (previous) drafts.push({...previous, key: `${previous.key}:backup:${crypto.randomUUID()}`});
        const row = {...current, revision: (previous?.revision || 0)+1, updated: new Date().toISOString()};
        if (index < 0) drafts.push(row); else drafts[index] = row;
      }
      resolve({lock, browser: {drafts, preferences}});
    };
    tx.onabort = tx.onerror = () => reject(new Error(error || tx.error?.message || 'Could not lock draft storage.'));
  });
}
export async function updateMaintenance(lock: MaintenanceLock | undefined, token: string) {
  const connection = await db();
  return new Promise<void>((resolve,reject) => {
    const tx = connection.transaction('meta','readwrite'), meta = tx.objectStore('meta'), req = meta.get('control');
    req.onsuccess = () => { const state = control(req.result); if (state.lock?.token !== token) { tx.abort(); return; } meta.put(JSON.parse(JSON.stringify({...state, lock}))); };
    tx.oncomplete = () => resolve(); tx.onabort = tx.onerror = () => reject(new Error('Maintenance ownership changed. Reopen Backup & reset.'));
  });
}
export async function finishBrowser(lock: MaintenanceLock, browser?: BrowserBackup) {
  const connection = await db();
  await new Promise<void>((resolve,reject) => {
    const tx = connection.transaction(['drafts','meta'],'readwrite'), meta = tx.objectStore('meta'), req = meta.get('control');
    req.onsuccess = () => {
      const state = control(req.result);
      if (state.lock?.token !== lock.token) { tx.abort(); return; }
      if (state.lock.applied) return;
      const store = tx.objectStore('drafts'); store.clear();
      for (const row of browser?.drafts || []) store.put(JSON.parse(JSON.stringify(row)));
      meta.put(JSON.parse(JSON.stringify({...state, generation: crypto.randomUUID(), lock: {...lock, applied: true}})));
    };
    tx.oncomplete = () => resolve(); tx.onabort = tx.onerror = () => reject(new Error('Server operation completed; browser recovery is still pending. Retry recovery.'));
  });
  // Keep the pending marker until preferences are applied, so a quota failure or
  // crash can retry without resubmitting old drafts to the database.
  const keys = Object.keys(localStorage).filter(k => k.startsWith('studio.ribbon.'));
  for (const k of keys) localStorage.removeItem(k);
  for (const [k,v] of Object.entries(browser?.preferences || {})) localStorage.setItem(k,v);
  await updateMaintenance(undefined, lock.token);
  maintenanceReloading = true;
}
