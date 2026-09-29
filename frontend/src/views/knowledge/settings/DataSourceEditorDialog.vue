<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import {
  createDataSource,
  updateDataSource,
  triggerSync,
  validateConnection,
  validateCredentials,
  listResources,
  resolveResourceAncestors,
  deleteDataSource,
  putDataSourceCredentials,
  deleteDataSourceCredentials,
  type DataSource,
  type Resource,
} from "@/api/datasource";
import {
  BookOpenIcon,
  InfoIcon as InfoCircleFilledIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ChevronUpIcon,
  CircleAlertIcon,
  CircleCheckIcon,
  CircleHelpIcon,
  CircleXIcon,
  FileIcon,
  FolderIcon,
  FolderOpenIcon,
  ListChecksIcon,
  LinkIcon,
  Loader2Icon,
  LockIcon,
  PlusIcon,
  Trash2Icon,
  XIcon,
  type LucideIcon,
} from "@lucide/vue";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

// The resource tree's row icons, keyed by the TDesign names resourceIconName() returns.
const dsIcons: Record<string, LucideIcon> = {
  folder: FolderIcon,
  "folder-open": FolderOpenIcon,
  file: FileIcon,
  "root-list": ListChecksIcon,
  book: BookOpenIcon,
};

function dsIcon(name: string): LucideIcon {
  return dsIcons[name] ?? FileIcon;
}
import DataSourceTypeIcon from "./DataSourceTypeIcon.vue";
import { getDatasourceIconUrl } from "./datasourceIcons";

const props = defineProps<{
  kbId: string;
  dataSource: DataSource | null;
}>();

const visible = defineModel<boolean>("visible", { default: false });
const emit = defineEmits<{ saved: [] }>();
const { t } = useI18n();

const isEdit = computed(() => !!props.dataSource);
const step = ref(0);
const submitting = ref(false);

// In edit mode the credential "configured?" flag travels on the main
// DataSource response (DataSource.credentials.credentials.configured —
// server-side dto.DataSourceResponse.Credentials). True iff a credential
// map is currently stored server-side.
const credentialsConfigured = ref(false);

// "Replace credentials" mode toggle in edit. Defaults to false: a configured
// connector shows a small "Credentials configured ✓" line with Replace /
// Remove actions. Toggling Replace reveals the credential inputs so the
// user can type a new set. Untoggling discards anything typed.
const replaceCredentialsMode = ref(false);

// Whether the credential input section is interactive right now. In create
// mode it's always shown; in edit mode only when the user opted in to
// Replace, OR when nothing is configured yet (degenerate case where the
// data source row exists with no credentials stored).
const credentialsInputVisible = computed(() => {
  if (!isEdit.value) return true;
  if (!credentialsConfigured.value) return true;
  return replaceCredentialsMode.value;
});

function refreshCredentialsStatus() {
  // Re-derive from whatever the parent passed in props.dataSource. Called
  // when the dialog opens or props.dataSource is swapped; the parent is
  // expected to re-fetch the data source list after credential mutations
  // so the new metadata flows in here automatically.
  if (!isEdit.value || !props.dataSource) {
    credentialsConfigured.value = false;
    return;
  }
  credentialsConfigured.value = props.dataSource.credentials?.credentials?.configured === true;
}

// Single-click remove with toast feedback. Mirrors the CredentialResource
// component's UX: the secret is irrecoverable client-side either way, so a
// modal confirm just adds friction. The danger-themed button is the deterrent.
const pendingRemoveCredentials = ref(false);
const removingCredentials = ref(false);

function requestRemoveCredentials() {
  pendingRemoveCredentials.value = true;
}

function cancelPendingRemoveCredentials() {
  pendingRemoveCredentials.value = false;
}

async function confirmRemoveCredentials() {
  if (!props.dataSource?.id) return;
  removingCredentials.value = true;
  try {
    await deleteDataSourceCredentials(props.dataSource.id);
    credentialsConfigured.value = false;
    replaceCredentialsMode.value = false;
    pendingRemoveCredentials.value = false;
    form.value.config.credentials = {};
    MessagePlugin.success(t("credential.removedToast"));
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("credential.removeFailed"));
  } finally {
    removingCredentials.value = false;
  }
}

function cancelReplaceCredentials() {
  replaceCredentialsMode.value = false;
  pendingRemoveCredentials.value = false;
  form.value.config.credentials = {};
  rssAuthHeaders.value = [];
  testResult.value = credentialsConfigured.value ? "success" : "";
  testErrorMsg.value = "";
}

interface CustomHeaderItem {
  key: string;
  value: string;
}

const rssAuthHeaders = ref<CustomHeaderItem[]>([]);

function serializeAuthHeaders(items: CustomHeaderItem[]): string {
  return items
    .filter((h) => h.key.trim())
    .map((h) => `${h.key.trim()}: ${h.value}`)
    .join("\n");
}

function syncRssAuthHeadersToCredentials() {
  if (form.value.type !== "rss") return;
  const serialized = serializeAuthHeaders(rssAuthHeaders.value);
  if (serialized) {
    form.value.config.credentials.auth_headers = serialized;
  } else {
    delete form.value.config.credentials.auth_headers;
  }
}

// Feed URLs may still live in credentials on older rows (not returned by the
// API). The backend copies them into settings on read; fall back to the
// selected feed resource IDs when settings are still empty.
function hydrateRssFeedUrlsFromConfig(config: { settings?: Record<string, any>; resource_ids?: string[] }) {
  const settings = config.settings || {};
  if (String(settings.feed_urls || "").trim()) {
    return { ...settings };
  }
  const ids = config.resource_ids || [];
  if (ids.length === 0) {
    return { ...settings };
  }
  return { ...settings, feed_urls: ids.join("\n") };
}

function addRssAuthHeader() {
  rssAuthHeaders.value.push({ key: "", value: "" });
}

function removeRssAuthHeader(idx: number) {
  rssAuthHeaders.value.splice(idx, 1);
}

function needsConnectionTest(): boolean {
  return !(isEdit.value && credentialsConfigured.value && !replaceCredentialsMode.value);
}

function enterReplaceCredentials() {
  pendingRemoveCredentials.value = false;
  replaceCredentialsMode.value = true;
  testResult.value = "";
  testErrorMsg.value = "";
}

// Form data
const form = ref({
  name: "",
  type: "",
  config: {
    credentials: {} as Record<string, any>,
    resource_ids: [] as string[],
    settings: {} as Record<string, any>,
  },
  sync_schedule: "0 0 */6 * * *",
  sync_mode: "incremental" as "incremental" | "full",
  conflict_strategy: "overwrite" as "overwrite" | "skip",
  sync_deletions: true,
});

// Step 2: Resources
const resources = ref<Resource[]>([]);
const loadingResources = ref(false);
const selectedResourceIds = ref<string[]>([]);
const expandedResourceIds = ref(new Set<string>());
// Lazy loading: parents whose children have already been fetched, and parents
// currently being fetched. Used to load hierarchical sources (e.g. Feishu wiki)
// one level at a time instead of traversing the whole tree up front (#1672).
const loadedChildrenIds = ref(new Set<string>());
const loadingChildrenIds = ref(new Set<string>());
// True when the initial listing already returned the whole tree (connectors like
// Notion populate parent_id on the first call). In that case expanding a node
// never needs an extra request.
const treeFullyLoaded = ref(false);

// Drive (云盘) root input: the Drive connectors have no "list spaces" API, so
// the user must supply a root folder_token. We collect it here, write it into
// form.config.resource_ids as the single root, then loadResources lists its
// children. See 飞书云盘数据源设计.md §5.2 / ADR-0004.
const driveFolderToken = ref("");
// 必填校验的内联错误文案：非空时输入框显示 error 状态 + 下方 tips,
// 替代全局 MessagePlugin,与表单字段的就地校验风格一致。
const driveFolderTokenError = ref("");
const driveRootLoaded = ref(false);
const isDriveConnector = (type: string) => type === "feishu_drive" || type === "lark_drive";
const isWikiConnector = (type: string) => type === "feishu" || type === "lark";

// Pasted wiki doc links. Personal document libraries (个人文档库) are hidden
// wiki spaces that the space listing never returns, so docs living there can
// only be added by URL. Stored in resource_ids as "node:<node_token>"; the
// backend resolves the owning space via get_node at sync time.
interface ManualWikiDoc {
  id: string;
  name: string;
}
const manualWikiDocs = ref<ManualWikiDoc[]>([]);
const wikiDocInput = ref("");
const wikiDocError = ref("");
const addingWikiDoc = ref(false);
const WIKI_NODE_ID_PREFIX = "node:";

