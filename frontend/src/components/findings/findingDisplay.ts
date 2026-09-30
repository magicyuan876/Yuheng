import type { FindingSeverity } from "@/api/findings";

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
    default:
      return type;
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
