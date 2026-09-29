<template>
  <div class="text-muted-foreground flex flex-wrap items-center justify-end gap-x-3 gap-y-2 text-[13px]">
    <!--
      The pager under the members and pending-invitation tables. It stands in
      for TDesign's t-pagination with show-page-size, show-page-number and
      show-jumper, which is the one combination those tables used: a total,
      a page-size select, numbered pages and a "go to page" box.
    -->
    <span class="whitespace-nowrap">{{ $t("tenantMember.pager.total", { total }) }}</span>

    <Select :model-value="String(pageSize)" @update:model-value="(v) => onPageSizeChange(Number(v))">
      <SelectTrigger size="sm" class="text-[13px]" :aria-label="$t('tenantMember.pager.pageSizeLabel')">
        <SelectValue />
      </SelectTrigger>
      <!-- Above the settings modal (z 1100) and the invite popovers (z 3050). -->
      <SelectContent position="popper" class="z-[6200]">
        <SelectItem v-for="size in pageSizeOptions" :key="size" :value="String(size)">
          {{ $t("tenantMember.pager.pageSize", { size }) }}
        </SelectItem>
      </SelectContent>
    </Select>

    <!--
      Reka's pagination primitives, styled with the button variants
      directly: the shadcn wrappers in ui/pagination import a style path
      this project does not have.
    -->
    <PaginationRoot
      v-slot="{ page: current }"
      :page="page"
      :total="total"
      :items-per-page="pageSize"
      :sibling-count="1"
      show-edges
      @update:page="onPageChange"
    >
      <PaginationList v-slot="{ items }" class="flex items-center gap-0.5">
        <PaginationPrev
          :class="buttonVariants({ variant: 'ghost', size: 'icon-sm' })"
          :aria-label="$t('tenantMember.pager.previous')"
        >
          <ChevronLeftIcon />
        </PaginationPrev>
        <template v-for="(item, index) in items" :key="index">
          <PaginationListItem
            v-if="item.type === 'page'"
            :value="item.value"
            :class="[
              buttonVariants({ variant: item.value === current ? 'outline' : 'ghost', size: 'icon-sm' }),
              'text-[13px]',
              item.value === current ? 'border-primary text-primary hover:text-primary' : 'text-foreground',
            ]"
          >
            {{ item.value }}
          </PaginationListItem>
          <PaginationEllipsis v-else :index="index" class="flex size-7 items-center justify-center">
            <EllipsisIcon class="size-4" />
          </PaginationEllipsis>
        </template>
        <PaginationNext
          :class="buttonVariants({ variant: 'ghost', size: 'icon-sm' })"
          :aria-label="$t('tenantMember.pager.next')"
        >
          <ChevronRightIcon />
        </PaginationNext>
      </PaginationList>
    </PaginationRoot>

    <label class="flex items-center gap-1.5 whitespace-nowrap">
      {{ $t("tenantMember.pager.jumpTo") }}
      <Input
        v-model="jumpValue"
        type="number"
        :min="1"
        :max="pageCount"
        class="h-7 w-14 px-1.5 text-center text-[13px]"
        @keydown.enter="commitJump"
        @blur="commitJump"
      />
      <span v-if="$t('tenantMember.pager.jumpToSuffix')">{{ $t("tenantMember.pager.jumpToSuffix") }}</span>
    </label>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ChevronLeftIcon, ChevronRightIcon, EllipsisIcon } from "@lucide/vue";
import { Input } from "@/components/ui/input";
import {
  PaginationEllipsis,
  PaginationList,
  PaginationListItem,
  PaginationNext,
  PaginationPrev,
  PaginationRoot,
} from "reka-ui";
import { buttonVariants } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const props = defineProps<{
  page: number;
  pageSize: number;
  total: number;
  pageSizeOptions: number[];
}>();

// `change` fires once after either model changes, the way t-pagination's
// @change did, so the parent reloads exactly once per user action.
const emit = defineEmits<{
  (e: "update:page", value: number): void;
  (e: "update:pageSize", value: number): void;
  (e: "change"): void;
}>();

const pageCount = computed(() => Math.max(1, Math.ceil(props.total / Math.max(1, props.pageSize))));

const jumpValue = ref<string | number>("");

function onPageChange(next: number) {
  if (next === props.page) return;
  emit("update:page", next);
  emit("change");
}

// A new page size keeps the reader on the page that still exists: the
// current page is clamped to the new page count rather than reset to one,
// which is what TDesign's pager did.
function onPageSizeChange(size: number) {
  if (!Number.isFinite(size) || size <= 0 || size === props.pageSize) return;
  const maxPage = Math.max(1, Math.ceil(props.total / size));
  emit("update:pageSize", size);
  if (props.page > maxPage) emit("update:page", maxPage);
  emit("change");
}

function commitJump() {
  const raw = Number(jumpValue.value);
  jumpValue.value = "";
  if (!Number.isFinite(raw) || raw <= 0) return;
  onPageChange(Math.min(pageCount.value, Math.max(1, Math.floor(raw))));
}
</script>
