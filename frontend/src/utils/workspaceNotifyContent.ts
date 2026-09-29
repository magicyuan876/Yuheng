// Shared content renderer for the post-login and post-tenant-switch
// NotifyPlugin cards. Both cards present the same shape ("you are in
// <workspace> as <role>"), so we render them with a unified visual
// language to keep the two surfaces consistent.
//
// Design choices:
//
//   * The workspace name is the primary anchor of the sentence, so it
//     is rendered as plain bold text (no chip / background) — the
//     surrounding sentence already frames it and another box around
//     would be double-emphasis.
//   * The role is a categorical attribute and benefits from a visible
//     coloured tag, so it goes through TDesign's <t-tag> with the same
//     role→theme mapping used by TenantMembers (settings). This keeps
//     "what an owner / admin / contributor / viewer looks like"
//     consistent across the product instead of inventing a parallel
//     chip palette.
//   * No forced wrapping — let the Notify width drive line breaks
//     naturally so the sentence reads as one phrase whenever it fits.
//
// The renderer interpolates a template string carrying `{name}` and
// optionally `{role}` placeholders. Anything around them is rendered
// verbatim so translators can reorder the sentence per locale.

import { h, type Component, type VNode } from "vue";

// Mirrors TenantMembers.roleTagClass() so the "owner is blue, admin is
// orange, ..." identity stays consistent across surfaces; if that map changes
// there, change it here too. The notification body is rendered by TDesign's
// NotifyPlugin, but it is our own markup, so it is styled like any other new
// component: utilities over the bridged tokens. These are the light tints of
// the small tag TDesign drew here before (variant "light").
const ROLE_TAG_CLASS: Record<string, string> = {
  owner: "bg-primary/10 text-primary",
  admin: "bg-warning/10 text-warning",
  contributor: "bg-success/10 text-success",
};
const DEFAULT_ROLE_TAG_CLASS = "bg-secondary text-foreground";

function roleTagClass(roleEnum: string | undefined): string {
  return (roleEnum && ROLE_TAG_CLASS[roleEnum]) || DEFAULT_ROLE_TAG_CLASS;
}

export interface WorkspaceNotifyContentOptions {
  /**
   * The i18n-translated template carrying `{name}` and optionally
   * `{role}` placeholders. Anything around the placeholders is rendered
   * as plain text in the output, in order, so the sentence reads
   * naturally in every locale. Pass the message via `tm()` (not `t()`)
   * so the placeholders survive without interpolation.
   */
  template: string;
  /** Workspace display name. Rendered as bold inline text. */
  name: string;
  /** Human-readable role label, e.g. "所有者" / "Owner". Omit for the no-role variant. */
  roleLabel?: string;
  /** Raw role enum value, e.g. "owner". Drives the tag colour. */
  roleEnum?: string;
  /**
   * Icon for the role chip — pass `useRoleLabel().roleIcon(roleEnum)`.
   * Omitted renders the chip without a leading icon.
   */
  roleIcon?: Component;
}

/**
 * Build a NotifyPlugin `content` factory rendering the workspace name
 * in bold and the role as a small tinted tag. Returns a `() => VNode` so
 * TDesign's NotifyPlugin re-invokes it per render, matching its TNode
 * contract.
 */
export function renderWorkspaceNotifyContent(opts: WorkspaceNotifyContentOptions): () => VNode {
  return () => {
    const tokens = opts.template.split(/(\{name\}|\{role\})/g);
    const parts: VNode[] = [];
    for (const tok of tokens) {
      if (tok === "{name}") {
        parts.push(h("strong", { class: "text-foreground font-semibold" }, opts.name));
      } else if (tok === "{role}") {
        if (opts.roleLabel) {
          parts.push(
            h(
              "span",
              {
                class: [
                  "mx-0.5 inline-flex h-[22px] items-center gap-1 rounded-[3px] px-2 align-middle text-xs leading-none",
                  roleTagClass(opts.roleEnum),
                ],
              },
              [opts.roleIcon ? h(opts.roleIcon, { class: "size-3 shrink-0" }) : null, opts.roleLabel],
            ),
          );
        }
        // If template has {role} but no label was passed, drop the
        // marker silently — caller should pick the no-role template
        // instead, but this guards against the raw "{role}" leaking
        // through if they forget.
      } else if (tok) {
        parts.push(h("span", tok));
      }
    }
    return h("span", { class: "text-muted-foreground leading-[1.6]" }, parts);
  };
}
