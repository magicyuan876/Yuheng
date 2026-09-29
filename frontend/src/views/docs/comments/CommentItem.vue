<template>
  <div class="py-1">
    <header class="flex items-baseline gap-1.5 text-xs">
      <span class="text-foreground font-medium">{{ authorName }}</span>
      <time class="text-placeholder text-[11px]" :datetime="comment.created_at">{{ when }}</time>
      <span v-if="comment.edited_at" class="text-placeholder text-[11px]">{{ t("docs.comments.edited") }}</span>

      <span class="flex-1" />
      <template v-if="!editing">
        <button
          v-if="comment.can_edit"
          type="button"
          data-slot="comment-action"
          class="text-placeholder hover:text-primary text-[11px]"
          @click.stop="startEditing"
        >
          {{ t("common.edit") }}
        </button>
        <button
          v-if="comment.can_delete"
          type="button"
          data-slot="comment-action"
          class="text-placeholder hover:text-primary text-[11px]"
          @click.stop="confirmDelete"
        >
          {{ t("common.delete") }}
        </button>
      </template>
    </header>

    <CommentComposer
      v-if="editing"
      :initial="comment.body"
      :busy="busy"
      :submit-label="t('common.save')"
      @submit="commitEdit"
      @cancel="editing = false"
    />
    <!-- The body is generated markup, so its paragraphs and code blocks are
         styled as descendants: tight paragraph spacing, and a tinted block
         for code. -->
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div
      v-else
      class="[&_pre]:bg-accent mt-0.5 text-[13px] leading-[1.6] [word-break:break-word] [&_p]:mt-0 [&_p]:mb-1 [&_p:last-child]:mb-0 [&_pre]:overflow-x-auto [&_pre]:rounded-[4px] [&_pre]:px-2 [&_pre]:py-1.5 [&_pre]:text-xs"
      v-html="html"
    />
  </div>
</template>

<script setup lang="ts">
import { generateHTML } from "@tiptap/core";
import { DialogPlugin } from "tdesign-vue-next";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

import type { CommentView } from "@/api/docs";

import { officialExtensions } from "../editor/extensions";

import CommentComposer from "./CommentComposer.vue";

const props = defineProps<{ comment: CommentView; busy?: boolean }>();
const emit = defineEmits<{ edit: [body: unknown]; delete: [] }>();
const { t, locale } = useI18n();

const editing = ref(false);

const authorName = computed(
  () => props.comment.creator?.username || props.comment.creator?.email || t("docs.links.someone"),
);

const when = computed(() => {
  const at = new Date(props.comment.created_at);
  if (Number.isNaN(at.getTime())) return props.comment.created_at;
  return at.toLocaleString(locale.value, { dateStyle: "short", timeStyle: "short" });
});

/**
 * The comment, rendered read-only.
 *
 * Markup rather than an editor instance, as everywhere else a stored document
 * is shown: a thread can hold dozens of these and none of them is editable
 * until somebody says so. Nothing in the markup can execute — the schema has
 * no script, iframe or style node type, and the server narrows a comment
 * further still.
 */
const html = computed(() => {
  const body = props.comment.body;
  if (!body) return "";
  try {
    return generateHTML(body as Record<string, unknown>, officialExtensions() as never);
  } catch {
    return "";
  }
});

function startEditing() {
  editing.value = true;
}

function commitEdit(body: unknown) {
  editing.value = false;
  emit("edit", body);
}

function confirmDelete() {
  const dialog = DialogPlugin.confirm({
    header: t("docs.comments.deleteTitle"),
    body: props.comment.parent_id ? t("docs.comments.deleteConfirm") : t("docs.comments.deleteThreadConfirm"),
    confirmBtn: { content: t("common.delete"), theme: "danger" },
    onConfirm: () => {
      dialog.hide();
      emit("delete");
    },
  });
}
</script>
