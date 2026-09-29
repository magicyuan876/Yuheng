/*
 * The layout classes of the chat view's root and message scroller, kept out
 * of the template so the one piece of layout that changes with state — the
 * room made for the references panel — can be tested without mounting the
 * whole chat view (its stream, stores and router).
 *
 * The references panel (ChatReferencesDrawer) is fixed to the right edge of
 * the window, 420px wide. On a wide screen the chat keeps its messages and
 * input clear of it by reserving that width as right padding; below 960px
 * the panel overlays the chat instead, so nothing moves.
 */

export interface ChatLayoutState {
  sidebarCollapsed: boolean;
  referencesPanelOpen: boolean;
}

export function chatRootClasses(state: ChatLayoutState): string[] {
  const classes = [
    // `chat` is a hook: the wiki fix drawer embeds this view and refits it
    // through `.chat`, `.chat_scroll_box`, `.chat > .input-container` and
    // `.msg_list` (WikiBrowser.vue).
    "chat",
    // min-h-0 lets the scroller below shrink under its content and scroll,
    // rather than pushing the input out of view (the parent route outlet is
    // itself a min-height:0 flex column). No right padding, so the scrollbar
    // sits on the content area's right edge.
    "relative box-border flex min-h-0 min-w-[400px] flex-1 flex-col items-center pb-5 pl-5 text-[20px]",
    state.sidebarCollapsed ? "max-w-[calc(100vw-60px)]" : "max-w-[calc(100vw-260px)]",
  ];
  if (state.referencesPanelOpen) {
    classes.push("has-references-panel", "min-[960px]:pr-[420px]");
  }
  return classes;
}

export function chatScrollBoxClasses(state: Pick<ChatLayoutState, "referencesPanelOpen">): string[] {
  return [
    "chat_scroll_box",
    // Same min-h-0 reasoning as the root: without it a flex-column child
    // grows to fit every message. The native scrollbar is kept (an overlay
    // scrollbar on macOS, as in ChatGPT).
    "box-border min-h-0 w-full flex-1 overflow-y-auto pt-2 [scrollbar-color:auto] [scrollbar-width:auto]",
    // With the panel docked the chat header becomes a bar above the
    // messages (ChatHeader), so the scroller loses its top gap.
    state.referencesPanelOpen ? "min-[960px]:pt-0" : "",
  ].filter(Boolean);
}
