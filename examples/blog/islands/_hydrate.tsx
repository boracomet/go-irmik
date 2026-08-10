import { createElement, type ComponentType } from "react";
import { createRoot } from "react-dom/client";

/**
 * Mount a React island into every `[data-island="{name}"]` node.
 * Go emits empty divs with JSON in data-props — use createRoot (not hydrateRoot).
 */
export function createIsland<P extends object>(
  name: string,
  Component: ComponentType<P>,
): void {
  const nodes = document.querySelectorAll(`[data-island="${name}"]`);
  nodes.forEach((el) => {
    const raw = el.getAttribute("data-props") || "{}";
    let props: P;
    try {
      props = JSON.parse(raw) as P;
    } catch {
      props = {} as P;
    }
    createRoot(el).render(createElement(Component, props));
  });
}
