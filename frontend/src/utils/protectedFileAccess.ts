/**
 * 受保护文件（resource:// 引用）的访问上下文。
 *
 * 后端按访问主体拆分文件代理，鉴权模型互不相同：
 *   - `/files`                                   → 登录态 Bearer + X-Tenant-ID
 *   - `/api/v1/knowledge-bases/:id/files`        → 知识库访问权限（跨租户共享库）
 *   - `/api/v1/sessions/:id/messages/:mid/files` → 会话消息归属
 *
 * 选哪条代理由渲染组件声明的作用域决定，这里把"作用域 → 代理 URL"收敛成
 * 单一真相源，渲染组件只需声明作用域，不再各自拼 URL。
 */

// A stored file is referenced as resource://<handle>, and nothing else: the
// backend's own locators (local://, s3://) never leave the server, which also
// refuses them on every file proxy. The handle alphabet is URL-safe base64.
const RESOURCE_REF_RE = /^resource:\/\/[0-9A-Za-z_-]{22}$/;

const KB_FILE_PROXY_PATH_RE = /^\/api\/v1\/knowledge-bases\/[^/]+\/files$/;
const MESSAGE_FILE_PROXY_PATH_RE = /^\/api\/v1\/sessions\/[^/]+\/messages\/[^/]+\/files$/;

export type ProtectedFileAccessContext =
  /** 登录态用户：Bearer + 选中租户。 */
  | { mode: "tenant" }
  /** 知识库作用域：登录态用户读取共享库中归属其他租户的对象。 */
  | { mode: "knowledgeBase"; kbId: string }
  /** 消息作用域：登录态用户读取会话回复里引用的源空间资源。 */
  | { mode: "message"; sessionId: string; messageId: string };

export interface ProtectedFileRequest {
  url: string;
  headers: Record<string, string>;
}

const TENANT_ACCESS: ProtectedFileAccessContext = { mode: "tenant" };

/**
 * 组件声明的作用域；缺省或标识为空时退回租户作用域，而不是拼出一条
 * 缺参数的代理 URL。
 */
export function resolveProtectedFileAccess(override?: ProtectedFileAccessContext | null): ProtectedFileAccessContext {
  if (!override) return TENANT_ACCESS;
  if (override.mode === "knowledgeBase" && !override.kbId.trim()) return TENANT_ACCESS;
  if (override.mode === "message" && (!override.sessionId.trim() || !override.messageId.trim())) {
    return TENANT_ACCESS;
  }
  return override;
}

/** 是否为需要经代理拉取的存储引用（resource://<handle>）。 */
export function isResourceRef(url: string): boolean {
  return RESOURCE_REF_RE.test(url.trim());
}

/** 是否为受保护文件代理之一的路径。 */
export function isProtectedFileProxyPath(pathname: string): boolean {
  return pathname === "/files" || KB_FILE_PROXY_PATH_RE.test(pathname) || MESSAGE_FILE_PROXY_PATH_RE.test(pathname);
}

function tenantRequestHeaders(): Record<string, string> {
  const headers: Record<string, string> = {};
  try {
    const token = (localStorage.getItem("yuheng_token") || "").trim();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const selectedTenantId = (localStorage.getItem("yuheng_selected_tenant_id") || "").trim();
    if (selectedTenantId) {
      // Always attach when a selected tenant is set. Same rationale as
      // utils/request.ts / api/chat/streame.ts: the
      // "selectedTenantId === defaultTenantId → skip" short-circuit
      // silently drops the header whenever any code path writes the
      // active tenant into yuheng_tenant, leaving authenticated file
      // fetches landing on the home tenant.
      headers["X-Tenant-ID"] = selectedTenantId;
    }
  } catch {
    // ignore localStorage read errors
  }
  return headers;
}

/**
 * 为一个存储引用构造代理请求。返回 null 表示这不是需要代理的存储引用。
 */
export function buildProtectedFileRequest(
  sourceURL: string,
  access: ProtectedFileAccessContext,
): ProtectedFileRequest | null {
  const filePath = sourceURL.trim();
  if (!isResourceRef(filePath)) return null;

  const query = new URLSearchParams({ file_path: filePath }).toString();

  if (access.mode === "knowledgeBase") {
    return {
      url: `/api/v1/knowledge-bases/${encodeURIComponent(access.kbId.trim())}/files?${query}`,
      headers: tenantRequestHeaders(),
    };
  }

  if (access.mode === "message") {
    return {
      url: `/api/v1/sessions/${encodeURIComponent(access.sessionId.trim())}/messages/${encodeURIComponent(access.messageId.trim())}/files?${query}`,
      headers: tenantRequestHeaders(),
    };
  }

  return { url: `/files?${query}`, headers: tenantRequestHeaders() };
}
