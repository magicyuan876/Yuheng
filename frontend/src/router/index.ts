import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";
import { useGovernanceStore } from "@/stores/governance";
import { getCurrentUser, userInfoFromApi } from "@/api/auth";
import type { DeploymentCapabilityKey } from "@/config/deploymentCapabilities";
import { MessagePlugin } from "tdesign-vue-next";
import i18n from "@/i18n";
import { refreshUploadLimits } from "@/api/system";

// 上传上限是可动态调整的系统设置；每个会话在首次通过认证守卫时刷新一次
// （fire-and-forget，失败静默保留 config.js 的静态快照）。
let uploadLimitsRequested = false;
function ensureUploadLimitsFresh() {
  if (uploadLimitsRequested) return;
  uploadLimitsRequested = true;
  void refreshUploadLimits();
}

function hasPendingOIDCCallback() {
  if (typeof window === "undefined") return false;
  const hash = window.location.hash || "";
  return hash.includes("oidc_result=") || hash.includes("oidc_error=");
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/",
      redirect: "/platform/knowledge-bases",
    },
    {
      path: "/login",
      name: "login",
      component: () => import("../views/auth/Login.vue"),
      meta: { requiresAuth: false, requiresInit: false },
    },
    {
      path: "/register",
      name: "registerByInvite",
      // Share-link landing page reuses the Login form: the same Vue
      // component renders both modes and detects ?token=xxx on mount
      // to switch into invite-register flow. Avoids a parallel page
      // that would duplicate the OIDC / language-switch / styling
      // surface for one extra field.
      component: () => import("../views/auth/Login.vue"),
      meta: { requiresAuth: false, requiresInit: false },
    },
    // Anonymous document routes. No authentication, no workspace, no init
    // check: a visitor following a shared URL has none of those and must not
    // be sent to a login page. The short "/d/" prefix keeps a link that
    // somebody pastes into a chat readable.
    {
      path: "/d/:key",
      name: "docsPublicLink",
      component: () => import("../views/docs/public/PublicDoc.vue"),
      meta: { requiresAuth: false, requiresInit: false, requiresTenant: false },
    },
    {
      path: "/s/:spaceId",
      name: "docsPublicSpace",
      component: () => import("../views/docs/public/PublicSpace.vue"),
      meta: { requiresAuth: false, requiresInit: false, requiresTenant: false },
    },
    {
      path: "/s/:spaceId/:short",
      name: "docsPublicSpacePage",
      component: () => import("../views/docs/public/PublicDoc.vue"),
      meta: { requiresAuth: false, requiresInit: false, requiresTenant: false },
    },
    {
      path: "/onboarding/workspace",
      name: "workspaceOnboarding",
      component: () => import("../views/auth/WorkspaceOnboarding.vue"),
      meta: { requiresAuth: true, requiresInit: false, requiresTenant: false },
    },
    {
      path: "/platform",
      name: "Platform",
      redirect: "/platform/knowledge-bases",
      component: () => import("../views/platform/index.vue"),
      meta: { requiresInit: true, requiresAuth: true },
      children: [
        {
          path: "settings",
          name: "settings",
          component: () => import("../views/settings/Settings.vue"),
          meta: { requiresInit: true, requiresAuth: true },
        },
        {
          path: "knowledge-bases",
          name: "knowledgeBaseList",
          component: () => import("../views/knowledge/KnowledgeBaseList.vue"),
          meta: { requiresInit: true, requiresAuth: true },
        },
        {
          path: "knowledge-bases/:kbId",
          name: "knowledgeBaseDetail",
          component: () => import("../views/knowledge/KnowledgeBase.vue"),
          meta: { requiresInit: true, requiresAuth: true },
        },
        {
          path: "creatChat",
          name: "globalCreatChat",
          component: () => import("../views/creatChat/creatChat.vue"),
          meta: { requiresInit: true, requiresAuth: true },
        },
        {
          path: "knowledge-bases/:kbId/creatChat",
          name: "kbCreatChat",
          component: () => import("../views/creatChat/creatChat.vue"),
          meta: { requiresInit: true, requiresAuth: true },
        },
        {
          path: "chat/:chatid",
          name: "chat",
          component: () => import("../views/chat/index.vue"),
          meta: { requiresInit: true, requiresAuth: true },
        },
        // Online documents (docs module). Registered only when the backend
        // reports the `docs` capability; see internal/docs.
        {
          path: "docs",
          name: "docsSpaceList",
          component: () => import("../views/docs/SpaceList.vue"),
          meta: { requiresInit: true, requiresAuth: true, requiredCapability: "docs" },
        },
        {
          path: "docs/spaces/:slug/settings",
          name: "docsSpaceSettings",
          component: () => import("../views/docs/SpaceSettings.vue"),
          meta: { requiresInit: true, requiresAuth: true, requiredCapability: "docs" },
        },
        {
          // One record for the space home and its pages so the tree keeps
          // its state while the user moves between pages.
          path: "docs/spaces/:slug/:pageSlug?",
          name: "docsSpace",
          component: () => import("../views/docs/SpaceHome.vue"),
          meta: { requiresInit: true, requiresAuth: true, requiredCapability: "docs" },
        },
      ],
    },
    // Dev-only markdown rendering test page
    ...(import.meta.env.DEV
      ? [
          {
            path: "/platform/dev/markdown",
            name: "markdownTest",
            component: () => import("../views/dev/MarkdownTestPage.vue"),
            meta: { requiresAuth: false, requiresInit: false },
          },
        ]
      : []),
  ],
});

