<template>
  <!-- A page another document superseded: still readable — from a bookmark,
       an old link — but out of the knowledge base, and the reader should know
       what replaced it. Letting it back in is its writers' call, and clears
       the mark. -->
  <Alert
    v-if="mark"
    class="border-warning/40 bg-warning/5 text-foreground"
    role="status"
    data-testid="superseded-banner"
  >
    <ReplaceIcon class="text-warning" />
    <AlertTitle>{{ t("docs.superseded.title") }}</AlertTitle>
    <AlertDescription class="flex flex-wrap items-center gap-x-2 gap-y-1">
      <span>
        {{ t("docs.superseded.body", { title: mark.title || t("docs.tree.untitled"), time: formatDate(mark.at) }) }}
      </span>
      <Button
        v-if="mark.page_id"
        variant="link"
        size="xs"
        class="h-auto px-0"
        :disabled="opening"
        data-testid="superseded-open"
        @click="openReplacement"
      >
        {{ t("docs.superseded.open") }}
      </Button>
      <Button
        v-if="page.can_edit"
        variant="outline"
        size="xs"
        class="ml-auto"
        data-testid="superseded-restore"
        @click="emit('restore')"
      >
        {{ t("docs.superseded.restore") }}
      </Button>
    </AlertDescription>
  </Alert>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
import { ReplaceIcon } from "@lucide/vue";

import { getPage, getSpace, type PageView } from "@/api/docs";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";

import { pageSlug } from "../tree/pageTree";

const props = defineProps<{ page: PageView }>();
const emit = defineEmits<{
  /** Let the page back into the knowledge base, which clears the mark. */
  restore: [];
}>();

const { t } = useI18n();
const router = useRouter();

const mark = computed(() => props.page.superseded_by ?? null);
const opening = ref(false);

// The mark keeps the replacement's page ID, not its address: a page can be
// renamed or moved since. The address is looked up when the reader asks,
// under their own permissions — a replacement they cannot read stays a title.
async function openReplacement() {
  const id = mark.value?.page_id;
  if (!id || opening.value) return;
  opening.value = true;
  try {
    const target = await getPage(id);
    const space = await getSpace(target.space_id);
    await router.push({
      name: "docsSpace",
      params: { slug: space.slug, pageSlug: pageSlug(target.title, target.short_id) },
    });
  } catch (err) {
    console.debug("docs: replacement page unavailable", err);
    void MessagePlugin.warning(t("docs.superseded.unavailable"));
  } finally {
    opening.value = false;
  }
}

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString();
};
</script>
