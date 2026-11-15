import { ref, watch, onBeforeUnmount, type Ref } from "vue";

import { commitDraft, readDraft, readDraftState, draftBackups, pauseDrafts, resumeDrafts, drainDraftWrites, draftsPaused, maintenanceReloading, type DraftRecord } from "../draftStorage";
export { commitDraft, readDraft, draftBackups } from "../draftStorage";
export type { DraftRecord } from "../draftStorage";
const workspaces = new Set<{key: string; ready: Promise<void>; snapshot: () => string; flush: () => Promise<void>}>();
export async function captureWorkspaceDrafts() {
  pauseDrafts();
  const waited = new Set<unknown>();
  do {
    const current = [...workspaces];
    await Promise.all(current.map(w => w.ready));
    current.forEach(w => waited.add(w));
    await drainDraftWrites();
  } while ([...workspaces].some(w => !waited.has(w)));
  return [...workspaces].map(w => ({key: w.key, data: w.snapshot()}));
}
export function resumeWorkspaceDrafts() {
  resumeDrafts();
  for (const w of workspaces) void w.flush();
}
export function useWorkspaceDraft(
  key: string,
  fields: Record<string, Ref<any>>,
) {
  const status = ref<"loading" | "saved" | "saving" | "error" | "conflict">(
    "loading",
  );
  const message = ref("Restoring draft…");
  const updated = ref("");
  const backups = ref<DraftRecord[]>([]);
  const refreshBackups = async () => {
    try {
      backups.value = (await draftBackups(key)).map((r) => {
        let label = r.updated;
        try {
          const x = JSON.parse(r.data),
            root = x.draft || x.machine || x.workflow || x.model || x.project;
          label = `${root?.name || key} ${root?.version || ""} · ${new Date(r.updated).toLocaleString()}`;
        } catch {}
        return { ...r, label };
      });
    } catch {}
  };
  let generation = "initial";
  let revision = 0,
    persisted = "",
    disposed = false,
    archiveNext = false;
  const snapshot = () =>
    JSON.stringify(
      Object.fromEntries(Object.entries(fields).map(([k, v]) => [k, v.value])),
    );
  const rootField =
    fields.draft ||
    fields.machine ||
    fields.workflow ||
    fields.model ||
    fields.project;
  if (rootField)
    watch(
      rootField,
      () => {
        if (status.value !== "loading") archiveNext = true;
      },
      { flush: "sync" },
    );
  function restore(record: DraftRecord) {
    const data = JSON.parse(record.data);
    if (!data || typeof data !== "object" || Array.isArray(data))
      throw new Error(
        "Saved draft is unreadable. Export it before replacing it.",
      );
    for (const [name, field] of Object.entries(fields))
      if (Object.hasOwn(data, name)) field.value = data[name];
    revision = record.revision;
    persisted = snapshot();
    updated.value = record.updated;
  }
  const ready = (async () => {
    try {
      const state = await readDraftState(key);
      generation = state.generation;
      const record = state.record;
      if (record) restore(record);
      else persisted = snapshot();
      void refreshBackups();
      status.value = "saved";
      message.value = record
        ? "Draft restored · saved on this browser"
        : "Autosave ready · this browser";
    } catch (e) {
      status.value = "error";
      message.value = `Draft recovery failed: ${String(e)}. Your saved draft has not been replaced.`;
    }
  })();
  const workspace = {key, ready, snapshot, flush};
  workspaces.add(workspace);
  async function flush() {
    await ready;
    if (
      disposed || draftsPaused ||
      ["loading", "saving", "conflict", "error"].includes(status.value)
    )
      return;
    const data = snapshot();
    if (data === persisted) return;
    status.value = "saving";
    message.value = "Saving draft…";
    const archive = archiveNext;
    archiveNext = false;
    try {
      const record = await commitDraft(key, revision, data, archive, generation);
      if (archive) void refreshBackups();
      revision = record.revision;
      persisted = data;
      updated.value = record.updated;
      status.value = "saved";
      message.value = "Draft saved · this browser";
      if (snapshot() !== persisted) void flush();
    } catch (e) {
      archiveNext ||= archive;
      status.value =
        e instanceof Error && e.message === "draft-conflict"
          ? "conflict"
          : "error";
      message.value =
        status.value === "conflict"
          ? "Another tab saved this workspace. Your edits are still here; export them before loading the saved draft."
          : `Autosave paused: ${e instanceof Error ? e.message : "Storage is unavailable"} Export your work before reloading.`;
    }
  }
  watch(
    Object.values(fields),
    () => {
      void flush();
    },
    { deep: true, flush: "post" },
  );
  async function loadSaved() {
    try {
      const record = await readDraft(key);
      if (record) restore(record);
      status.value = "saved";
      message.value = "Saved draft loaded";
    } catch {
      status.value = "error";
      message.value =
        "Could not load saved draft. Your current edits were kept.";
    }
  }
  async function keepMine() {
    try {
      const record = await readDraft(key);
      revision = record?.revision || 0;
      archiveNext = true;
      persisted = "";
      status.value = "saved";
      await flush();
    } catch {
      status.value = "error";
      message.value = "Could not save your draft. Export a recovery copy.";
    }
  }
  async function restoreBackup(backupKey: string) {
    if (!backupKey) return;
    try {
      const record = await readDraft(backupKey);
      if (!record) return;
      const currentRevision = revision;
      restore(record);
      revision = currentRevision;
      archiveNext = true;
      persisted = "";
      status.value = "saved";
      await flush();
    } catch {
      status.value = "error";
      message.value = "Could not restore that recovery snapshot.";
    }
  }
  function exportRecovery() {
    const url = URL.createObjectURL(
      new Blob([snapshot()], { type: "application/json" }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = `lce-${key}-draft-recovery.json`;
    a.click();
    URL.revokeObjectURL(url);
  }
  function unload(e: BeforeUnloadEvent) {
    if (!maintenanceReloading && (snapshot() !== persisted || status.value === "loading")) {
      e.preventDefault();
      e.returnValue = "";
    }
  }
  window.addEventListener("beforeunload", unload);
  const visibility = () => {
    if (document.visibilityState === "hidden") void flush();
  };
  document.addEventListener("visibilitychange", visibility);
  onBeforeUnmount(() => {
    disposed = true;
    workspaces.delete(workspace);
    window.removeEventListener("beforeunload", unload);
    document.removeEventListener("visibilitychange", visibility);
  });
  return {
    status,
    message,
    updated,
    backups,
    restoreBackup,
    ready,
    flush,
    loadSaved,
    keepMine,
    exportRecovery,
    unsaved: () => snapshot() !== persisted,
  };
}
export type WorkspaceDraft = ReturnType<typeof useWorkspaceDraft>;