// extractWikiNodeToken accepts a bare node token or a wiki doc URL
// (https://xxx.feishu.cn/wiki/<token>, dedicated domains included) and returns
// the token. Returns "" when nothing usable is found.
function extractWikiNodeToken(input: string): string {
  const raw = (input || "").trim();
  if (!raw) return "";
  if (!raw.includes("://") && !raw.includes("/")) return raw;
  const match = raw.match(/\/(?:wiki|docx|docs)\/([^/?#]+)/);
  if (match && match[1]) return match[1];
  try {
    const u = new URL(raw);
    const segs = u.pathname.split("/").filter(Boolean);
    return segs[segs.length - 1] || "";
  } catch {
    return "";
  }
}

// addWikiDocByUrl validates the pasted link against the backend (listResources
// with the "node:<token>" parentID resolves it via get_node) so the user gets
// the doc's real title in the tag and an early permission error instead of a
// silent sync failure.
async function addWikiDocByUrl() {
  const token = extractWikiNodeToken(wikiDocInput.value);
  if (!token) {
    wikiDocError.value = t("datasource.wikiDoc.invalid");
    return;
  }
  const id = WIKI_NODE_ID_PREFIX + token;
  if (manualWikiDocs.value.some((d) => d.id === id)) {
    wikiDocInput.value = "";
    return;
  }
  wikiDocError.value = "";
  addingWikiDoc.value = true;
  try {
    if (!tempDsId.value) {
      const res = await createDataSource({
        ...form.value,
        knowledge_base_id: props.kbId,
        status: "paused",
      } as any);
      const created = res?.data || res;
      tempDsId.value = created.id;
    }
    const res = await listResources(tempDsId.value, id);
    const list = res?.data || res || [];
    manualWikiDocs.value.push({ id, name: list[0]?.name || token });
    wikiDocInput.value = "";
  } catch (e: any) {
    wikiDocError.value = e?.message || e?.error || t("datasource.wikiDoc.loadFailed");
  }
  addingWikiDoc.value = false;
}

function removeManualWikiDoc(index: number) {
  manualWikiDocs.value.splice(index, 1);
}

// Feishu-family connectors support syncing embedded/drive video files into
// the video ingestion pipeline (timeline transcription + keyframe captions).
const isFeishuFamily = (type: string) =>
  type === "feishu" || type === "lark" || type === "feishu_drive" || type === "lark_drive";

const DEFAULT_VIDEO_MAX_MB = 2048;

// settings.sync_video_attachments defaults to true when absent.
const videoSyncEnabled = computed({
  get: () => form.value.config.settings.sync_video_attachments !== false,
  set: (v: boolean) => {
    form.value.config.settings.sync_video_attachments = v;
  },
});

const videoMaxMB = computed({
  get: () => {
    const n = Number(form.value.config.settings.video_max_mb);
    return Number.isFinite(n) && n > 0 ? n : DEFAULT_VIDEO_MAX_MB;
  },
  set: (v: number) => {
    form.value.config.settings.video_max_mb = Number.isFinite(v) && v > 0 ? v : DEFAULT_VIDEO_MAX_MB;
  },
});

// settings.sync_linked_pages defaults to false when absent: link-heavy docs
// would otherwise trigger mass web crawling the user never asked for.
const linkedPagesEnabled = computed({
  get: () => form.value.config.settings.sync_linked_pages === true,
  set: (v: boolean) => {
    form.value.config.settings.sync_linked_pages = v;
  },
});
const isGitLabConnector = (type: string) => type === "gitlab";

interface GitLabProjectInput {
  project_id: string;
  ref: string;
  pathsText: string;
}
const gitlabProjects = ref<GitLabProjectInput[]>([]);
function syncGitLabProjectsToSettings() {
  if (!isGitLabConnector(form.value.type)) return;
  form.value.config.settings.projects = gitlabProjects.value
    .filter((project) => project.project_id.trim())
    .map((project) => ({
      project_id: project.project_id.trim(),
      ref: project.ref.trim(),
      paths: project.pathsText
        .split(/[\n,]/)
        .map((path) => path.trim())
        .filter(Boolean),
    }));
}
function addGitLabProject() {
  gitlabProjects.value.push({ project_id: "", ref: "", pathsText: "" });
}
function removeGitLabProject(index: number) {
  gitlabProjects.value.splice(index, 1);
  syncGitLabProjectsToSettings();
}

// extractDriveFolderToken accepts either a bare folder_token or a Drive folder
// URL (https://xxx.feishu.cn/drive/folder/<token> or the Lark equivalent
// https://xxx.larksuite.com/drive/folder/<token>) and returns the token.
// Matching is path-based, host-agnostic. Trims surrounding whitespace.
// Returns "" when nothing usable is found.
function extractDriveFolderToken(input: string): string {
  const raw = (input || "").trim();
  if (!raw) return "";
  // Bare token: no scheme, no slash - use as-is.
  if (!raw.includes("://") && !raw.includes("/")) return raw;
  // URL form: extract the segment after /drive/folder/.
  const match = raw.match(/\/drive\/folder\/([^/?#]+)/);
  if (match && match[1]) return match[1];
  // Fallback: last path segment of a URL, or the raw string.
  try {
    const u = new URL(raw);
    const segs = u.pathname.split("/").filter(Boolean);
    return segs[segs.length - 1] || raw;
  } catch {
    return raw;
  }
}

// loadDriveRoot writes the user-supplied folder_token (or the token extracted
// from a pasted URL) as the root resource_id, then lists the root's children
// so the lazy-load tree can populate. On failure it classifies the error so the
// user gets an actionable hint (e.g. share the folder with the app) instead of
// a raw Feishu error body.
async function loadDriveRoot() {
  const token = extractDriveFolderToken(driveFolderToken.value);
  if (!token) {
    driveFolderTokenError.value = t("datasource.drive.folderTokenRequired");
    return;
  }
  driveFolderTokenError.value = "";
  // Normalize the input so the user sees the extracted token, not the full URL.
  driveFolderToken.value = token;
  form.value.config.resource_ids = [token];
  driveRootLoaded.value = false;
  loadingResources.value = true;
  try {
    if (!tempDsId.value) {
      const res = await createDataSource({
        ...form.value,
        knowledge_base_id: props.kbId,
        status: "paused",
      } as any);
      const created = res?.data || res;
      tempDsId.value = created.id;
    } else {
      // Edit mode OR a previously-created temp row: persist the new folder_token
      // so listResources sees the updated config. Previously this branch skipped
      // updates in edit mode, leaving listResources reading the old folder_token.
      await updateDataSource(tempDsId.value, {
        ...form.value,
        knowledge_base_id: props.kbId,
      } as any);
    }

    const res = await listResources(tempDsId.value);
    resources.value = res?.data || res || [];
    if (resources.value.length > 0) {
      // Mirror loadResources' tree initialization: index parents that already
      // arrived with children and auto-expand them.
      const parentsWithChildren = new Set<string>();
      for (const r of resources.value) {
        if (r.parent_id) parentsWithChildren.add(r.parent_id);
      }
      loadedChildrenIds.value = parentsWithChildren;
      loadingChildrenIds.value = new Set<string>();
      treeFullyLoaded.value = parentsWithChildren.size > 0;
      expandedResourceIds.value = new Set(
        resources.value
          .filter((r) => !r.parent_id && r.has_children && parentsWithChildren.has(r.external_id))
          .map((r) => r.external_id),
      );
      driveRootLoaded.value = true;
      // In edit mode, reveal pre-existing selections that live below the
      // (not-yet-expanded) tree so they are visible and checked - mirrors
      // loadResources' behavior for non-Drive connectors.
      if (isEdit.value && !treeFullyLoaded.value) {
        const loaded = new Set(resources.value.map((r) => r.external_id));
        const hidden = selectedResourceIds.value.filter((id) => !loaded.has(id));
        if (hidden.length > 0) void revealExistingSelections(hidden);
      }
    }
  } catch (e: any) {
    MessagePlugin.error(classifyDriveLoadError(e));
  }
  loadingResources.value = false;
}

// classifyDriveLoadError turns a raw Drive list error into an actionable i18n
// message. The Feishu list API returns 403 with code=1061004 when the app has
// not been shared the target folder; without this the user sees "forbidden"
// and has no idea what to do.
function classifyDriveLoadError(e: any): string {
  const raw = String(e?.message || e?.error || "");
  const lower = raw.toLowerCase();
  // 403 / forbidden / 1061004 -> the app lacks access to this specific folder;
  // the user must share it with the app's group in Feishu Drive.
  if (
    lower.includes("status=403") ||
    lower.includes("forbidden") ||
    lower.includes('"code":1061004') ||
    lower.includes("code=1061004")
  ) {
    return t("datasource.drive.loadForbiddenHint");
  }
  // 401 / auth -> app credentials wrong or app lacks the drive scopes.
  if (lower.includes("status=401") || lower.includes("auth") || lower.includes("1061005")) {
    return t("datasource.drive.loadAuthHint");
  }
  // Invalid / not-found folder_token.
  if (lower.includes("1061003") || lower.includes("not found")) {
    return t("datasource.drive.loadNotFoundHint");
  }
  return raw || t("datasource.resourceLoadFailed");
}

// Shared children/parent indexes — used by tree rendering and selection logic
const childrenMap = computed(() => {
  const map = new Map<string, Resource[]>();
  for (const r of resources.value) {
    if (r.parent_id) {
      const siblings = map.get(r.parent_id);
      if (siblings) siblings.push(r);
      else map.set(r.parent_id, [r]);
    }
  }
  return map;
});

const parentMap = computed(() => {
  const map = new Map<string, string>();
  for (const r of resources.value) {
    if (r.parent_id) map.set(r.external_id, r.parent_id);
  }
  return map;
});

// `selectedResourceIds` is a MINIMAL COVER SET: only the roots of fully-selected
// subtrees. Sending this to the backend gives "sync these IDs and all descendants"
// semantics — including any pages added later under a selected parent.
type CheckState = "checked" | "indeterminate" | "unchecked";

const checkStates = computed(() => {
  const states = new Map<string, CheckState>();
  const cover = new Set(selectedResourceIds.value);

  // Single post-order walk: a node is `checked` if itself or any ancestor is
  // in the cover set; otherwise `indeterminate` if any descendant is checked;
  // otherwise `unchecked`. Returns whether the subtree contains a checked node.
  function walk(node: Resource, ancestorChecked: boolean): boolean {
    const selfChecked = ancestorChecked || cover.has(node.external_id);
    let descendantChecked = false;
    for (const c of childrenMap.value.get(node.external_id) || []) {
      if (walk(c, selfChecked)) descendantChecked = true;
    }
    if (selfChecked) states.set(node.external_id, "checked");
    else states.set(node.external_id, descendantChecked ? "indeterminate" : "unchecked");
    return selfChecked || descendantChecked;
  }
  for (const r of resources.value) {
    if (!r.parent_id) walk(r, false);
  }
  return states;
});

function toggleExpand(id: string) {
  const next = new Set(expandedResourceIds.value);
  if (next.has(id)) {
    next.delete(id);
    expandedResourceIds.value = next;
    return;
  }
  next.add(id);
  expandedResourceIds.value = next;
  void ensureChildrenLoaded(id);
}

// ensureChildrenLoaded fetches the direct children of a node on demand. It is a
// no-op when the connector already delivered the whole tree in one call (e.g.
// Notion) or when this node's children have already been fetched.
async function ensureChildrenLoaded(id: string) {
  if (!tempDsId.value) return;
  if (loadedChildrenIds.value.has(id) || loadingChildrenIds.value.has(id)) return;
  if (treeFullyLoaded.value) {
    loadedChildrenIds.value = new Set(loadedChildrenIds.value).add(id);
    return;
  }

  loadingChildrenIds.value = new Set(loadingChildrenIds.value).add(id);
  try {
    const res = await listResources(tempDsId.value, id);
    const children: Resource[] = res?.data || res || [];
    if (children.length > 0) {
      const existing = new Set(resources.value.map((r) => r.external_id));
      const merged = resources.value.slice();
      for (const c of children) {
        if (!existing.has(c.external_id)) merged.push(c);
      }
      resources.value = merged;
    }
    loadedChildrenIds.value = new Set(loadedChildrenIds.value).add(id);
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.resourceLoadFailed"));
    // Collapse again so the user can retry the expand.
    const next = new Set(expandedResourceIds.value);
    next.delete(id);
    expandedResourceIds.value = next;
  } finally {
    const s = new Set(loadingChildrenIds.value);
    s.delete(id);
    loadingChildrenIds.value = s;
  }
}

const visibleTree = computed(() => {
  const roots = resources.value.filter((r) => !r.parent_id);
  const result: { resource: Resource; depth: number }[] = [];
  function walk(items: Resource[], depth: number) {
    for (const r of items) {
      result.push({ resource: r, depth });
      if (r.has_children && expandedResourceIds.value.has(r.external_id)) {
        walk(childrenMap.value.get(r.external_id) || [], depth + 1);
      }
    }
  }
  walk(roots, 0);
  return result;
});

// Connection test
const testing = ref(false);
const testResult = ref<"success" | "error" | "">("");
const testErrorMsg = ref("");

// Collapsible prereq in Step 1
const prereqExpanded = ref(false);

// Temp data source for resource listing
const tempDsId = ref("");

// Schedule presets
const schedulePresets = computed(() => [
  { label: t("datasource.schedule30min"), value: "0 */30 * * * *" },
  { label: t("datasource.schedule1h"), value: "0 0 * * * *" },
  { label: t("datasource.schedule6h"), value: "0 0 */6 * * *" },
  { label: t("datasource.schedule12h"), value: "0 0 */12 * * *" },
  { label: t("datasource.schedule24h"), value: "0 0 2 * * *" },
]);

// --- Connector definitions ---
interface ConnectorDef {
  type: string;
  available: boolean;
  docUrl: string;
  permissionDocUrl: string;
  permissionPageUrl: string;
  requiredPermissions: string[];
  fields: {
    key: string;
    labelKey: string;
    placeholder: string;
    secret?: boolean;
    optional?: boolean;
    hintKey?: string;
    multiline?: boolean;
    fieldType?: "custom_headers";
  }[];
}

const connectorDefs = computed<ConnectorDef[]>(() => [
  {
    type: "feishu",
    available: true,
    docUrl: "https://open.feishu.cn/app",
    permissionDocUrl: "https://open.feishu.cn/document/server-docs/docs/wiki-v2/wiki-overview",
    permissionPageUrl: "https://open.feishu.cn/app",
    requiredPermissions: [
      "wiki:wiki:readonly",
      "drive:drive:readonly",
      "drive:export:readonly",
      "docx:document:readonly",
    ],
    fields: [
      { key: "app_id", labelKey: "datasource.field.appId", placeholder: "cli_xxxx" },
      { key: "app_secret", labelKey: "datasource.field.appSecret", placeholder: "", secret: true },
      {
        key: "base_url",
        labelKey: "datasource.field.baseUrl",
        placeholder: "https://open.feishu.cn",
        optional: true,
        hintKey: "datasource.field.baseUrlHint",
      },
      {
        key: "web_base_url",
        labelKey: "datasource.field.webBaseUrl",
        placeholder: "https://feishu.cn",
        optional: true,
        hintKey: "datasource.field.webBaseUrlHint",
      },
    ],
  },
  {
    // Lark is Feishu's international cloud. Same wiki/docx/drive APIs and the
    // same scope identifiers, but a separate console, tenant and app — an app
    // created on open.feishu.cn cannot read a Lark wiki.
    type: "lark",
    available: true,
    docUrl: "https://open.larksuite.com/app",
    permissionDocUrl: "https://open.larksuite.com/document/server-docs/docs/wiki-v2/wiki-overview",
    permissionPageUrl: "https://open.larksuite.com/app",
    requiredPermissions: [
      "wiki:wiki:readonly",
      "drive:drive:readonly",
      "drive:export:readonly",
      "docx:document:readonly",
    ],
    fields: [
      { key: "app_id", labelKey: "datasource.field.appId", placeholder: "cli_xxxx" },
      { key: "app_secret", labelKey: "datasource.field.appSecret", placeholder: "", secret: true },
      {
        key: "base_url",
        labelKey: "datasource.field.baseUrl",
        placeholder: "https://open.larksuite.com",
        optional: true,
        hintKey: "datasource.field.baseUrlHint",
      },
      {
        key: "web_base_url",
        labelKey: "datasource.field.webBaseUrl",
        placeholder: "https://larksuite.com",
        optional: true,
        hintKey: "datasource.field.webBaseUrlHint",
      },
    ],
  },
  {
    // Feishu Drive (云盘) mode: sync documents/files under a user-supplied Drive
    // folder_token. Same auth as the wiki connector but no wiki:wiki:readonly
    // scope - Drive only needs drive + export + docx.
    type: "feishu_drive",
    available: true,
    docUrl: "https://open.feishu.cn/app",
    permissionDocUrl: "https://open.feishu.cn/document/server-docs/docs/drive-v1/file/list",
    permissionPageUrl: "https://open.feishu.cn/app",
    requiredPermissions: ["drive:drive:readonly", "drive:export:readonly", "docx:document:readonly"],
    fields: [
      { key: "app_id", labelKey: "datasource.field.appId", placeholder: "cli_xxxx" },
      { key: "app_secret", labelKey: "datasource.field.appSecret", placeholder: "", secret: true },
      {
        key: "base_url",
        labelKey: "datasource.field.baseUrl",
        placeholder: "https://open.feishu.cn",
        optional: true,
        hintKey: "datasource.field.baseUrlHint",
      },
      {
        key: "web_base_url",
        labelKey: "datasource.field.webBaseUrl",
        placeholder: "https://feishu.cn",
        optional: true,
        hintKey: "datasource.field.webBaseUrlHint",
      },
    ],
  },
  {
    // Lark Drive: international counterpart of feishu_drive.
    type: "lark_drive",
    available: true,
    docUrl: "https://open.larksuite.com/app",
    permissionDocUrl: "https://open.larksuite.com/document/server-docs/docs/drive-v1/file/list",
    permissionPageUrl: "https://open.larksuite.com/app",
    requiredPermissions: ["drive:drive:readonly", "drive:export:readonly", "docx:document:readonly"],
    fields: [
      { key: "app_id", labelKey: "datasource.field.appId", placeholder: "cli_xxxx" },
      { key: "app_secret", labelKey: "datasource.field.appSecret", placeholder: "", secret: true },
      {
        key: "base_url",
        labelKey: "datasource.field.baseUrl",
        placeholder: "https://open.larksuite.com",
        optional: true,
        hintKey: "datasource.field.baseUrlHint",
      },
      {
        key: "web_base_url",
        labelKey: "datasource.field.webBaseUrl",
        placeholder: "https://larksuite.com",
        optional: true,
        hintKey: "datasource.field.webBaseUrlHint",
      },
    ],
  },
  {
    type: "notion",
    available: true,
    docUrl: "https://www.notion.so/my-integrations",
    permissionDocUrl: "",
    permissionPageUrl: "",
    requiredPermissions: [],
    fields: [{ key: "api_key", labelKey: "datasource.field.integrationToken", placeholder: "ntn_xxxx", secret: true }],
  },
  {
    type: "yuque",
    available: true,
    docUrl: "https://www.yuque.com/yuque/developer/api",
    permissionDocUrl: "https://www.yuque.com/yuque/developer/api",
    permissionPageUrl: "https://www.yuque.com/settings/tokens",
    requiredPermissions: ["repo:read", "doc:read"],
    fields: [
      { key: "api_token", labelKey: "datasource.field.apiToken", placeholder: "", secret: true },
      {
        key: "base_url",
        labelKey: "datasource.field.baseUrl",
        placeholder: "https://www.yuque.com",
        optional: true,
        hintKey: "datasource.field.baseUrlHint",
      },
    ],
  },
  {
    // Tencent IMA (ima.qq.com). Uses the OpenAPI at /openapi/wiki/v1 with two
    // static headers (ima-openapi-clientid + ima-openapi-apikey); no OAuth.
    type: "ima",
    available: true,
    docUrl: "https://ima.qq.com/agent-interface",
    permissionDocUrl: "https://ima.qq.com/agent-interface",
    permissionPageUrl: "https://ima.qq.com/agent-interface",
    requiredPermissions: [],
    fields: [
      { key: "client_id", labelKey: "datasource.field.imaClientId", placeholder: "", secret: true },
      { key: "api_key", labelKey: "datasource.field.imaApiKey", placeholder: "", secret: true },
      {
        key: "base_url",
        labelKey: "datasource.field.baseUrl",
        placeholder: "https://ima.qq.com",
        optional: true,
        hintKey: "datasource.field.baseUrlHint",
      },
    ],
  },
  {
    type: "rss",
    available: true,
    docUrl: "",
    permissionDocUrl: "",
    permissionPageUrl: "",
    requiredPermissions: [],
    fields: [
      {
        key: "auth_headers",
        labelKey: "datasource.field.authHeaders",
        placeholder: "",
        optional: true,
        hintKey: "datasource.field.authHeadersHint",
        fieldType: "custom_headers",
      },
    ],
  },
  {
    type: "gitlab",
    available: true,
    docUrl: "",
    permissionDocUrl: "",
    permissionPageUrl: "",
    requiredPermissions: [],
    fields: [
      { key: "base_url", labelKey: "datasource.gitlab.baseUrl", placeholder: "https://gitlab.example.com" },
      { key: "access_token", labelKey: "datasource.gitlab.accessToken", placeholder: "", secret: true },
    ],
  },
]);

const currentDef = computed(() => connectorDefs.value.find((d) => d.type === form.value.type));

// --- Drawer lifecycle ---
watch(visible, async (v) => {
  if (!v) {
    if (!isEdit.value && tempDsId.value) {
      try {
        await deleteDataSource(tempDsId.value);
      } catch {
        // Ignore cleanup errors
      }
      tempDsId.value = "";
    }
    return;
  }
  step.value = isEdit.value ? 1 : 0;
  testResult.value = "";
  testErrorMsg.value = "";
  tempDsId.value = "";
  prereqExpanded.value = false;
  pendingRemoveCredentials.value = false;
  resources.value = [];
  selectedResourceIds.value = [];
  expandedResourceIds.value = new Set();
  loadedChildrenIds.value = new Set();
  loadingChildrenIds.value = new Set();
  treeFullyLoaded.value = false;
  driveFolderToken.value = "";
  driveFolderTokenError.value = "";
  driveRootLoaded.value = false;
  manualWikiDocs.value = [];
  wikiDocInput.value = "";
  wikiDocError.value = "";
  rssAuthHeaders.value = [];
  gitlabProjects.value = [];

  if (isEdit.value && props.dataSource) {
    // Reset edit/replace toggle every open so an aborted replace doesn't
    // carry over. credentialsConfigured will be refreshed from the
    // /credentials subresource (run separately below).
    replaceCredentialsMode.value = false;
    credentialsConfigured.value = false;
    refreshCredentialsStatus();
    testResult.value = credentialsConfigured.value ? "success" : "";
    const editConfig = props.dataSource.config || {};
    form.value = {
      name: props.dataSource.name,
      type: props.dataSource.type,
      config: {
        credentials: {},
        resource_ids: editConfig.resource_ids || [],
        settings:
          props.dataSource.type === "rss" ? hydrateRssFeedUrlsFromConfig(editConfig) : editConfig.settings || {},
      },
      sync_schedule: props.dataSource.sync_schedule,
      sync_mode: props.dataSource.sync_mode,
      conflict_strategy: props.dataSource.conflict_strategy,
      sync_deletions: props.dataSource.sync_deletions,
    };
    // Pasted-doc IDs ("node:<token>") are managed as tags, not tree selections;
    // keeping them out of selectedResourceIds stops the tree reveal logic from
    // trying to surface nodes that are not part of the browsable space tree.
    const savedIds: string[] = form.value.config?.resource_ids || [];
    manualWikiDocs.value = savedIds
      .filter((id) => id.startsWith(WIKI_NODE_ID_PREFIX))
      .map((id) => ({ id, name: id.slice(WIKI_NODE_ID_PREFIX.length) }));
    selectedResourceIds.value = savedIds.filter((id) => !id.startsWith(WIKI_NODE_ID_PREFIX));
    if (isGitLabConnector(form.value.type)) {
      const savedProjects = Array.isArray(form.value.config.settings.projects)
        ? form.value.config.settings.projects
        : [];
      gitlabProjects.value = savedProjects.map((project: any) => ({
        project_id: String(project.project_id || ""),
        ref: String(project.ref || ""),
        pathsText: Array.isArray(project.paths) ? project.paths.join("\n") : "",
      }));
    }
    // Pre-fill the Drive root folder_token from the saved resource_ids so the
    // user sees what they previously entered. driveRootLoaded stays false: the
    // tree has not been listed yet, and clicking "load" triggers listResources
    // + revealExistingSelections so pre-existing selections are revealed.
    if (isDriveConnector(form.value.type)) {
      const rids = form.value.config?.resource_ids || [];
      if (rids.length > 0) {
        // resource_id is "folderToken" or "folderToken:fileToken"; the root is
        // the first segment.
        driveFolderToken.value = rids[0].split(":")[0];
      }
    }
    tempDsId.value = props.dataSource.id;
  } else {
    replaceCredentialsMode.value = false;
    credentialsConfigured.value = false;
    form.value = {
      name: "",
      type: "",
      config: { credentials: {}, resource_ids: [], settings: {} },
      sync_schedule: "0 0 */6 * * *",
      sync_mode: "incremental",
      conflict_strategy: "overwrite",
      sync_deletions: true,
    };
  }
});

watch(
  () => form.value.config.credentials,
  () => {
    if (needsConnectionTest()) {
      testResult.value = "";
      testErrorMsg.value = "";
    }
  },
  { deep: true },
);

watch(
  rssAuthHeaders,
  () => {
    syncRssAuthHeadersToCredentials();
    if (needsConnectionTest()) {
      testResult.value = "";
      testErrorMsg.value = "";
    }
  },
  { deep: true },
);

watch(
  () => form.value.config.settings.feed_urls,
  () => {
    if (needsConnectionTest()) {
      testResult.value = "";
      testErrorMsg.value = "";
    }
  },
);

function selectType(def: ConnectorDef) {
  if (!def.available) return;
  form.value.type = def.type;
  form.value.name = t(`datasource.connector.${def.type}`);
  form.value.config.credentials = {};
  if (isGitLabConnector(def.type)) addGitLabProject();
  rssAuthHeaders.value = [];
  step.value = 1;
}

// --- Test connection (stateless, no DB write) ---
async function testConnection() {
  syncRssAuthHeadersToCredentials();
  if (!validateRssFeedUrls()) return;
  if (!isEdit.value || !credentialsConfigured.value || replaceCredentialsMode.value) {
    const fields = currentDef.value?.fields || [];
    for (const f of fields) {
      if (f.optional || f.fieldType === "custom_headers") continue;
      if (!form.value.config.credentials[f.key]) {
        MessagePlugin.warning(`${t(f.labelKey)} ${t("datasource.isRequired")}`);
        return;
      }
    }
  }

  testing.value = true;
  testResult.value = "";
  testErrorMsg.value = "";
  try {
    if (isEdit.value && tempDsId.value) {
      await updateDataSource(tempDsId.value, {
        ...form.value,
        knowledge_base_id: props.kbId,
      } as any);
      await validateConnection(tempDsId.value);
    } else {
      const creds = { ...form.value.config.credentials };
      if (form.value.type === "rss") {
        // validate-credentials is credentials-only; feed URLs live in settings.
        creds.feed_urls = form.value.config.settings.feed_urls;
      }
      await validateCredentials(form.value.type, creds);
    }
    testResult.value = "success";
    MessagePlugin.success(t("datasource.testSuccess"));
  } catch (e: any) {
    testResult.value = "error";
    testErrorMsg.value = e?.message || e?.error || "";
    MessagePlugin.error(t("datasource.testFailed"));
  }
  testing.value = false;
}

// --- Load resources ---
async function loadResources() {
  loadingResources.value = true;
  try {
    if (!tempDsId.value) {
      const res = await createDataSource({
        ...form.value,
        knowledge_base_id: props.kbId,
        status: "paused",
      } as any);
      const created = res?.data || res;
      tempDsId.value = created.id;
    } else if (!isEdit.value) {
      await updateDataSource(tempDsId.value, {
        ...form.value,
        knowledge_base_id: props.kbId,
      } as any);
    }

    const res = await listResources(tempDsId.value);
    resources.value = res?.data || res || [];
    // Any parent that already arrived with children (connectors returning the
    // full tree, e.g. Notion) needs no further lazy fetch.
    const parentsWithChildren = new Set<string>();
    for (const r of resources.value) {
      if (r.parent_id) parentsWithChildren.add(r.parent_id);
    }
    loadedChildrenIds.value = parentsWithChildren;
    loadingChildrenIds.value = new Set<string>();
    // If any resource already has a parent, the connector returned the whole tree
    // up front, so per-node lazy fetching is unnecessary.
    treeFullyLoaded.value = parentsWithChildren.size > 0;
    // Auto-expand top-level nodes whose children are already loaded; lazy nodes
    // (children not yet fetched) stay collapsed until the user expands them.
    expandedResourceIds.value = new Set(
      resources.value
        .filter((r) => !r.parent_id && r.has_children && parentsWithChildren.has(r.external_id))
        .map((r) => r.external_id),
    );
    // When editing a lazily-loaded source, reveal pre-existing selections that
    // live below the (not-yet-loaded) tree so they are visible and checked.
    if (isEdit.value && !treeFullyLoaded.value) {
      const loaded = new Set(resources.value.map((r) => r.external_id));
      const hidden = selectedResourceIds.value.filter((id) => !loaded.has(id));
      if (hidden.length > 0) void revealExistingSelections(hidden);
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.resourceLoadFailed"));
  }
  loadingResources.value = false;
}

// revealExistingSelections asks the backend which ancestors must be expanded to
// surface the current (possibly deeply nested) selection, then loads each level
// so the saved selection becomes visible and correctly checked in the tree.
async function revealExistingSelections(hiddenIds: string[]) {
  if (!tempDsId.value || hiddenIds.length === 0) return;
  try {
    const res = await resolveResourceAncestors(tempDsId.value, hiddenIds);
    const ancestors: string[] = res?.data?.ancestors || res?.ancestors || [];
    if (ancestors.length === 0) return;
    const expanded = new Set(expandedResourceIds.value);
    for (const id of ancestors) expanded.add(id);
    expandedResourceIds.value = expanded;
    // Load each ancestor level (children include the next ancestor / the
    // selection itself); calls are independent and dedup on merge.
    await Promise.all(ancestors.map((id) => ensureChildrenLoaded(id)));
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.resourceLoadFailed"));
  }
}

function getDescendantIds(id: string): string[] {
  const ids: string[] = [];
  const children = childrenMap.value.get(id) || [];
  for (const c of children) {
    ids.push(c.external_id);
    ids.push(...getDescendantIds(c.external_id));
  }
  return ids;
}

function getAncestorChain(id: string): string[] {
  const chain = [id];
  for (let p = parentMap.value.get(id); p; p = parentMap.value.get(p)) {
    chain.push(p);
  }
  return chain;
}

function isCovered(id: string, cover: Set<string>): boolean {
  for (let cur: string | undefined = id; cur; cur = parentMap.value.get(cur)) {
    if (cover.has(cur)) return true;
  }
  return false;
}

function checkResource(id: string, cover: Set<string>) {
  if (isCovered(id, cover)) return;
  const descendants = new Set(getDescendantIds(id));
  for (const d of [...cover]) {
    if (descendants.has(d)) cover.delete(d);
  }
  cover.add(id);
}

// Removes id from the cover set. If id is covered transitively (an ancestor is
// in the cover set), the ancestor is replaced with explicit entries for each
// sibling along the path so the rest of the subtree stays selected.
function uncheckResource(id: string, cover: Set<string>) {
  const chain = getAncestorChain(id); // [id, parent, ..., root]
  let highestIdx = -1;
  for (let i = chain.length - 1; i >= 0; i--) {
    if (cover.has(chain[i])) {
      highestIdx = i;
      break;
    }
  }
  if (highestIdx > 0) {
    cover.delete(chain[highestIdx]);
    for (let i = highestIdx; i > 0; i--) {
      const parent = chain[i];
      const next = chain[i - 1];
      for (const sib of childrenMap.value.get(parent) || []) {
        if (sib.external_id !== next) cover.add(sib.external_id);
      }
    }
  }
  cover.delete(id);
  const descendants = new Set(getDescendantIds(id));
  for (const d of [...cover]) {
    if (descendants.has(d)) cover.delete(d);
  }
}

function toggleResource(id: string) {
  const cover = new Set(selectedResourceIds.value);
  if ((checkStates.value.get(id) || "unchecked") === "unchecked") {
    checkResource(id, cover);
  } else {
    uncheckResource(id, cover);
  }
  selectedResourceIds.value = [...cover];
}

function validateRssFeedUrls(): boolean {
  if (form.value.type !== "rss") return true;
  if (!String(form.value.config.settings.feed_urls || "").trim()) {
    MessagePlugin.warning(`${t("datasource.field.feedUrls")} ${t("datasource.isRequired")}`);
    return false;
  }
  return true;
}

function validateStep1Fields(): boolean {
  syncRssAuthHeadersToCredentials();
  if (!validateRssFeedUrls()) return false;
  if (isEdit.value && credentialsConfigured.value && !replaceCredentialsMode.value) {
    return true;
  }

  const fields = currentDef.value?.fields || [];
  for (const f of fields) {
    if (f.optional || f.fieldType === "custom_headers") continue;
    if (!form.value.config.credentials[f.key]) {
      MessagePlugin.warning(`${t(f.labelKey)} ${t("datasource.isRequired")}`);
      return false;
    }
  }
  return true;
}

async function nextStep() {
  if (step.value === 1) {
    if (!validateStep1Fields()) return;
    if (needsConnectionTest() && testResult.value !== "success") {
      await testConnection();
      if ((testResult.value as string) !== "success") return;
    }
  }
  if (step.value === 2 && isDriveConnector(form.value.type)) {
    // folder_token 是 Drive 连接器的必填项：为空就地标错并留在本步,
    // 不允许带着空 token 进入同步策略。
    if (!driveFolderToken.value.trim()) {
      driveFolderTokenError.value = t("datasource.drive.folderTokenRequired");
      return;
    }
    driveFolderTokenError.value = "";
  }
  if (step.value === 2 && isGitLabConnector(form.value.type)) {
    syncGitLabProjectsToSettings();
    if (!gitlabProjects.value.some((project) => project.project_id.trim())) {
      MessagePlugin.warning(t("datasource.gitlab.projectRequired"));
      return;
    }
  }
  step.value++;
  if (step.value === 2) {
    // Drive connectors need a user-supplied folder_token before listing.
    // In edit mode with a saved folder_token, auto-load so the saved tree
    // (and any pre-existing selections) are revealed without an extra click.
    // In create mode (no folder_token yet), just show the placeholder.
    if (isDriveConnector(form.value.type)) {
      if (!driveRootLoaded.value && driveFolderToken.value.trim()) {
        void loadDriveRoot();
      }
      return;
    }
    if (isGitLabConnector(form.value.type)) return;
    loadResources();
  }
}

function prevStep() {
  step.value--;
}

// Build the config payload for Create / Update requests.
//
// Create mode: credentials flow inline so the initial data source row
// already carries them.
//
// Edit mode: credentials NEVER flow through the main PUT — they go via the
// /credentials subresource, committed before the main submit (see
// commitCredentialsIfNeeded). Sending an empty map keeps the backend
// validator happy.
function buildConfigPayload(): Record<string, unknown> {
  syncGitLabProjectsToSettings();
  return {
    credentials: isEdit.value ? {} : { ...form.value.config.credentials },
    resource_ids: form.value.config.resource_ids,
    settings: form.value.config.settings,
  };
}

// In edit mode, when the user opted in to Replace credentials and typed at
// least one value, commit it to /credentials before the main PUT. Aborts
// the whole submit on failure so we don't leave the row partially saved.
async function commitCredentialsIfNeeded(dsId: string): Promise<boolean> {
  if (!isEdit.value || !replaceCredentialsMode.value) return true;
  syncRssAuthHeadersToCredentials();
  const filled = Object.entries(form.value.config.credentials).filter(([, v]) =>
    typeof v === "string" ? v !== "" : v != null,
  );
  if (filled.length === 0) return true;
  try {
    await putDataSourceCredentials(dsId, Object.fromEntries(filled));
    credentialsConfigured.value = true;
    replaceCredentialsMode.value = false;
    form.value.config.credentials = {};
    rssAuthHeaders.value = [];
    return true;
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("credential.saveFailed"));
    return false;
  }
}

// --- Final submit ---
async function handleSubmit() {
  form.value.config.resource_ids = [...selectedResourceIds.value, ...manualWikiDocs.value.map((d) => d.id)];
  submitting.value = true;
  try {
    let dataSourceId = tempDsId.value;

    if (tempDsId.value) {
      // Commit credential replacement BEFORE the main PUT so a validation
      // failure on credentials doesn't leave us with an updated row that
      // still points at the old broken token.
      const credsOk = await commitCredentialsIfNeeded(tempDsId.value);
      if (!credsOk) {
        submitting.value = false;
        return;
      }
      await updateDataSource(tempDsId.value, {
        ...form.value,
        config: buildConfigPayload(),
        knowledge_base_id: props.kbId,
        status: "active",
      } as any);
    } else {
      const res = await createDataSource({
        ...form.value,
        config: buildConfigPayload(),
        knowledge_base_id: props.kbId,
        status: "active",
      } as any);
      const created = res?.data || res;
      dataSourceId = created.id;
      tempDsId.value = created.id;
    }

    if (isEdit.value) {
      MessagePlugin.warning(t("datasource.updateSuccessSyncHint"));
    } else {
      try {
        await triggerSync(dataSourceId);
        MessagePlugin.success(t("datasource.createAndSyncSuccess"));
      } catch (e: any) {
        MessagePlugin.warning(e?.message || e?.error || t("datasource.createButSyncFailed"));
      }
    }

    emit("saved");
    // Clear before close — otherwise the visible watcher treats the just-saved
    // row as an abandoned temp draft and DELETEs it (loadResources creates the
    // row early at step 2 with tempDsId).
    tempDsId.value = "";
    visible.value = false;
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.saveFailed"));
  }
  submitting.value = false;
}

function handleClose() {
  visible.value = false;
}

async function handleDrawerConfirm() {
  if (step.value === 1 || step.value === 2) {
    await nextStep();
  } else if (step.value === 3) {
    handleSubmit();
  }
}

const selectedResourceCount = computed(() => {
  let count = 0;
  for (const state of checkStates.value.values()) {
    if (state === "checked") count++;
  }
  return count;
});

const hasExpandableNodes = computed(() => resources.value.some((r) => r.has_children));

function resourceIconName(r: Resource): string {
  if (r.has_children) return "folder";
  switch (r.type) {
    case "wiki_space":
      return "root-list";
    case "book":
      return "book";
    case "doc_category":
      return "folder-open";
    default:
      return "file";
  }
}

function expandAllNodes() {
  const expandable = resources.value.filter((r) => r.has_children);
  expandedResourceIds.value = new Set(expandable.map((r) => r.external_id));
  // Lazily load children of any expanded node that hasn't been fetched yet.
  for (const r of expandable) {
    void ensureChildrenLoaded(r.external_id);
  }
}

function collapseAllNodes() {
  expandedResourceIds.value = new Set();
}

const resourceTypeLabelMap: Record<string, string> = {
  wiki_space: "datasource.resourceType.wikiSpace",
  doc_category: "datasource.resourceType.docCategory",
  book: "datasource.resourceType.book",
};

function resourceTypeLabel(type: string): string {
  const key = resourceTypeLabelMap[type];
  if (key) return t(key);
  return "";
}

function shouldShowResourceType(type: string): boolean {
  return !!resourceTypeLabelMap[type];
}

function resourceRowState(id: string): CheckState {
  return checkStates.value.get(id) || "unchecked";
}

const stepTitles = computed(() => [
  t("datasource.step.selectType"),
  t("datasource.step.credentials"),
  t("datasource.step.resources"),
  t("datasource.step.strategy"),
]);

const drawerTitle = computed(() => (isEdit.value ? t("datasource.editTitle") : t("datasource.createTitle")));

const drawerDescription = computed(() => stepTitles.value[step.value] ?? "");

const drawerConfirmText = computed(() => {
  if (step.value === 3) {
    return isEdit.value ? t("datasource.save") : t("datasource.createAndSync");
  }
  if (step.value >= 1) return t("datasource.next");
  return t("common.save");
});
// Class strings shared by several fields of the drawer, from the old
// .form-label / .form-desc / .option-pill rules. Kept here rather than
// repeated so the fields cannot drift apart.
// The flat variant is for labels the old CSS set to margin 0 (beside a switch,
// in the wiki-doc box, in the custom-headers header row).
const formLabelFlatClass = "text-foreground block text-[13px] leading-[1.4] font-medium";
const formLabelClass = `${formLabelFlatClass} mb-1.5`;
const requiredMarkClass =
  "before:text-destructive before:mr-1 before:leading-none before:font-medium before:content-['*']";
const formDescClass = "text-placeholder m-0 mt-1 text-xs leading-normal";
const resourceHintClass = "text-placeholder m-0 -mt-2 text-xs leading-normal";
// t-button variant="text" size="small" inside the credential field.
const credentialActionClass = "h-6 rounded px-2 text-xs";
const credentialDangerClass = "text-destructive hover:text-destructive hover:bg-destructive/10";
const treeActionClass =
  "text-placeholder hover:text-muted-foreground focus-visible:text-muted-foreground text-xs transition-colors duration-[120ms] focus-visible:outline-none";
const optionGroupClass =
  "border-border bg-muted inline-flex w-fit max-w-full items-center gap-1 rounded-lg border p-[3px]";
const optionPillClass =
  "inline-flex min-h-7 items-center justify-center rounded-md border px-2.5 py-1 text-xs leading-[1.3] whitespace-nowrap transition-[background,color,border-color] duration-150 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--td-brand-color)]";
const optionPillActiveClass = "border-border bg-card text-foreground shadow-[0_1px_2px_rgba(15,23,42,0.05)]";
const optionPillIdleClass = "text-muted-foreground hover:text-foreground border-transparent bg-transparent";
</script>

<template>
  <!--
    This drawer opens from the KB editor modal (z-[1000]); SettingDrawer sits
    on the old TDesign drawer layer (z-[2500]) and forwards the class below
    onto the panel.
    `datasource-editor-drawer--<type>` and `ds-fixed-step` are hook classes.
  -->
  <SettingDrawer
    v-model:visible="visible"
    :title="drawerTitle"
    :description="drawerDescription"
    :class="[
      form.type ? `datasource-editor-drawer datasource-editor-drawer--${form.type}` : 'datasource-editor-drawer',
      {
        'ds-fixed-step [&_[data-setting-drawer-body]]:overflow-hidden': step === 2 && !isGitLabConnector(form.type),
      },
    ]"
    :hide-footer="step === 0"
    :confirm-text="drawerConfirmText"
    :confirm-loading="submitting || (step === 1 && testing)"
    storage-key="setting-drawer:width:datasource-editor"
    width="640px"
    @confirm="handleDrawerConfirm"
    @cancel="handleClose"
  >
    <template v-if="form.type && getDatasourceIconUrl(form.type)" #headerIcon>
      <img :src="getDatasourceIconUrl(form.type)" :alt="form.type" class="datasource-header-icon__img" />
    </template>

    <template v-if="step === 1" #footer-left>
      <Button v-if="!isEdit" variant="outline" @click="step = 0">
        {{ t("datasource.back") }}
      </Button>
      <Button variant="outline" :disabled="testing" @click="testConnection">
        <Loader2Icon v-if="testing" class="animate-spin" />
        <CircleCheckIcon v-else-if="testResult === 'success'" class="text-primary size-4 shrink-0" />
        <CircleXIcon v-else-if="testResult === 'error'" class="text-destructive size-4 shrink-0" />
        {{ testing ? t("model.editor.testing") : t("datasource.testConnection") }}
      </Button>
      <span
        v-if="testResult"
        class="min-w-0 flex-1 truncate text-xs leading-[1.4]"
        :class="testResult === 'success' ? 'text-[var(--td-brand-color-active)]' : 'text-destructive'"
        :title="testResult === 'error' ? testErrorMsg : t('datasource.connected')"
      >
        {{ testResult === "success" ? t("datasource.connected") : testErrorMsg || t("datasource.connectionFailed") }}
      </span>
    </template>

    <template v-else-if="step === 2 || step === 3" #footer-left>
      <Button variant="outline" @click="prevStep">
        {{ t("datasource.back") }}
      </Button>
    </template>

    <!-- Step indicator -->
    <div class="border-border mb-5 flex gap-2 border-b pb-3.5">
      <div
        v-for="(title, i) in stepTitles"
        :key="i"
        class="flex min-w-0 flex-1 items-center gap-2 text-[13px]"
        :class="{
          'text-primary font-medium': step === i,
          'text-muted-foreground font-medium': step > i,
          'text-placeholder': step < i,
        }"
      >
        <span
          class="flex size-[22px] shrink-0 items-center justify-center rounded-full border text-xs font-semibold"
          :class="{
            'border-primary bg-primary text-white': step === i,
            'bg-primary/12 text-primary border-transparent': step > i,
            'border-border text-placeholder bg-transparent': step < i,
          }"
        >
          <CheckIcon v-if="step > i" class="size-3.5" />
          <template v-else>{{ i + 1 }}</template>
        </span>
        <span class="min-w-0 truncate">{{ title }}</span>
      </div>
    </div>

    <!-- Step 0: Select connector type -->
    <section v-if="step === 0" class="setting-drawer__section">
      <h4 class="setting-drawer__section-title">{{ t("datasource.step.selectType") }}</h4>
      <div class="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2.5">
        <button
          v-for="def in connectorDefs"
          :key="def.type"
          type="button"
          data-slot="ds-type-card"
          class="border-border bg-card rounded-[10px] border p-3.5 text-left transition-[border-color,box-shadow] duration-[180ms] focus-visible:border-[var(--td-brand-color-3,var(--td-brand-color))] focus-visible:shadow-[0_4px_14px_rgba(15,23,42,0.06)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--td-brand-color)]"
          :class="
            def.available
              ? 'cursor-pointer hover:border-[var(--td-brand-color-3,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)]'
              : 'cursor-not-allowed opacity-50'
          "
          :disabled="!def.available"
          @click="selectType(def)"
        >
          <div class="mb-1.5 flex items-center gap-2">
            <DataSourceTypeIcon :type="def.type" :size="20" />
            <span class="text-[13px] font-semibold">{{ t(`datasource.connector.${def.type}`) }}</span>
            <span
              v-if="!def.available"
              class="text-placeholder rounded-[3px] bg-[var(--td-bg-color-component)] px-1.5 py-px text-[10px]"
              >{{ t("datasource.comingSoon") }}</span
            >
          </div>
          <div class="text-muted-foreground text-[11px] leading-normal">
            {{ t(`datasource.connectorDesc.${def.type}`) }}
          </div>
        </button>
      </div>
    </section>

    <!-- Step 1: Credentials -->
    <template v-if="step === 1">
      <div v-if="currentDef && currentDef.requiredPermissions.length > 0" class="mb-1 flex flex-col gap-1">
        <button
          type="button"
          data-slot="ds-setup-guide-toggle"
          class="text-muted-foreground hover:text-foreground focus-visible:text-foreground flex w-full items-center gap-2 text-left text-[13px] leading-normal transition-colors duration-[120ms] focus-visible:outline-none"
          :aria-expanded="prereqExpanded"
          @click="prereqExpanded = !prereqExpanded"
        >
          <InfoCircleFilledIcon class="text-placeholder size-[15px] shrink-0" />
          <span class="min-w-0 flex-1">
            {{ t(`datasource.prereqBarText_${form.type}`, t("datasource.prereqBarText")) }}
          </span>
          <component
            :is="prereqExpanded ? ChevronUpIcon : ChevronDownIcon"
            class="text-placeholder size-3.5 shrink-0"
          />
        </button>
        <div v-if="prereqExpanded" class="pl-[23px]">
          <ol class="m-0 mt-2.5 flex flex-col gap-2.5 pl-[18px]">
            <li class="text-foreground text-[13px] leading-normal">
              <span class="mb-0.5 block font-medium">{{
                t(`datasource.prereqStep1Brief_${form.type}`, t("datasource.prereqBotBrief"))
              }}</span>
              <span class="text-muted-foreground block">{{
                t(`datasource.prereqStep1Desc_${form.type}`, t("datasource.prereqBotDesc"))
              }}</span>
            </li>
            <li class="text-foreground text-[13px] leading-normal">
              <span class="mb-0.5 block font-medium">{{
                t(`datasource.prereqStep2Brief_${form.type}`, t("datasource.prereqPermBrief"))
              }}</span>
              <span class="text-muted-foreground block">
                <template v-if="!t(`datasource.prereqStep2Desc_${form.type}`)">
                  <code
                    v-for="perm in currentDef.requiredPermissions"
                    :key="perm"
                    class="bg-card text-muted-foreground my-0.5 mr-1 ml-0 inline-block rounded-[3px] px-[5px] py-px [font-family:var(--app-font-family-mono,ui-monospace,monospace)] text-[11px]"
                    >{{ perm }}</code
                  >
                </template>
                <template v-else>{{ t(`datasource.prereqStep2Desc_${form.type}`) }}</template>
              </span>
            </li>
            <li class="text-foreground text-[13px] leading-normal">
              <span class="mb-0.5 block font-medium">{{
                t(`datasource.prereqStep3Brief_${form.type}`, t("datasource.prereqMemberBrief"))
              }}</span>
              <span class="text-muted-foreground block">{{
                t(`datasource.prereqStep3Desc_${form.type}`, t("datasource.prereqMemberDesc"))
              }}</span>
            </li>
          </ol>
          <!-- `doc-link` is the global external-link style (assets/theme/theme.css). -->
          <a
            v-if="currentDef.permissionPageUrl"
            :href="currentDef.permissionPageUrl"
            target="_blank"
            rel="noopener"
            class="doc-link mt-2.5 text-[13px]"
          >
            {{ t(`datasource.prereqOpenConsole_${form.type}`, t("datasource.prereqOpenConsole")) }}
            <LinkIcon class="link-icon size-[1em]" />
          </a>
        </div>
      </div>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ t("datasource.sectionBasic") }}</h4>

        <div
          v-if="currentDef?.docUrl"
          class="text-muted-foreground flex flex-wrap items-center gap-2 text-[13px] leading-normal"
        >
          <InfoCircleFilledIcon class="text-placeholder size-[15px] shrink-0" />
          <span class="min-w-0 flex-auto">{{ t("datasource.docHint") }}</span>
          <a
            :href="currentDef.docUrl"
            target="_blank"
            rel="noopener"
            class="doc-link gap-0.5 text-[13px] font-medium whitespace-nowrap transition-colors duration-150"
          >
            {{ t("datasource.openDoc") }}
            <LinkIcon class="link-icon size-[1em]" />
          </a>
        </div>

        <div>
          <label :class="[formLabelClass, requiredMarkClass]">{{ t("datasource.nameLabel") }}</label>
          <Input v-model="form.name" :placeholder="t('datasource.namePlaceholder')" />
        </div>
      </section>

      <section v-if="isFeishuFamily(form.type)" class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ t("datasource.videoSync.title") }}</h4>
        <div>
          <label class="flex items-center gap-2">
            <Switch v-model="videoSyncEnabled" size="sm" />
            <span :class="formLabelFlatClass">{{ t("datasource.videoSync.enable") }}</span>
          </label>
          <p :class="formDescClass">{{ t("datasource.videoSync.enableHint") }}</p>
        </div>
        <div v-if="videoSyncEnabled">
          <label :class="formLabelClass">{{ t("datasource.videoSync.maxSize") }}</label>
          <!-- t-input-number with suffix="MB": the unit sits inside the field. -->
          <div class="relative w-[220px]">
            <Input v-model.number="videoMaxMB" type="number" :min="1" :max="10240" :step="256" class="pr-10" />
            <span class="text-placeholder pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 text-xs"
              >MB</span
            >
          </div>
          <p :class="formDescClass">{{ t("datasource.videoSync.maxSizeHint") }}</p>
        </div>
      </section>

      <section v-if="isFeishuFamily(form.type)" class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ t("datasource.linkedPages.title") }}</h4>
        <div>
          <label class="flex items-center gap-2">
            <Switch v-model="linkedPagesEnabled" size="sm" />
            <span :class="formLabelFlatClass">{{ t("datasource.linkedPages.enable") }}</span>
          </label>
          <p :class="formDescClass">{{ t("datasource.linkedPages.enableHint") }}</p>
        </div>
      </section>

      <section v-if="form.type === 'rss'" class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ t("datasource.field.feedUrls") }}</h4>
        <div>
          <label :class="[formLabelClass, requiredMarkClass]">{{ t("datasource.field.feedUrls") }}</label>
          <!-- autosize 2..6 rows -->
          <Textarea
            v-model="form.config.settings.feed_urls"
            placeholder="https://example.com/feed.xml"
            class="max-h-[136px] min-h-14"
            autocomplete="off"
            spellcheck="false"
          />
          <p :class="formDescClass">{{ t("datasource.field.feedUrlsHint") }}</p>
        </div>
      </section>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ t("datasource.credentialsLabel") }}</h4>

        <div v-if="isEdit && credentialsConfigured && !replaceCredentialsMode">
          <div
            class="flex h-8 items-center gap-2 rounded-md border py-0 pr-1 pl-3 text-[13px] transition-[border-color,background-color] duration-150"
            :class="
              pendingRemoveCredentials
                ? 'border-[var(--td-error-color-focus)] bg-[var(--td-error-color-light)]'
                : 'border-border bg-card hover:border-[var(--td-brand-color-hover,var(--td-brand-color))]'
            "
            :title="pendingRemoveCredentials ? '' : t('credential.configured')"
          >
            <template v-if="pendingRemoveCredentials">
              <CircleAlertIcon class="text-destructive size-4 shrink-0" />
              <span class="text-destructive min-w-0 flex-1 truncate font-medium">{{
                t("credential.confirmRemovePrompt")
              }}</span>
              <div class="flex shrink-0 items-center gap-0.5">
                <Button variant="ghost" :class="credentialActionClass" @click="cancelPendingRemoveCredentials">
                  {{ t("common.cancel") }}
                </Button>
                <span class="bg-border mx-0.5 h-3.5 w-px" />
                <Button
                  variant="ghost"
                  :class="[credentialActionClass, credentialDangerClass]"
                  :disabled="removingCredentials"
                  @click="confirmRemoveCredentials"
                >
                  <Loader2Icon v-if="removingCredentials" class="size-3 animate-spin" />
                  {{ t("credential.confirmRemove") }}
                </Button>
              </div>
            </template>
            <template v-else>
              <CircleCheckIcon class="text-success size-4 shrink-0" />
              <span class="text-foreground min-w-0 flex-1 truncate">{{ t("credential.configured") }}</span>
              <div class="flex shrink-0 items-center gap-0.5">
                <Button variant="ghost" :class="credentialActionClass" @click="enterReplaceCredentials">
                  {{ t("credential.update") }}
                </Button>
                <span class="bg-border mx-0.5 h-3.5 w-px" />
                <Button
                  variant="ghost"
                  :class="[credentialActionClass, credentialDangerClass]"
                  @click="requestRemoveCredentials"
                >
                  {{ t("credential.remove") }}
                </Button>
              </div>
            </template>
          </div>
        </div>

        <div v-else-if="isEdit && !credentialsConfigured && !replaceCredentialsMode">
          <div
            class="border-border bg-card hover:bg-accent flex h-8 cursor-pointer items-center gap-2 rounded-md border py-0 pr-1 pl-3 text-[13px] transition-[border-color,background-color] duration-150 hover:border-[var(--td-brand-color-hover,var(--td-brand-color))]"
            @click="enterReplaceCredentials"
          >
            <LockIcon class="text-placeholder size-4 shrink-0" />
            <span class="text-placeholder min-w-0 flex-1 truncate">{{ t("credential.unconfigured") }}</span>
            <div class="flex shrink-0 items-center gap-0.5">
              <Button
                variant="ghost"
                :class="[credentialActionClass, 'text-primary hover:text-primary']"
                @click.stop="enterReplaceCredentials"
              >
                {{ t("credential.configure") }}
              </Button>
            </div>
          </div>
        </div>

        <template v-else-if="credentialsInputVisible">
          <div v-for="field in currentDef?.fields || []" :key="field.key">
            <template v-if="field.fieldType === 'custom_headers'">
              <div class="mb-1.5 flex items-center justify-between">
                <label :class="formLabelFlatClass">{{ t(field.labelKey) }}</label>
                <Button
                  variant="ghost"
                  size="xs"
                  class="text-primary hover:text-primary hover:bg-accent"
                  @click="addRssAuthHeader"
                >
                  <PlusIcon />
                  {{ t("model.editor.customHeadersAdd") }}
                </Button>
              </div>
              <p v-if="field.hintKey" class="text-placeholder m-0 mb-2.5 text-xs leading-normal">
                {{ t(field.hintKey) }}
              </p>
              <div v-if="rssAuthHeaders.length > 0" class="flex flex-col gap-2">
                <div v-for="(item, idx) in rssAuthHeaders" :key="idx" class="flex items-center gap-2">
                  <Input
                    v-model="item.key"
                    :placeholder="t('model.editor.customHeadersKeyPlaceholder')"
                    class="flex-[0_0_38%]"
                    autocomplete="off"
                    spellcheck="false"
                  />
                  <Input
                    v-model="item.value"
                    :placeholder="t('model.editor.customHeadersValuePlaceholder')"
                    class="flex-1"
                    autocomplete="off"
                    spellcheck="false"
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    class="text-placeholder shrink-0 rounded-md transition-all duration-[180ms] hover:bg-[var(--td-error-color-light)] hover:text-[var(--td-error-color)]"
                    :aria-label="t('common.delete')"
                    @click="removeRssAuthHeader(idx)"
                  >
                    <XIcon />
                  </Button>
                </div>
              </div>
            </template>
            <template v-else>
              <label :class="[formLabelClass, { [requiredMarkClass]: !field.optional }]">
                {{ t(field.labelKey) }}
              </label>
              <Textarea
                v-if="field.multiline"
                v-model="form.config.credentials[field.key]"
                :placeholder="field.placeholder || t('credential.inputPlaceholder')"
                class="max-h-[136px] min-h-14"
                autocomplete="off"
                spellcheck="false"
              />
              <div v-else class="relative">
                <Input
                  v-model="form.config.credentials[field.key]"
                  :placeholder="field.placeholder || t('credential.inputPlaceholder')"
                  :type="field.secret ? 'password' : 'text'"
                  :class="{ 'pl-8': field.secret }"
                  autocomplete="off"
                  spellcheck="false"
                />
                <LockIcon
                  v-if="field.secret"
                  class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2"
                />
              </div>
              <p v-if="field.hintKey" :class="formDescClass">{{ t(field.hintKey) }}</p>
            </template>
          </div>
          <div v-if="isEdit && replaceCredentialsMode" class="flex justify-end">
            <Button variant="ghost" size="sm" class="h-7 px-3 text-xs" @click="cancelReplaceCredentials">
              {{ t("common.cancel") }}
            </Button>
          </div>
        </template>
      </section>
    </template>

    <!-- Step 2: Select resources -->
    <!--
      Outside GitLab this step does not scroll as a whole (see ds-fixed-step on
      the drawer): the token inputs stay put and the area below — placeholder,
      loading, empty state or tree — fills the remaining height, with the tree
      list scrolling inside it. The section's gap needs `!` to beat
      SettingDrawer's unlayered section rule.
    -->
    <section
      v-if="step === 2"
      class="setting-drawer__section ds-resource-section gap-2.5!"
      :class="{ 'min-h-0 flex-1 overflow-hidden': !isGitLabConnector(form.type) }"
    >
      <template v-if="isGitLabConnector(form.type)">
        <h4 class="setting-drawer__section-title">{{ t("datasource.gitlab.projects") }}</h4>
        <p :class="resourceHintClass">{{ t("datasource.gitlab.projectsHint") }}</p>
        <div class="mb-5 grid gap-3">
          <div
            v-for="(project, index) in gitlabProjects"
            :key="index"
            class="border-border bg-card grid gap-2 rounded-md border p-3"
          >
            <div class="flex items-center justify-between">
              <strong>{{ t("datasource.gitlab.project") }} {{ index + 1 }}</strong>
              <Button
                variant="ghost"
                size="icon-xs"
                class="text-destructive hover:text-destructive hover:bg-destructive/10"
                :aria-label="t('common.delete')"
                @click="removeGitLabProject(index)"
              >
                <Trash2Icon />
              </Button>
            </div>
            <label :class="[formLabelClass, requiredMarkClass]">{{ t("datasource.gitlab.projectId") }}</label>
            <Input v-model="project.project_id" :placeholder="t('datasource.gitlab.projectIdPlaceholder')" />
            <label :class="formLabelClass">{{ t("datasource.gitlab.ref") }}</label>
            <Input v-model="project.ref" :placeholder="t('datasource.gitlab.refPlaceholder')" />
            <label :class="formLabelClass">{{ t("datasource.gitlab.paths") }}</label>
            <!-- autosize 2..5 rows -->
            <Textarea
              v-model="project.pathsText"
              :placeholder="t('datasource.gitlab.pathsPlaceholder')"
              class="max-h-[116px] min-h-14"
            />
          </div>
          <Button variant="outline" @click="addGitLabProject">
            <PlusIcon />{{ t("datasource.gitlab.addProject") }}
          </Button>
        </div>
      </template>
      <template v-else>
        <h4 class="setting-drawer__section-title">{{ t("datasource.step.resources") }}</h4>
        <p :class="resourceHintClass">{{ t("datasource.resourceHint") }}</p>

        <!-- Wiki doc-by-URL input: personal document libraries are hidden wiki
           spaces that never appear in the space tree below, so docs there can
           only be added by pasting their link. -->
        <div
          v-if="isWikiConnector(form.type)"
          class="border-border bg-card mb-3 flex flex-col gap-2 rounded-md border p-3"
        >
          <label :class="formLabelFlatClass">{{ t("datasource.wikiDoc.label") }}</label>
          <!--
            pb-5 reserves the room the tip under the input takes: like
            t-input's `tips` it is positioned below the field, out of flow,
            so the button stays aligned with the input.
          -->
          <div class="flex items-center gap-2 pb-5">
            <div class="relative flex-1">
              <Input
                v-model="wikiDocInput"
                :placeholder="t('datasource.wikiDoc.placeholder')"
                class="pr-8"
                :class="{ 'border-destructive': wikiDocError }"
                :aria-invalid="wikiDocError ? true : undefined"
                @keydown.enter="addWikiDocByUrl"
                @update:model-value="wikiDocError = ''"
              />
              <button
                v-if="wikiDocInput"
                type="button"
                data-slot="input-clear"
                class="text-placeholder hover:text-muted-foreground absolute top-4 right-2 -translate-y-1/2"
                :aria-label="t('common.clear')"
                @click="
                  wikiDocInput = '';
                  wikiDocError = '';
                "
              >
                <CircleXIcon class="size-3.5" />
              </button>
              <p
                class="absolute top-full left-0 m-0 mt-1 text-xs leading-normal"
                :class="wikiDocError ? 'text-destructive' : 'text-placeholder'"
              >
                {{ wikiDocError || t("datasource.wikiDoc.hint") }}
              </p>
            </div>
            <Button :disabled="addingWikiDoc" @click="addWikiDocByUrl">
              <Loader2Icon v-if="addingWikiDoc" class="animate-spin" />
              {{ t("datasource.wikiDoc.add") }}
            </Button>
          </div>
          <div v-if="manualWikiDocs.length" class="flex flex-wrap gap-2">
            <Badge v-for="(doc, index) in manualWikiDocs" :key="doc.id" class="bg-primary/10 text-primary gap-1 pr-1">
              {{ doc.name }}
              <button
                type="button"
                data-slot="tag-close"
                class="inline-flex items-center opacity-70 hover:opacity-100"
                :aria-label="t('common.remove')"
                @click="removeManualWikiDoc(index)"
              >
                <XIcon class="size-3" />
              </button>
            </Badge>
          </div>
        </div>

        <!-- Drive (云盘) root input: shown alongside the tree (not as a switch).
           The user supplies a folder_token (or a Drive folder URL) and clicks
           "load"; the tree below stays as a placeholder until load succeeds.
           Other connectors skip this and go straight to the tree. -->
        <div v-if="isDriveConnector(form.type)" class="border-border bg-card flex flex-col gap-2 rounded-md border p-3">
          <label
            class="text-foreground before:text-destructive flex items-center gap-1 text-[13px] font-medium before:leading-none before:font-medium before:content-['*']"
          >
            {{ t("datasource.drive.folderTokenLabel") }}
            <Tooltip>
              <TooltipTrigger as-child>
                <span
                  tabindex="0"
                  class="text-placeholder hover:text-muted-foreground inline-flex cursor-help"
                  :aria-label="t('datasource.drive.rootNotSupportedHint')"
                >
                  <CircleHelpIcon class="size-[15px]" />
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{{ t("datasource.drive.rootNotSupportedHint") }}</TooltipContent>
            </Tooltip>
          </label>
          <div class="flex items-center gap-2 pb-5">
            <div class="relative flex-1">
              <Input
                v-model="driveFolderToken"
                :placeholder="t('datasource.drive.folderTokenPlaceholder')"
                class="pr-8"
                :class="{ 'border-destructive': driveFolderTokenError }"
                :aria-invalid="driveFolderTokenError ? true : undefined"
                @keydown.enter="loadDriveRoot"
                @update:model-value="driveFolderTokenError = ''"
              />
              <button
                v-if="driveFolderToken"
                type="button"
                data-slot="input-clear"
                class="text-placeholder hover:text-muted-foreground absolute top-4 right-2 -translate-y-1/2"
                :aria-label="t('common.clear')"
                @click="
                  driveFolderToken = '';
                  driveFolderTokenError = '';
                "
              >
                <CircleXIcon class="size-3.5" />
              </button>
              <p
                class="absolute top-full left-0 m-0 mt-1 text-xs leading-normal"
                :class="driveFolderTokenError ? 'text-destructive' : 'text-placeholder'"
              >
                {{ driveFolderTokenError ? driveFolderTokenError : t("datasource.drive.shareHint") }}
              </p>
            </div>
            <Button :disabled="loadingResources" @click="loadDriveRoot">
              <Loader2Icon v-if="loadingResources" class="animate-spin" />
              {{ t("datasource.drive.load") }}
            </Button>
          </div>
        </div>

        <!-- Drive placeholder before the first load: the tree cannot render until
           a folder_token is supplied and loaded. Non-Drive connectors never hit
           this branch. -->
        <div
          v-if="isDriveConnector(form.type) && !driveRootLoaded && !loadingResources"
          class="border-border bg-background flex min-h-[120px] flex-1 flex-col items-center justify-center gap-1.5 rounded-md border border-dashed px-3 py-6 text-center"
        >
          <p class="text-foreground m-0 text-[13px] font-medium">{{ t("datasource.drive.placeholderTitle") }}</p>
          <p class="text-placeholder m-0 text-xs leading-normal">{{ t("datasource.drive.placeholderDesc") }}</p>
        </div>

        <div v-else-if="loadingResources" class="flex min-h-0 flex-1 items-center justify-center p-6">
          <Loader2Icon class="text-primary size-6 animate-spin" />
        </div>
        <div v-else-if="resources.length > 0" class="flex min-h-0 flex-1 flex-col gap-1.5">
          <div class="flex items-center justify-between gap-2.5">
            <span class="text-muted-foreground text-xs">
              {{ t("knowledgeBase.selectedCount", { count: selectedResourceCount }) }}
            </span>
            <div v-if="hasExpandableNodes" class="flex shrink-0 items-center gap-1.5">
              <button type="button" data-slot="tree-action" :class="treeActionClass" @click="expandAllNodes">
                {{ t("knowledgeStages.expandBranch") }}
              </button>
              <span class="text-xs text-[var(--td-text-color-disabled)] select-none" aria-hidden="true">·</span>
              <button type="button" data-slot="tree-action" :class="treeActionClass" @click="collapseAllNodes">
                {{ t("knowledgeStages.collapseBranch") }}
              </button>
            </div>
          </div>
          <!-- Inset panel: light grouped surface without an outer stroke. -->
          <div class="bg-muted min-h-0 flex-1 overflow-y-auto overscroll-contain rounded-lg px-1.5 py-1" role="tree">
            <div
              v-for="{ resource: r, depth } in visibleTree"
              :key="r.external_id"
              class="hover:bg-muted relative mb-0.5 grid min-h-[34px] cursor-pointer grid-cols-[16px_16px_16px_1fr] items-center gap-x-2 rounded-md py-[5px] pr-2 transition-[background] duration-[120ms] last:mb-0"
              :class="{
                'bg-muted':
                  resourceRowState(r.external_id) === 'checked' || resourceRowState(r.external_id) === 'indeterminate',
              }"
              :style="{ paddingLeft: `calc(8px + ${depth} * 14px)` }"
              role="treeitem"
              :aria-expanded="r.has_children ? expandedResourceIds.has(r.external_id) : undefined"
              @click="toggleResource(r.external_id)"
            >
              <button
                v-if="r.has_children"
                type="button"
                data-slot="tree-expand"
                class="text-placeholder hover:text-muted-foreground focus-visible:text-muted-foreground inline-flex size-4 shrink-0 items-center justify-center rounded transition-colors duration-[120ms] hover:bg-[color-mix(in_srgb,var(--td-text-color-placeholder)_12%,transparent)] focus-visible:bg-[color-mix(in_srgb,var(--td-text-color-placeholder)_12%,transparent)] focus-visible:outline-none"
                :aria-label="
                  expandedResourceIds.has(r.external_id)
                    ? t('knowledgeStages.collapseBranch')
                    : t('knowledgeStages.expandBranch')
                "
                @click.stop="toggleExpand(r.external_id)"
              >
                <Loader2Icon v-if="loadingChildrenIds.has(r.external_id)" class="size-3 animate-spin" />
                <component
                  :is="expandedResourceIds.has(r.external_id) ? ChevronDownIcon : ChevronRightIcon"
                  v-else
                  class="size-3"
                />
              </button>
              <span v-else class="size-4 shrink-0" aria-hidden="true" />
              <span
                class="box-border inline-flex size-4 shrink-0 items-center justify-center rounded-[3px] border-[1.5px] border-solid transition-[background,border-color] duration-[120ms]"
                :class="resourceRowState(r.external_id) === 'unchecked' ? 'border-border' : 'border-primary bg-primary'"
                aria-hidden="true"
              >
                <svg
                  v-if="resourceRowState(r.external_id) === 'checked'"
                  width="10"
                  height="10"
                  viewBox="0 0 12 12"
                  fill="none"
                >
                  <path
                    d="M10 3L4.5 8.5L2 6"
                    stroke="#fff"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  />
                </svg>
                <span
                  v-else-if="resourceRowState(r.external_id) === 'indeterminate'"
                  class="h-0.5 w-2 rounded-[1px] bg-white"
                />
              </span>
              <span
                class="text-muted-foreground inline-flex size-4 shrink-0 items-center justify-center"
                aria-hidden="true"
              >
                <component :is="dsIcon(resourceIconName(r))" class="size-4" />
              </span>
              <span class="flex min-w-0 items-center gap-1.5">
                <span
                  class="text-foreground min-w-0 truncate text-[13px] leading-[1.4]"
                  :title="r.name || t('datasource.untitled')"
                >
                  {{ r.name || t("datasource.untitled") }}
                </span>
                <span
                  v-if="shouldShowResourceType(r.type)"
                  class="text-placeholder shrink-0 rounded bg-[color-mix(in_srgb,var(--td-text-color-placeholder)_8%,transparent)] px-[5px] py-0.5 text-[10px] leading-none"
                  >{{ resourceTypeLabel(r.type) }}</span
                >
              </span>
            </div>
          </div>
        </div>
        <div v-else class="min-h-0 flex-1 py-6 text-center">
          <InfoCircleFilledIcon class="text-warning mx-auto mb-2 size-8" />
          <p class="text-foreground m-0 mb-1 text-sm font-semibold">{{ t("datasource.noResources") }}</p>
          <p class="text-muted-foreground m-0 mb-4 text-xs">
            {{ t(`datasource.noResourcesDesc_${form.type}`, t("datasource.noResourcesDesc")) }}
          </p>
          <div class="mx-auto mb-4 flex max-w-[440px] flex-col gap-2 text-left">
            <div v-for="n in 3" :key="n" class="text-foreground flex items-start gap-2 text-[13px] leading-normal">
              <span
                class="border-border bg-muted text-muted-foreground mt-px flex size-5 shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold"
                >{{ n }}</span
              >
              <span>{{ t(`datasource.guideStep${n}_${form.type}`, t(`datasource.guideStep${n}`)) }}</span>
            </div>
          </div>
          <div class="flex items-center justify-center gap-4">
            <button
              type="button"
              data-slot="ds-empty-retry"
              class="text-primary text-[13px] font-medium transition-colors duration-[120ms] hover:text-[var(--td-brand-color-active)] focus-visible:text-[var(--td-brand-color-active)] focus-visible:outline-none"
              @click="loadResources"
            >
              {{ t("datasource.retryLoadResources") }}
            </button>
            <a
              v-if="currentDef?.permissionDocUrl"
              :href="currentDef.permissionDocUrl"
              target="_blank"
              rel="noopener"
              class="doc-link"
            >
              {{ t("datasource.permissionDocLink") }}
              <LinkIcon class="link-icon size-[1em]" />
            </a>
          </div>
        </div>
      </template>
    </section>

    <!-- Step 3: Sync strategy -->
    <template v-if="step === 3">
      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ t("datasource.syncScheduleLabel") }}</h4>
        <Select v-model="form.sync_schedule">
          <SelectTrigger class="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="p in schedulePresets" :key="p.value" :value="p.value">{{ p.label }}</SelectItem>
          </SelectContent>
        </Select>
      </section>

      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ t("datasource.syncModeLabel") }}</h4>
        <div>
          <div :class="optionGroupClass" role="radiogroup" :aria-label="t('datasource.syncModeLabel')">
            <button
              v-for="mode in ['incremental', 'full'] as const"
              :key="mode"
              type="button"
              data-slot="option-pill"
              :class="[optionPillClass, form.sync_mode === mode ? optionPillActiveClass : optionPillIdleClass]"
              role="radio"
              :aria-checked="form.sync_mode === mode"
              @click="form.sync_mode = mode"
            >
              {{ t(`datasource.syncMode.${mode}`) }}
            </button>
          </div>
        </div>

        <div>
          <label :class="formLabelClass">{{ t("datasource.conflictLabel") }}</label>
          <div :class="optionGroupClass" role="radiogroup" :aria-label="t('datasource.conflictLabel')">
            <button
              v-for="strategy in ['overwrite', 'skip'] as const"
              :key="strategy"
              type="button"
              data-slot="option-pill"
              :class="[
                optionPillClass,
                form.conflict_strategy === strategy ? optionPillActiveClass : optionPillIdleClass,
              ]"
              role="radio"
              :aria-checked="form.conflict_strategy === strategy"
              @click="form.conflict_strategy = strategy"
            >
              {{ t(`datasource.conflict.${strategy}`) }}
            </button>
          </div>
          <p :class="formDescClass">{{ t("datasource.conflictHint") }}</p>
        </div>

        <div>
          <!-- A <label>, so the text toggles the box as t-checkbox's label did. -->
          <label class="inline-flex cursor-pointer items-center gap-2">
            <Checkbox
              :model-value="form.sync_deletions"
              @update:model-value="(v: boolean | 'indeterminate') => (form.sync_deletions = v === true)"
            />
            <span class="text-muted-foreground text-xs leading-normal">{{ t("datasource.syncDeletions") }}</span>
          </label>
        </div>
      </section>
    </template>
  </SettingDrawer>
</template>

<!--
  Not scoped, and CSS because neither element is ours to class: the header
  badge is rendered by SettingDrawer around the logo passed through its
  headerIcon slot, and it is only recoloured when a logo is in it — the same
  white badge as the list cards. `setting-drawer__header-icon` is the hook
  SettingDrawer keeps for exactly this.
-->
<style>
.datasource-editor-drawer .setting-drawer__header-icon:has(.datasource-header-icon__img) {
  background: var(--td-bg-color-container, #fff);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.datasource-header-icon__img {
  display: block;
  width: 24px;
  height: 24px;
  object-fit: contain;
}
</style>
