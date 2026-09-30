import type { ComposerTranslation } from "vue-i18n";

type AttachmentParsingEvent = {
  success?: boolean;
  output?: string;
  error?: string;
  tool_data?: Record<string, unknown> | null;
};

/**
 * The parsed / skipped counts of an attachment step. The backend reports them
 * in tool_data; a step without them (image analysis) counts as nothing parsed.
 */
export function resolveAttachmentParsingCounts(event: AttachmentParsingEvent): {
  parsed: number;
  skipped: number;
} {
  const toolData = event.tool_data;
  return {
    parsed: Number(toolData?.parsed_count) || 0,
    skipped: Number(toolData?.skipped_count) || 0,
  };
}

export function getAttachmentParsingSummaryHtml(t: ComposerTranslation, event: AttachmentParsingEvent): string {
  if (event.success === false) {
    const err = String(event.error || event.output || "").trim();
    if (!err) return "";
    const normalized = err.replace(/^附件解析失败:\s*/i, "").trim();
    return normalized || err;
  }

  const { parsed, skipped } = resolveAttachmentParsingCounts(event);
  if (parsed === 0 && skipped === 0) {
    return t("agentStream.attachmentParsing.noneReady");
  }
  if (skipped > 0) {
    return t("agentStream.attachmentParsing.parsedWithSkipped", {
      parsed: `<strong>${parsed}</strong>`,
      skipped: `<strong>${skipped}</strong>`,
    });
  }
  return t("agentStream.attachmentParsing.parsedSummary", {
    count: `<strong>${parsed}</strong>`,
  });
}
