import type { DisputeReport, FindingSeverity } from "@/api/findings";

// How a knowledge-health finding reads on screen. Shared by the knowledge
// base's health view and the docs page notice, so a finding looks the same
// wherever it turns up.

type Translate = (key: string) => string;

/**
 * The label for a finding type. Types this build knows get a translated
 * label; any other type — another edition's detector, or one added after this
 * build — shows its raw name, so a finding is never hidden for want of a
 * label.
 */
export function findingTypeLabel(type: string, t: Translate): string {
  switch (type) {
    case "duplicate":
      return t("knowledgeHealth.types.duplicate");
    case "divergent":
      return t("knowledgeHealth.types.divergent");
    case "stale":
      return t("knowledgeHealth.types.stale");
    case "disputed":
      return t("knowledgeHealth.types.disputed");
    default:
      return type;
  }
}

/** One sentence on what a finding of this type means and what to do about
 * it; empty for a type this build does not know. */
export function findingTypeHint(type: string, t: Translate): string {
  switch (type) {
    case "duplicate":
      return t("knowledgeHealth.typeHints.duplicate");
    case "divergent":
      return t("knowledgeHealth.typeHints.divergent");
    case "stale":
      return t("knowledgeHealth.typeHints.stale");
    case "disputed":
      return t("knowledgeHealth.typeHints.disputed");
    default:
      return "";
  }
}

/** Why a closed finding was closed, for its status badge; empty when there is
 * nothing to add. */
export function resolutionLabel(resolution: string | null | undefined, t: Translate): string {
  switch (resolution) {
    case "distinct_scope":
      return t("knowledgeHealth.resolution.distinct_scope");
    case "intentional":
      return t("knowledgeHealth.resolution.intentional");
    default:
      return "";
  }
}

/** Tints for the severity badge, from the shared semantic palette so both
 * themes follow. An unknown severity reads as informational. */
export function severityBadgeClass(severity: FindingSeverity | string): string {
  switch (severity) {
    case "error":
      return "border-destructive/40 bg-destructive/10 text-destructive";
    case "warning":
      return "border-warning/40 bg-warning/10 text-warning";
    default:
      return "border-border bg-muted text-muted-foreground";
  }
}

export function severityLabel(severity: FindingSeverity | string, t: Translate): string {
  switch (severity) {
    case "error":
      return t("knowledgeHealth.severity.error");
    case "warning":
      return t("knowledgeHealth.severity.warning");
    case "info":
      return t("knowledgeHealth.severity.info");
    default:
      return severity;
  }
}

/** A 0..1 ratio as a whole percentage. Out-of-range and missing values are
 * clamped rather than shown as "130%" or "NaN%". */
export function formatPercent(ratio: number | null | undefined): string {
  const value = typeof ratio === "number" && Number.isFinite(ratio) ? ratio : 0;
  return `${Math.round(Math.min(1, Math.max(0, value)) * 100)}%`;
}

/** The finding types about two documents, which have a similarity and an
 * overlap to show; the rest are about one document alone. */
export const PAIR_FINDING_TYPES = new Set(["duplicate", "divergent"]);

/** The finding types a person settles by confirming the document is still right. */
export const CONFIRMABLE_FINDING_TYPES = new Set(["stale", "disputed"]);

/** The reports of a dispute finding, read defensively from its extra: an
 * older server or another detector may put something else there. */
export function disputeReports(extra: Record<string, unknown> | undefined): DisputeReport[] {
  const raw = extra?.reports;
  return Array.isArray(raw) ? (raw as DisputeReport[]) : [];
}
