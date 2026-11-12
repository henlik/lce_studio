import { computed, nextTick, onBeforeUnmount, ref, watch, type Ref } from "vue";
import "../styles/canvasFocus.css";

let activeRestore: (() => void) | undefined;

/** Expand only this workspace; browser fullscreen permissions are not needed. */
export function useCanvasFocus(
  wrapper: Ref<HTMLElement | null>,
  onLayout?: () => void | Promise<void>,
) {
  const expanded = ref(false);
  let priorFocus: HTMLElement | null = null;
  let priorOverflow = "";
  let observer: ResizeObserver | undefined;
  let frame = 0;
  let disposed = false;

  function scheduleLayout() {
    if (disposed) return;
    cancelAnimationFrame(frame);
    void nextTick(() => {
      if (disposed) return;
      frame = requestAnimationFrame(() => {
        if (wrapper.value?.getBoundingClientRect().width) void onLayout?.();
      });
    });
  }
  function restore(returnFocus = true) {
    if (!expanded.value) return;
    expanded.value = false;
    document.body.style.overflow = priorOverflow;
    if (activeRestore === restore) activeRestore = undefined;
    scheduleLayout();
    if (returnFocus && priorFocus?.isConnected) {
      void nextTick(() => priorFocus?.focus({ preventScroll: true }));
    }
  }
  function toggleExpand() {
    if (expanded.value) return restore();
    if (!wrapper.value) return;
    activeRestore?.();
    priorFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    priorOverflow = document.body.style.overflow;
    activeRestore = restore;
    expanded.value = true;
    document.body.style.overflow = "hidden";
    void nextTick(() => wrapper.value?.focus({ preventScroll: true }));
    scheduleLayout();
  }
  function onKey(event: KeyboardEvent) {
    if (!expanded.value || event.defaultPrevented) return;
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      restore();
    } else if (event.key === "Tab" && wrapper.value) {
      const controls = [...wrapper.value.querySelectorAll<HTMLElement>(
        'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]',
      )].filter((element) => element.getClientRects().length > 0);
      const first = controls[0], last = controls[controls.length - 1];
      if (event.shiftKey && (document.activeElement === first || document.activeElement === wrapper.value)) {
        event.preventDefault(); last?.focus();
      } else if (!event.shiftKey && (document.activeElement === last || document.activeElement === wrapper.value)) {
        event.preventDefault(); first?.focus();
      }
    }
  }
  watch(wrapper, (element, previous) => {
    previous?.removeEventListener("keydown", onKey);
    observer?.disconnect();
    // Conditional canvases can disappear without unmounting their component
    // (for example, removing an infrastructure model). Release focus/scroll.
    if (previous && previous !== element && expanded.value) restore(false);
    if (!element) return;
    // Programmatic focus is scoped to the expanded wrapper, never the document.
    if (!element.hasAttribute("tabindex")) element.tabIndex = -1;
    element.addEventListener("keydown", onKey);
    observer = new ResizeObserver(([entry]) => {
      if (!entry) return;
      if (!entry.contentRect.width || !entry.contentRect.height) {
        if (expanded.value) restore(false);
        return;
      }
      scheduleLayout();
    });
    observer.observe(element);
    scheduleLayout();
  }, { flush: "post" });
  onBeforeUnmount(() => {
    restore(false);
    disposed = true;
    observer?.disconnect();
    wrapper.value?.removeEventListener("keydown", onKey);
    cancelAnimationFrame(frame);
  });
  return { expanded, toggleExpand, restore, scheduleLayout };
}

/** A fit-relative SVG viewport that measures its real container, including when
 * a v-show workspace reopens. Width and height both participate in fitting. */
export function useCanvasViewport(viewport: Ref<HTMLElement | null>) {
  const zoom = ref(1);
  const size = ref({ width: 800, height: 420 });
  let observer: ResizeObserver | undefined;
  let disposed = false;
  const svgStyle = computed(() => ({
    width: `${Math.max(1, size.value.width * zoom.value)}px`,
    height: `${Math.max(1, size.value.height * zoom.value)}px`,
  }));
  function measure() {
    const element = viewport.value;
    if (!element || !element.clientWidth || !element.clientHeight) return;
    size.value = { width: element.clientWidth, height: element.clientHeight };
  }
  function fit() {
    zoom.value = 1;
    void nextTick(() => {
      if (disposed) return;
      measure();
      viewport.value?.scrollTo({ left: 0, top: 0 });
    });
  }
  function setZoom(value: number) {
    const element = viewport.value;
    const previous = zoom.value;
    const x = element ? (element.scrollLeft + element.clientWidth / 2) / previous : 0;
    const y = element ? (element.scrollTop + element.clientHeight / 2) / previous : 0;
    zoom.value = Math.min(4, Math.max(0.5, Math.round(value * 100) / 100));
    void nextTick(() => {
      if (disposed || !element) return;
      element.scrollTo({ left: x * zoom.value - element.clientWidth / 2, top: y * zoom.value - element.clientHeight / 2 });
    });
  }
  watch(viewport, (element) => {
    observer?.disconnect();
    if (!element) return;
    observer = new ResizeObserver(measure);
    observer.observe(element);
    measure();
  }, { flush: "post" });
  onBeforeUnmount(() => { disposed = true; observer?.disconnect(); });
  return { zoom, svgStyle, fit, zoomIn: () => setZoom(zoom.value + 0.25), zoomOut: () => setZoom(zoom.value - 0.25) };
}
