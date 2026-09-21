import { ref } from "vue";
import { defineStore } from "pinia";
import { getGovernance } from "@/api/system";

/**
 * 平台治理模式。
 *
 * `centralizedInfra` 为 true 时，基础设施（模型、MCP、解析引擎、向量库、存储、
 * 沙箱、网络搜索、Ollama）的配置权归系统管理员，空间管理员只保留读权限 ——
 * 建知识库、建智能体时仍能正常选用平台资源，只是设置页里不再出现这些入口。
 *
 * 为什么需要这个 store：空间角色表达不了这件事。每个自助注册的用户都是自己个人
 * 空间的 Owner，`hasRole('admin')` 恒真，所以必须有一个与角色正交的维度。
 *
 * 失败时 fail-open（保持 false，即非集中管控）：探测失败只应让入口多显示，不该
 * 把管理员锁在自己的基础设施外面。真正的权限判定始终在后端路由守卫
 * （middleware.RequirePlatformManaged），前端这里只决定入口渲不渲染。
 */
export const useGovernanceStore = defineStore("governance", () => {
  const centralizedInfra = ref(false);
  const loaded = ref(false);
  const loadError = ref("");
  let loadingPromise: Promise<void> | null = null;

  const ensureLoaded = async (force = false): Promise<void> => {
    if (loaded.value && !force) return;
    if (loadingPromise) return loadingPromise;

    loadingPromise = (async () => {
      try {
        const response = await getGovernance();
        centralizedInfra.value = response.data?.centralized_infra === true;
        loadError.value = "";
      } catch (error) {
        centralizedInfra.value = false;
        loadError.value = error instanceof Error ? error.message : String(error);
      } finally {
        loaded.value = true;
        loadingPromise = null;
      }
    })();

    return loadingPromise;
  };

  return {
    centralizedInfra,
    loaded,
    loadError,
    ensureLoaded,
  };
});
