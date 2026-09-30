import type { Component } from "vue";
import { BrainIcon, ClipboardPasteIcon, DatabaseIcon, GlobeIcon, PaperclipIcon } from "@lucide/vue";

import type { RetrievalSearchSource } from "@/utils/agent-tool-display";

/**
 * The icon of one quick-answer timeline step. Only the tools in
 * RAG_TIMELINE_TOOL_NAMES reach the timeline; a retrieval step that searched
 * the web alone wears the globe instead of the database.
 */
export function getRagStepIcon(toolName: string, searchSource?: RetrievalSearchSource): Component {
  switch (toolName) {
    case "knowledge_search":
      return searchSource === "web" ? GlobeIcon : DatabaseIcon;
    case "query_understand":
    case "image_analysis":
      return BrainIcon;
    case "attachment_parsing":
      return PaperclipIcon;
    default:
      return ClipboardPasteIcon;
  }
}
