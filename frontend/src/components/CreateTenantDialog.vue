<template>
  <!-- 自助创建新工作区弹窗。任意已登录用户均可调用 POST /api/v1/tenants
       （后端 router 已去掉 g.CrossTenant() 守卫），handler 会自动把当前
       用户 EnsureOwner 成新空间的 Owner。 -->
  <Dialog :open="visible" @update:open="onVisibleUpdate">
    <!-- While a submit is in flight the dialog must not be dismissed by the
         overlay or Esc, as the old t-dialog's close-on-* flags ensured. -->
    <DialogContent
      class="sm:max-w-[480px]"
      @interact-outside="(e: Event) => submitting && e.preventDefault()"
      @escape-key-down="(e: KeyboardEvent) => submitting && e.preventDefault()"
    >
      <DialogHeader>
        <DialogTitle class="inline-flex items-center gap-2">
          <LayoutGridIcon class="text-primary size-5 shrink-0" aria-hidden="true" />
          <span>{{ $t("tenant.create.dialogTitle") }}</span>
        </DialogTitle>
        <DialogDescription class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
          {{ $t("tenant.create.dialogSubtitle") }}
        </DialogDescription>
      </DialogHeader>

      <form class="flex flex-col gap-4" novalidate @submit.prevent="handleSubmit">
        <div class="flex flex-col gap-1.5">
          <Label for="create-tenant-name">{{ $t("tenant.create.nameLabel") }}</Label>
          <Input
            id="create-tenant-name"
            v-model="form.name"
            :placeholder="$t('tenant.create.namePlaceholder')"
            :maxlength="128"
            :aria-invalid="!!nameError || undefined"
            autofocus
            @blur="validateName"
          />
          <p v-if="nameError" class="text-destructive m-0 text-xs">{{ nameError }}</p>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="create-tenant-description">{{ $t("tenant.create.descriptionLabel") }}</Label>
          <!-- The old autosize grew between three and five rows. -->
          <Textarea
            id="create-tenant-description"
            v-model="form.description"
            :placeholder="$t('tenant.create.descriptionPlaceholder')"
            :maxlength="512"
            class="max-h-[7.5rem] min-h-[4.75rem]"
          />
        </div>
        <!-- A hidden submit button lets Enter in the name field submit the form,
             which is what the old @enter handler did. -->
        <button type="submit" class="hidden" tabindex="-1" aria-hidden="true" />
      </form>

      <DialogFooter>
        <Button variant="outline" :disabled="submitting" @click="handleClose">{{ $t("tenant.create.cancel") }}</Button>
        <Button :disabled="submitting" @click="handleSubmit">
          <Loader2Icon v-if="submitting" class="animate-spin" />
          {{ $t("tenant.create.submit") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { LayoutGridIcon, Loader2Icon } from "@lucide/vue";
import { MessagePlugin } from "tdesign-vue-next";
import { createTenant, type TenantInfo } from "@/api/tenant";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

const props = defineProps<{
  visible: boolean;
}>();

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  // 创建成功后由父组件决定如何导航（切换到新空间、刷新本地列表等）。
  (e: "created", tenant: TenantInfo): void;
}>();

const { t } = useI18n();

const submitting = ref(false);

const form = reactive({
  name: "",
  description: "",
});

// The field error under the name input. It replaces the TDesign form rule,
// and like that rule it is shown on blur and on submit.
const nameError = ref("");

// Trim-aware required check：全空格不算通过；这里手动校验 trim 后非空。
// max 长度由 :maxlength 在键入时硬限制，所以这里不再重复挂规则（避免与
// 硬限制双重提示）。
const validateName = (): boolean => {
  const ok = (form.name ?? "").trim().length > 0;
  nameError.value = ok ? "" : t("tenant.create.nameRequired");
  return ok;
};

watch(
  () => props.visible,
  (open) => {
    if (open) {
      form.name = "";
      form.description = "";
      nameError.value = "";
    }
  },
);

const onVisibleUpdate = (next: boolean) => {
  if (!next && submitting.value) return;
  emit("update:visible", next);
};

const handleClose = () => {
  if (submitting.value) return;
  emit("update:visible", false);
};

const handleSubmit = async () => {
  if (submitting.value) return;
  if (!validateName()) return;

  submitting.value = true;
  try {
    const response = await createTenant({
      name: form.name.trim(),
      description: form.description.trim() || undefined,
    });
    if (!response.success || !response.data) {
      MessagePlugin.error(response.message || t("tenant.create.failed"));
      return;
    }
    MessagePlugin.success(t("tenant.create.success"));
    emit("created", response.data);
    emit("update:visible", false);
  } catch (error: any) {
    console.error("Failed to create tenant:", error);
    MessagePlugin.error(error?.message || t("tenant.create.failed"));
  } finally {
    submitting.value = false;
  }
};
</script>
