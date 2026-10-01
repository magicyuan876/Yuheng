<template>
  <div class="w-full">
    <div class="mb-5">
      <h2 class="mt-0 mb-1.5 text-xl">{{ $t("kbSettings.storage.title") }}</h2>
      <p class="text-muted-foreground">
        {{ $t("kbSettings.storage.selectDescription") }}
      </p>
    </div>
    <div v-if="loading" class="flex items-center gap-2">
      <Loader2Icon class="size-4 animate-spin" /><span>{{ $t("kbSettings.storage.loading") }}</span>
    </div>
    <div v-else class="flex flex-col">
      <div class="flex justify-between gap-7">
        <div class="flex-1">
          <label>{{ $t("kbSettings.storage.instanceLabel") }}</label>
          <p class="text-muted-foreground">
            {{ $t("kbSettings.storage.instanceDesc") }}
          </p>
        </div>
        <div class="w-[45%] min-w-[300px]">
          <Select v-model="localID" @update:model-value="handleChange">
            <SelectTrigger class="w-full" style="min-width: 260px">
              <!-- The trigger shows the backend name only, as the old select's label did; without
                   a slot Reka would copy the whole item, badges included, into the trigger. -->
              <SelectValue>{{ selected?.name }}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="backend in backends" :key="backend.id" :value="backend.id">
                <span class="flex items-center gap-2">
                  <span>{{ backend.name }}</span>
                  <Badge class="bg-primary/10 text-primary hover:bg-primary/10">{{
                    backend.provider.toUpperCase()
                  }}</Badge>
                  <Badge v-if="backend.id === defaultID" variant="secondary">{{
                    $t("kbSettings.storage.defaultTag")
                  }}</Badge>
                </span>
              </SelectItem>
            </SelectContent>
          </Select>
          <p v-if="rebinding" class="text-warning my-2 text-xs">
            {{ $t("kbSettings.storage.rebindHint") }}
          </p>
          <p v-else-if="selected" class="text-muted-foreground my-2 text-xs">
            {{
              selected.config.endpoint ||
              selected.config.bucket_name ||
              selected.config.path_prefix ||
              $t("kbSettings.storage.localStorage")
            }}
          </p>
          <a
            v-if="canManageStorage"
            href="javascript:void(0)"
            class="text-primary text-[13px]"
            @click.prevent="goToSettings"
            >{{ $t("kbSettings.storage.manageInstances") }}</a
          >
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { Loader2Icon } from "@lucide/vue";
import { listStorageBackends, type StorageBackend } from "@/api/storage-backend";
import { useUIStore } from "@/stores/ui";
import { usePlatformInfraAccess } from "@/composables/usePlatformInfraAccess";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

// The binding decides only where new files go: each stored file is read
// through its own record, which remembers its backend. So a knowledge base
// with files may be rebound, and the panel says what that means instead of
// locking the choice.
// boundStorageBackendId is the binding the knowledge base was opened with;
// the panel is remounted whenever the editor switches sections, so it cannot
// remember that itself.
const props = defineProps<{ storageBackendId?: string; boundStorageBackendId?: string; hasFiles?: boolean }>();
const emit = defineEmits<{
  "update:storageBackendId": [value: string];
}>();
const uiStore = useUIStore();
// 集中管控模式下存储配置归系统管理员，入口对其他人隐藏；不 gate 的话这个链接会
// 打开一个渲染不出内容的设置页。选择器本身照常工作，只是不再提供「去配置」。
const canManageStorage = usePlatformInfraAccess("storage");
const loading = ref(false),
  backends = ref<StorageBackend[]>([]),
  defaultID = ref(""),
  localID = ref(props.storageBackendId || "");
const selected = computed(() => backends.value.find((item) => item.id === localID.value));
const rebinding = computed(
  () => !!props.hasFiles && !!props.boundStorageBackendId && localID.value !== props.boundStorageBackendId,
);

function handleChange() {
  emit("update:storageBackendId", localID.value);
}
async function load() {
  loading.value = true;
  try {
    const response = await listStorageBackends();
    backends.value = (response.data || []).filter((item) => item.status === "active");
    defaultID.value = response.default_storage_backend_id;
    // A new knowledge base starts on the workspace default, which is what the
    // server would bind anyway; showing it makes the choice visible.
    if (!localID.value) localID.value = defaultID.value;
    if (localID.value) handleChange();
  } finally {
    loading.value = false;
  }
}
function goToSettings() {
  uiStore.closeKBEditor?.();
  uiStore.openSettings?.("storage");
}
watch(
  () => props.storageBackendId,
  (value) => {
    if (value) localID.value = value;
  },
);
onMounted(load);
</script>
