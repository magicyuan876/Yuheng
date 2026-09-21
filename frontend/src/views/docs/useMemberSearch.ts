import { ref } from "vue";

import { listMembers, type TenantMember } from "@/api/tenant/members";
import { useAuthStore } from "@/stores/auth";

export interface MemberOption {
  label: string;
  value: string;
  member: TenantMember;
}

/**
 * Remote search over the current workspace's active members, for user
 * pickers. Results are debounced and stale responses are dropped.
 */
export function useMemberSearch(pageSize = 20) {
  const authStore = useAuthStore();
  const options = ref<MemberOption[]>([]);
  const loading = ref(false);
  let timer: ReturnType<typeof setTimeout> | null = null;
  let seq = 0;

  const run = async (q: string) => {
    const tenantId = authStore.effectiveTenantId;
    if (!tenantId) {
      options.value = [];
      return;
    }
    const mine = ++seq;
    loading.value = true;
    try {
      const res = await listMembers(Number(tenantId), { q, page: 1, page_size: pageSize });
      if (mine !== seq) return;
      const rows = (res.data?.members ?? []).filter((m) => m.status === "active");
      options.value = rows.map((m) => ({
        label: m.username ? `${m.username} <${m.email}>` : m.email || m.user_id,
        value: m.user_id,
        member: m,
      }));
    } catch {
      if (mine === seq) options.value = [];
    } finally {
      if (mine === seq) loading.value = false;
    }
  };

  const search = (q: string) => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      void run(q);
    }, 250);
  };

  return { options, loading, search, load: run };
}
