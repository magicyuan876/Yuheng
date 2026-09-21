/**
 * 外部文档 / 仓库链接的唯一来源。
 *
 * 平台部署为内部系统，默认不指向任何上游开源仓库。所有值留空时，引用它们
 * 的入口（用户菜单的「文档」「GitHub」、各设置页的「查看文档」等）会自动
 * 隐藏 —— 调用方以 `v-if="LINK"` 或 `if (!LINK) return` 做判断。
 *
 * 若之后自建了文档站，把 DOCS_BASE_URL 填成文档站根地址即可，
 * docsUrl() 会拼出对应页面；无需改动各个组件。
 */

/** 文档站根地址，例如 'https://kb-docs.internal.example.com'。留空则隐藏所有文档入口。 */
export const DOCS_BASE_URL: string = "";

/** 源码仓库地址。留空则隐藏「GitHub」类入口。 */
export const REPO_URL: string = "";

/** 问题反馈地址（工单系统或 issue 页）。留空则隐藏「反馈问题」入口。 */
export const ISSUE_TRACKER_URL: string = "";

/**
 * 拼出一篇文档的地址。`path` 用相对路径，例如 'RBAC.md' 或 'features/graph'。
 * DOCS_BASE_URL 为空时返回空串，调用方据此隐藏入口。
 */
export function docsUrl(path: string): string {
  if (!DOCS_BASE_URL) return "";
  return `${DOCS_BASE_URL.replace(/\/+$/, "")}/${path.replace(/^\/+/, "")}`;
}
