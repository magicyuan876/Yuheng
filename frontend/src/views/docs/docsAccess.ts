import type { PrincipalType, SpaceMember, SpaceRole, SpaceVisibility } from "@/api/docs";

// Pure helpers behind the docs UI. They mirror the server's rules
// (internal/docs/service) so forms can validate before a round trip; the
// server remains authoritative.

export const SPACE_ROLE_LEVEL: Record<SpaceRole, number> = { none: 0, reader: 1, writer: 2, admin: 3 };

/** Roles a member can be granted (none is the absence of a grant). */
export const GRANTABLE_SPACE_ROLES: readonly SpaceRole[] = ["reader", "writer", "admin"];

/** Default roles an open space may hand out. */
export const OPEN_SPACE_DEFAULT_ROLES: readonly SpaceRole[] = ["reader", "writer"];

/** Visibilities the UI offers today; public arrives with sharing. */
export const SPACE_VISIBILITIES: readonly SpaceVisibility[] = ["private", "open"];

export function roleAtLeast(role: SpaceRole | undefined, min: SpaceRole): boolean {
  return (SPACE_ROLE_LEVEL[role ?? "none"] ?? 0) >= SPACE_ROLE_LEVEL[min];
}

export function canManageSpace(role: SpaceRole | undefined): boolean {
  return roleAtLeast(role, "admin");
}

export function canEditSpaceContent(role: SpaceRole | undefined): boolean {
  return roleAtLeast(role, "writer");
}

export const MIN_SLUG_LENGTH = 2;
export const MAX_SLUG_LENGTH = 64;
const SLUG_RE = /^[a-z0-9][a-z0-9-]{1,63}$/;

export function isValidSlug(slug: string): boolean {
  return SLUG_RE.test(slug);
}

/**
 * Derive a slug candidate from a name the way the server does: ASCII
 * letters and digits are kept (lower-cased), runs of anything else become
 * one hyphen. Names with no ASCII content yield '' and the server generates
 * a slug instead.
 */
export function suggestSlug(name: string): string {
  let out = "";
  let lastHyphen = true;
  for (const ch of name.toLowerCase()) {
    if (/^[a-z0-9]$/.test(ch)) {
      out += ch;
      lastHyphen = false;
    } else if (!lastHyphen) {
      out += "-";
      lastHyphen = true;
    }
  }
  out = out.replace(/^-+|-+$/g, "");
  if (out.length > MAX_SLUG_LENGTH) {
    out = out.slice(0, MAX_SLUG_LENGTH).replace(/-+$/g, "");
  }
  return out.length < MIN_SLUG_LENGTH ? "" : out;
}

export interface SpaceFormModel {
  name: string;
  slug: string;
  description: string;
  visibility: SpaceVisibility;
  default_role: SpaceRole;
}

/**
 * Apply the visibility/default-role coupling: a private space grants
 * nothing by default; an open space must grant at least reader.
 */
export function normaliseSpaceForm<T extends Pick<SpaceFormModel, "visibility" | "default_role">>(form: T): T {
  if (form.visibility === "private") {
    return { ...form, default_role: "none" };
  }
  if (form.visibility === "open" && !OPEN_SPACE_DEFAULT_ROLES.includes(form.default_role)) {
    return { ...form, default_role: "reader" };
  }
  return form;
}

export interface SpaceFormProblem {
  field: keyof SpaceFormModel;
  key: string;
}

/** Client-side validation; returns i18n keys under `docs.spaces.form`. */
export function validateSpaceForm(form: SpaceFormModel, options: { requireSlug?: boolean } = {}): SpaceFormProblem[] {
  const problems: SpaceFormProblem[] = [];
  const name = form.name.trim();
  if (!name) {
    problems.push({ field: "name", key: "nameRequired" });
  } else if ([...name].length > 100) {
    problems.push({ field: "name", key: "nameTooLong" });
  }
  const slug = form.slug.trim().toLowerCase();
  if (slug) {
    if (!isValidSlug(slug)) problems.push({ field: "slug", key: "slugInvalid" });
  } else if (options.requireSlug) {
    problems.push({ field: "slug", key: "slugRequired" });
  }
  if ([...form.description].length > 4000) {
    problems.push({ field: "description", key: "descriptionTooLong" });
  }
  return problems;
}

/**
 * Order members the way the server lists them: stronger roles first,
 * groups before users within a role, then by name. Used to keep a locally
 * edited list stable without a refetch.
 */
export function sortSpaceMembers<T extends Pick<SpaceMember, "role" | "principal_type" | "name">>(members: T[]): T[] {
  return [...members].sort((a, b) => {
    const level = SPACE_ROLE_LEVEL[b.role] - SPACE_ROLE_LEVEL[a.role];
    if (level !== 0) return level;
    if (a.principal_type !== b.principal_type) return a.principal_type === "group" ? -1 : 1;
    return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  });
}

/**
 * The "last administrator" invariant, mirrored so the UI can disable the
 * control instead of surfacing a server error. `change` is the intended
 * edit: a new role for one principal, or its removal (role null).
 */
export function wouldLeaveNoAdmin(
  members: Pick<SpaceMember, "role" | "principal_type" | "principal_id">[],
  change: { principal_type: PrincipalType; principal_id: string; role: SpaceRole | null },
): boolean {
  let admins = 0;
  for (const m of members) {
    const same = m.principal_type === change.principal_type && m.principal_id === change.principal_id;
    const role = same ? change.role : m.role;
    if (role === "admin") admins++;
  }
  return admins === 0;
}
