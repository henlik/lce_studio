export function focusField(root: Element | null, selector: string) {
  requestAnimationFrame(() => {
    if (!root) return;
    root
      .querySelectorAll(".validation-focus")
      .forEach((e) => e.classList.remove("validation-focus"));
    const target = root.querySelector<HTMLElement>(selector);
    if (!target) return;
    let parent: HTMLElement | null = target;
    while (parent) {
      if (parent instanceof HTMLDetailsElement) parent.open = true;
      parent = parent.parentElement;
    }
    (
      target.closest(".field, .state-card, .wf-panel, section") || target
    ).classList.add("validation-focus");
    target.scrollIntoView({ block: "center", behavior: "smooth" });
    target.focus({ preventScroll: true });
  });
}
