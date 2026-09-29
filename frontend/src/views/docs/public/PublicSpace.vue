<template>
  <div class="bg-background min-h-screen">
    <main class="mx-auto max-w-[720px] px-5 pt-16 pb-24">
      <div v-if="loading && !space && !missing" class="flex items-center justify-center gap-2 py-8">
        <Loader2Icon class="size-4 animate-spin" />
      </div>
      <template v-else>
        <section v-if="missing" class="text-muted-foreground flex flex-col items-center gap-3 py-20 text-center">
          <UnlinkIcon class="size-7" />
          <h1 class="text-foreground m-0 text-xl">{{ t("docs.public.state.gone") }}</h1>
          <p class="m-0 text-sm">{{ t("docs.public.stateHint") }}</p>
        </section>

        <template v-else-if="space">
          <header class="mb-8">
            <span v-if="space.icon" class="mb-2.5 block text-[44px] leading-none">{{ space.icon }}</span>
            <h1 class="m-0 text-[30px] font-bold text-balance">{{ space.name }}</h1>
            <p v-if="space.description" class="text-muted-foreground mt-2.5 mb-0 text-[15px] leading-[1.6]">
              {{ space.description }}
            </p>
          </header>

          <ul v-if="space.pages.length" class="m-0 flex list-none flex-col gap-0.5 p-0">
            <li v-for="p in space.pages" :key="p.short_id">
              <button
                type="button"
                data-slot="public-space-page"
                class="hover:bg-accent flex w-full cursor-pointer items-center gap-2.5 rounded-[8px] border-0 px-3 py-2.5 text-left text-[15px] text-inherit"
                @click="open(p.short_id)"
              >
                <span class="w-5 flex-none text-center">{{ p.icon || "📄" }}</span>
                <span>{{ p.title || t("docs.tree.untitled") }}</span>
              </button>
            </li>
          </ul>
          <p v-else class="text-placeholder text-sm">{{ t("docs.public.spaceEmpty") }}</p>
        </template>
      </template>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

import { Loader2Icon, UnlinkIcon } from "@lucide/vue";

import { visitPublicSpace, type PublicSpaceView } from "@/api/docs";

// The front door of a space published to anyone.
//
// Like the document view next to it, there is no application around this: a
// visitor gets the space's name and its top-level pages, and nothing that
// would only make sense to somebody with an account.

const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const loading = ref(false);
const missing = ref(false);
const space = shallowRef<PublicSpaceView | null>(null);

const spaceId = computed(() => String(route.params.spaceId ?? ""));

async function load() {
  loading.value = true;
  missing.value = false;
  space.value = null;
  const id = spaceId.value;
  try {
    const view = await visitPublicSpace(id);
    if (spaceId.value === id) space.value = view;
  } catch {
    // A private space, a space that does not exist, and a deployment with
    // sharing switched off are all the same thing from out here.
    if (spaceId.value === id) missing.value = true;
  } finally {
    if (spaceId.value === id) loading.value = false;
  }
}

function open(shortId: string) {
  router.push({ name: "docsPublicSpacePage", params: { spaceId: spaceId.value, short: shortId } });
}

watch(
  spaceId,
  () => {
    void load();
  },
  { immediate: true },
);
</script>
