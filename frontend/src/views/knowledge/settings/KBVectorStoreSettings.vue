<template>
  <div class="w-full">
    <div class="mb-5">
      <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ $t("kbSettings.vectorStore.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("kbSettings.vectorStore.description") }}</p>
    </div>

    <div
      v-if="props.mode === 'create' && loading"
      class="text-muted-foreground flex items-center gap-2 py-4 text-[13px]"
    >
      <Loader2Icon class="size-4 animate-spin" />
      <span>{{ $t("kbSettings.vectorStore.loading") }}</span>
    </div>

    <!-- CREATE mode: dropdown -->
    <div v-else-if="props.mode === 'create'" class="flex flex-col">
      <div class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b">
        <div class="max-w-[40%] shrink-0 basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("kbSettings.vectorStore.engineLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("kbSettings.vectorStore.engineDesc") }}
          </p>
        </div>
        <div class="flex max-w-[55%] shrink-0 basis-[55%] flex-col items-start gap-1.5">
          <Select :model-value="localVectorStoreId || SYSTEM_DEFAULT_VALUE" @update:model-value="handleChange">
            <SelectTrigger class="w-full" style="min-width: 220px">
              <!-- Only the option's label goes into the trigger, as with t-select; left to itself
                   Reka would copy the whole item, engine badge included. -->
              <SelectValue :placeholder="$t('kbSettings.vectorStore.systemDefault')">{{ selectedLabel }}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem :value="SYSTEM_DEFAULT_VALUE">
                <span class="inline-flex items-center gap-2">
                  <span>{{ $t("kbSettings.vectorStore.systemDefault") }}</span>
                  <Badge v-if="envEngineType" class="bg-primary/10 text-primary hover:bg-primary/10">
                    {{ envEngineType }}
                  </Badge>
                </span>
              </SelectItem>
              <SelectItem v-for="s in userStores" :key="s.id" :value="s.id || SYSTEM_DEFAULT_VALUE">
                <span class="inline-flex items-center gap-2">
                  <span>{{ s.name }}</span>
                  <Badge class="bg-success/10 text-success hover:bg-success/10">{{ s.engine_type }}</Badge>
                </span>
              </SelectItem>
            </SelectContent>
          </Select>
          <p class="text-placeholder m-0 text-xs leading-[1.4]">{{ $t("kbSettings.vectorStore.immutableHint") }}</p>
          <a
            v-if="canManageVectorStore"
            href="javascript:void(0)"
            class="text-primary mt-2 text-[13px] no-underline hover:underline"
            @click.prevent="goToVectorStoreSettings"
          >
            {{ $t("kbSettings.vectorStore.goGlobalSettings") }}
          </a>
        </div>
      </div>
    </div>

    <!-- EDIT mode: read-only display -->
    <div v-else class="flex flex-col">
      <div class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b">
        <div class="max-w-[40%] shrink-0 basis-2/5 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("kbSettings.vectorStore.boundLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("kbSettings.vectorStore.immutableEdit") }}
          </p>
        </div>
        <div class="flex max-w-[55%] shrink-0 basis-[55%] flex-col items-start gap-1.5">
          <VectorStoreBadge
            :source="props.boundSource"
            :name="props.boundName"
            :engine-type="props.boundEngineType"
            :status="props.boundStatus"
          />
          <p v-if="props.boundStatus === 'unavailable'" class="text-warning m-0 text-xs leading-[1.4]">
            {{ $t("kbSettings.vectorStore.unavailableHint") }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { Loader2Icon } from "@lucide/vue";
import { useUIStore } from "@/stores/ui";
import { usePlatformInfraAccess } from "@/composables/usePlatformInfraAccess";
import { listVectorStores, type VectorStoreEntity } from "@/api/vector-store";
import type { VectorStoreSource, VectorStoreStatus } from "@/api/knowledge-base";
import VectorStoreBadge from "@/components/VectorStoreBadge.vue";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const props = defineProps<{
  mode: "create" | "edit";
  // create mode — current selection (empty string for env-default)
  vectorStoreId?: string;
  // edit mode — bound store info already on the KB
  boundSource?: VectorStoreSource;
  boundName?: string;
  boundEngineType?: string;
  boundStatus?: VectorStoreStatus;
}>();

const emit = defineEmits<{
  (e: "update:vectorStoreId", id: string): void;
}>();

const { t } = useI18n();
const uiStore = useUIStore();
const canManageVectorStore = usePlatformInfraAccess("vectorstore");

// reka-ui SelectItem rejects an empty-string value, so the "System default"
// entry uses a sentinel and is mapped back to "" on change.
const SYSTEM_DEFAULT_VALUE = "__system_default__";

const loading = ref(false);
const allStores = ref<VectorStoreEntity[]>([]);
const localVectorStoreId = ref<string>(props.vectorStoreId || "");

// Only show user-defined stores in the dropdown. The env store is
// surfaced via the explicit "System default" entry at the top of the
// list; including it twice would confuse users about which one is the
// fallback path.
const userStores = computed(() => allStores.value.filter((s) => s.source === "user"));

// Engine type for the env store, shown as a tag next to the "System
// default" label so users know which storage backend handles unbound
// KBs (e.g. "postgres"). When the env-store entry is missing or its
// engine type is not populated, the tag is hidden entirely rather than
// showing a placeholder.
const envEngineType = computed(() => {
  const envStore = allStores.value.find((s) => s.source === "env");
  return envStore?.engine_type || "";
});

const selectedLabel = computed(() => {
  if (!localVectorStoreId.value) return t("kbSettings.vectorStore.systemDefault");
  return userStores.value.find((s) => s.id === localVectorStoreId.value)?.name || localVectorStoreId.value;
});

watch(
  () => props.vectorStoreId,
  (v) => {
    localVectorStoreId.value = v || "";
  },
);

const handleChange = (raw: unknown) => {
  const val = typeof raw === "string" ? raw : "";
  // Map the sentinel back to empty string so the parent treats it as
  // "use system default". The local copy is updated too, as the old
  // v-model did, so the select holds its value even if the parent does
  // not echo the prop back.
  localVectorStoreId.value = !val || val === SYSTEM_DEFAULT_VALUE ? "" : val;
  emit("update:vectorStoreId", localVectorStoreId.value);
};

// Open the global Settings panel directly on the Vector Stores
// section. This follows the same pattern as the other KB editor "go
// to settings" links (parser, storage, models): it talks to the UI
// store rather than navigating via the router, so the host editor
// modal stays mounted and can be returned to once the user closes the
// settings panel.
const goToVectorStoreSettings = () => {
  uiStore.openSettings("vectorstore");
};

onMounted(async () => {
  if (props.mode !== "create") return;
  loading.value = true;
  try {
    const resp = await listVectorStores();
    if (resp.success) allStores.value = resp.data || [];
  } catch (e) {
    // Graceful degradation: if vector-store listing fails the dropdown
    // simply renders only the "System default" entry, which is what a
    // tenant without custom stores sees anyway. The KB editor remains usable.
    console.warn("[KBVectorStoreSettings] failed to load vector stores", e);
  } finally {
    loading.value = false;
  }
});
</script>