async function hydrateSessionFromToken(authStore: ReturnType<typeof useAuthStore>) {
  const token = localStorage.getItem("yuheng_token");
  if (!token) return false;

  if (!authStore.token) {
    authStore.setToken(token);
  }

  const storedRefreshToken = localStorage.getItem("yuheng_refresh_token");
  if (storedRefreshToken && !authStore.refreshToken) {
    authStore.setRefreshToken(storedRefreshToken);
  }

  try {
    const response = await getCurrentUser();
    const user = response.data?.user;
    if (!response.success || !user) {
      return false;
    }

    authStore.setUser(userInfoFromApi(user, response.data?.tenant?.id));

    const tenant = response.data?.tenant;
    if (tenant) {
      authStore.setTenant({
        id: String(tenant.id) || "",
        name: tenant.name || "",
        owner_id: tenant.owner_id || user.id || "",
        description: tenant.description,
        status: tenant.status,
        business: tenant.business,
        storage_quota: tenant.storage_quota,
        storage_used: tenant.storage_used,
        created_at: tenant.created_at || new Date().toISOString(),
        updated_at: tenant.updated_at || new Date().toISOString(),
      });
    } else {
      authStore.setTenant(null);
    }

    // Refresh memberships on every page load — same reason as
    // App.vue's syncOIDCUserContext: without this the auth store
    // would only ever see the snapshot from the original /auth/login
    // call, so role changes (and tenant-switch role lookups) would
    // be silently stale until the user logged out and back in.
    const memberships = response.data?.memberships;
    if (Array.isArray(memberships)) {
      authStore.setMemberships(memberships);
    }

    const canCreateTenant = response.data?.capabilities?.can_create_tenant;
    if (typeof canCreateTenant === "boolean") {
      authStore.setCanCreateTenant(canCreateTenant);
    }

    authStore.setAutoAcceptInvitation(response.data?.capabilities?.auto_accept_invitation === true);

    return true;
  } catch {
    return false;
  }
}

// 路由守卫：检查认证状态和系统初始化状态
router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore();

  // OIDC 回跳登录结果依赖 App.vue 在挂载后消费 URL hash。
  // 如果这里先按“未登录”拦截到 /login，会导致回调结果没有机会落盘。
  if (hasPendingOIDCCallback()) {
    next();
    return;
  }

  // Tenantless onboarding still requires a valid user token even though it
  // deliberately skips the normal tenant/system-initialization gates.
  if (to.path === "/onboarding/workspace") {
    if (!authStore.isLoggedIn) {
      const restored = await hydrateSessionFromToken(authStore);
      if (!restored) {
        next("/login");
        return;
      }
    }
    if (authStore.hasValidTenant) {
      next("/platform/knowledge-bases");
    } else {
      next();
    }
    return;
  }

  // 如果访问的是登录页面或初始化页面，直接放行
  if (to.meta.requiresAuth === false || to.meta.requiresInit === false) {
    // 如果已登录用户访问登录页面，重定向到知识库列表页面
    if (to.path === "/login" && authStore.isLoggedIn) {
      next(authStore.hasValidTenant ? "/platform/knowledge-bases" : "/onboarding/workspace");
      return;
    }
    next();
    return;
  }

  // 检查用户认证状态
  if (to.meta.requiresAuth !== false) {
    if (!authStore.isLoggedIn) {
      const restored = await hydrateSessionFromToken(authStore);
      if (restored) {
        next(!authStore.hasValidTenant && to.meta.requiresTenant !== false ? "/onboarding/workspace" : to.fullPath);
        return;
      }

      next("/login");
      return;
    }
  }

  if (to.meta.requiresTenant !== false && !authStore.hasValidTenant) {
    next("/onboarding/workspace");
    return;
  }

  // 部署能力只描述“后端是否提供该功能”，不反映服务健康或是否已配置。
  // 探测失败时 Store 会 fail-open，真正的权限和可用性仍由后端接口校验。
  const deploymentCapabilities = useDeploymentCapabilitiesStore();
  // 顺带把上传上限刷新为系统设置的实时值（fire-and-forget，只在首次导航真正发请求）。
  ensureUploadLimitsFresh();
  // 治理模式决定设置页要不要渲染基础设施入口。和能力探测一起 await，避免菜单先
  // 按非集中管控渲染、拿到结果后再抽掉几项造成闪烁。探测失败同样 fail-open。
  const governance = useGovernanceStore();
  await Promise.all([deploymentCapabilities.ensureLoaded(), governance.ensureLoaded()]);
  const requiredCapability = to.meta.requiredCapability as DeploymentCapabilityKey | undefined;
  if (requiredCapability && !deploymentCapabilities.isSupported(requiredCapability)) {
    MessagePlugin.warning(i18n.global.t("settings.capabilityUnavailable"));
    next("/platform/knowledge-bases");
    return;
  }

  // SystemAdmin gate — checked AFTER auth so a non-admin who's logged
  // out gets redirected to /login first (consistent with how the rest
  // of the auth flow works), and only an authenticated non-admin sees
  // the bounce. This is UI-only; the server enforces the real check.
  if (to.meta.requiresSystemAdmin === true) {
    if (!authStore.isSystemAdmin) {
      next("/platform/knowledge-bases");
      return;
    }
  }

  next();
});

export default router;
