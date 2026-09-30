// The fallback used until the parser-engine list (the live source of which
// types this deployment can parse) has loaded. It mirrors
// supportedImportFileExtensions in internal/application/service/knowledge_util.go,
// the set every backend import path accepts; a type missing here would be
// refused in the browser although the server takes it.
const DEFAULT_VALID_TYPES = new Set([
  ...["pdf", "txt", "docx", "doc", "epub"],
  ...["html", "htm", "mhtml", "md", "markdown", "xmind"],
  ...["png", "jpg", "jpeg", "gif", "webp"],
  ...["csv", "xlsx", "xls", "pptx", "ppt", "json"],
  ...["mp3", "wav", "m4a", "flac", "ogg"],
  ...["mp4", "mov", "avi", "mkv", "webm", "wmv", "flv", "m4v"],
]);

export function shouldRejectKnowledgeFileType(filename: string, validTypes?: Set<string> | string[]) {
  const provided = validTypes ? (validTypes instanceof Set ? validTypes : new Set(validTypes)) : undefined;
  const allowed = provided && provided.size > 0 ? provided : DEFAULT_VALID_TYPES;
  const type = filename.substring(filename.lastIndexOf(".") + 1).toLowerCase();

  return !allowed.has(type);
}
