import { reactive } from "vue";

/**
 * Helpers shared by the system-administration pages (SystemSettings and
 * Users & workspaces): an inline confirmation anchored to the control it
 * confirms, and the tag / button colour classes those pages draw with.
 * They lived inside SystemSettings.vue until the user-account actions moved
 * to their own page; both pages must keep drawing the same way.
 */

export type PopconfirmTheme = "default" | "warning" | "danger";
export type PopconfirmBtn = { content: string; theme?: "primary" | "danger" | "warning" };

/**
 * An inline confirm (Popover) controller with promise semantics: `ask`
 * opens it and resolves with the user's answer, `finish` settles it, and
 * `onVisibleChange` treats a dismissal as "no". State is a reactive object
 * (not nested refs) so template bindings unwrap.
 */
export function createInlinePopconfirm() {
  const state = reactive({
    visible: false,
    content: "",
    theme: "warning" as PopconfirmTheme,
    confirmBtn: { content: "", theme: "primary" } as PopconfirmBtn,
  });
  let resolver: ((ok: boolean) => void) | null = null;
  let settled = false;

  function ask(opts: { content: string; theme?: PopconfirmTheme; confirmBtn: PopconfirmBtn }): Promise<boolean> {
    state.content = opts.content;
    state.theme = opts.theme ?? "warning";
    state.confirmBtn = opts.confirmBtn;
    settled = false;
    return new Promise((resolve) => {
      resolver = resolve;
      state.visible = true;
    });
  }

  function finish(ok: boolean) {
    if (settled) return;
    settled = true;
    state.visible = false;
    const r = resolver;
    resolver = null;
    r?.(ok);
  }

  function onVisibleChange(v: boolean) {
    if (!v && resolver) finish(false);
  }

  return Object.assign(state, { ask, finish, onVisibleChange });
}

export type InlinePopconfirm = ReturnType<typeof createInlinePopconfirm>;

/**
 * Tag classes matching the old small light t-tag: 20px high, 4px side
 * padding, 3px radius, regular weight, the theme's "-light" tint.
 */
export function settingTagClass(theme: "warning" | "primary" | "danger" | "success"): string {
  const base = "h-5 rounded-[3px] px-1 font-normal";
  switch (theme) {
    case "warning":
      return `${base} bg-[var(--td-warning-color-light)] text-warning`;
    case "primary":
      return `${base} bg-[var(--td-brand-color-light)] text-primary`;
    case "danger":
      return `${base} bg-[var(--td-error-color-light)] text-destructive`;
    default:
      return `${base} bg-[var(--td-success-color-light)] text-success`;
  }
}

/**
 * Solid confirm button colours for the inline confirms: TDesign's
 * confirm-btn theme painted a filled danger / warning / primary button.
 */
export function confirmBtnClass(theme: PopconfirmBtn["theme"]): string {
  if (theme === "danger") return "bg-destructive text-primary-foreground hover:bg-destructive/80";
  if (theme === "warning") return "bg-warning text-primary-foreground hover:bg-warning/80";
  return "";
}

/** The label / control row both pages lay their settings out in. */
export const settingRowClass =
  "border-border flex items-start justify-between border-b py-5 last:border-b-0 max-[860px]:flex-col max-[860px]:gap-3";
export const settingRowLabelClass =
  "max-w-[65%] flex-1 pr-6 max-[860px]:w-full max-[860px]:max-w-none max-[860px]:pr-0";
export const settingRowControlClass =
  "flex min-w-[280px] shrink-0 flex-col items-end gap-1.5 max-[860px]:w-full max-[860px]:items-start";
