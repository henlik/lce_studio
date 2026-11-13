import { reactive, watch, type Ref } from "vue";
export const notifications = reactive<
  { id: number; text: string; kind: "success" | "error" | "info" }[]
>([]);
export const pendingActions = reactive(new Map<string, string>());
let sequence = 0;
export function toast(
  text: string,
  kind: "success" | "error" | "info" = "success",
) {
  if (!text) return;
  const id = ++sequence;
  notifications.push({ id, text, kind });
  if (notifications.length > 5) notifications.shift();
  window.setTimeout(() => dismissToast(id), kind === "error" ? 12000 : 5500);
}
export function dismissToast(id: number) {
  const i = notifications.findIndex((n) => n.id === id);
  if (i >= 0) notifications.splice(i, 1);
}
export function useActionFeedback(
  scope: string,
  busy: Ref<string>,
  notice: Ref<string>,
  error: Ref<string>,
) {
  watch(busy, (value) => {
    if (value) pendingActions.set(scope, value);
    else pendingActions.delete(scope);
  });
  watch(notice, (value) => {
    if (value) toast(value);
  });
  watch(error, (value) => {
    if (value) toast(value, "error");
  });
}
