<template>
  <div class="bg-background min-h-screen">
    <main class="mx-auto max-w-[820px] px-5 pt-12 pb-24">
      <div v-if="loading && !page && !state" class="flex items-center justify-center gap-2 py-8">
        <Loader2Icon class="size-4 animate-spin" />
      </div>
      <template v-else>
        <!-- A dead link is a thing to render, not an error to throw. Each
             outcome gets its own sentence, because "something went wrong"
             tells the visitor nothing they can act on. -->
        <section v-if="state === 'password'" class="text-muted-foreground flex flex-col items-center gap-3 py-20 text-center">
          <LockIcon class="size-7" />
          <h1 class="text-foreground m-0 text-xl">{{ t("docs.public.passwordTitle") }}</h1>
          <p class="m-0 text-sm">{{ t("docs.public.passwordHint") }}</p>
          <form class="mt-2 flex w-full max-w-[320px] gap-2" @submit.prevent="submitPassword">
            <Input
              v-model="password"
              type="password"
              :placeholder="t('docs.public.passwordLabel')"
              :aria-invalid="wrong || undefined"
              autofocus
              @input="wrong = false"
            />
            <Button type="submit" :disabled="!password || unlocking">
              <Loader2Icon v-if="unlocking" class="animate-spin" />
              {{ t("docs.public.unlock") }}
            </Button>
          </form>
          <p v-if="wrong" class="text-destructive m-0 text-[13px]">{{ t("docs.public.wrongPassword") }}</p>
        </section>

        <section v-else-if="state && state !== 'ok'" class="text-muted-foreground flex flex-col items-center gap-3 py-20 text-center">
          <UnlinkIcon class="size-7" />
          <h1 class="text-foreground m-0 text-xl">{{ t(`docs.public.state.${state}`) }}</h1>
          <p class="m-0 text-sm">{{ t("docs.public.stateHint") }}</p>
        </section>

        <article v-else-if="page">
          <nav v-if="page.breadcrumb.length" class="text-placeholder mb-4 flex flex-wrap items-center gap-1 text-[13px]">
            <span v-for="crumb in page.breadcrumb" :key="crumb.short_id" class="inline-flex items-center gap-1">
              <button
                type="button"
                data-slot="public-doc-crumb"
                class="hover:text-primary cursor-pointer border-0 p-0"
                @click="open(crumb.short_id)"
              >
                {{ crumb.title || t("docs.tree.untitled") }}
              </button>
              <span aria-hidden="true">/</span>
            </span>
          </nav>

          <header class="mb-7">
            <span v-if="page.icon" class="mb-2 block text-[40px] leading-none">{{ page.icon }}</span>
            <h1 class="m-0 text-[32px] leading-[1.25] font-bold text-balance">
              {{ page.title || t("docs.tree.untitled") }}
            </h1>
            <p class="text-placeholder mt-2 mb-0 text-[13px]">
              {{ t("docs.public.from", { space: page.space_name }) }}
              <span v-if="page.updated_at"> · {{ t("docs.public.updated", { date: updated }) }}</span>
            </p>
          </header>

          <!-- The server renders the document; it is the same renderer the
               exports use, and it has already dropped everything a visitor
               may not see. -->
          <div class="doc-body prosemirror-host text-[15px] leading-[1.75]" v-html="page.html" />

          <nav v-if="page.children.length" class="mt-12 border-t border-[var(--td-component-stroke)] pt-5">
            <h2 class="text-muted-foreground m-0 mb-2.5 text-xs font-semibold tracking-[0.04em] uppercase">
              {{ t("docs.public.inThisSection") }}
            </h2>
            <ul class="m-0 flex list-none flex-col gap-0.5 p-0">
              <li v-for="c in page.children" :key="c.short_id">
                <button
                  type="button"
                  data-slot="public-doc-child"
                  class="hover:bg-accent flex w-full cursor-pointer items-center gap-2 rounded-md border-0 px-2.5 py-[7px] text-left text-sm text-inherit"
                  @click="open(c.short_id)"
                >
                  <span class="w-5 flex-none text-center">{{ c.icon || "📄" }}</span>
                  <span>{{ c.title || t("docs.tree.untitled") }}</span>
                </button>
              </li>
            </ul>
          </nav>
        </article>
      </template>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

import { Loader2Icon, LockIcon, UnlinkIcon } from "@lucide/vue";

import { unlockShare, visitPublicSpacePage, visitShare, type SharedPage, type ShareState } from "@/api/docs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// The page an anonymous visitor lands on.
//
// It serves both anonymous entrances — a share link and a page of a public
// space — because from the visitor's side they are the same thing: a document
// with no application around it. No sidebar, no editor, no comments, no
// account. Whatever the server did not send is not here to leak.

const route = useRoute();
const router = useRouter();
const { t, locale } = useI18n();

const loading = ref(false);
const unlocking = ref(false);
const wrong = ref(false);
const password = ref("");
const state = ref<ShareState | "">("");
const page = shallowRef<SharedPage | null>(null);

/** A public space addresses pages by id in the URL; a share link by key. */
const shareKey = computed(() => String(route.params.key ?? ""));
const spaceId = computed(() => String(route.params.spaceId ?? ""));
const wantPage = computed(() => String(route.query.page ?? route.params.short ?? ""));

/**
 * The unlock token lives in sessionStorage, keyed by link.
 *
 * Per tab and gone when the tab closes, which matches what a password on a
 * shared link is for. Wrapped because storage throws in a private window
 * rather than returning nothing.
 */
const tokenKey = computed(() => `yuheng.docs.share.${shareKey.value}`);

function readToken(): string {
  try {
    return sessionStorage.getItem(tokenKey.value) ?? "";
  } catch {
    return "";
  }
}

function writeToken(token: string) {
  try {
    sessionStorage.setItem(tokenKey.value, token);
  } catch {
    // A visitor in a locked-down browser types the password once per page
    // instead of once per session. That is a worse experience, not a broken
    // one, and it is not worth failing the visit over.
  }
}

const updated = computed(() => {
  if (!page.value?.updated_at) return "";
  const at = new Date(page.value.updated_at);
  return Number.isNaN(at.getTime()) ? "" : at.toLocaleDateString(locale.value);
});

async function load() {
  loading.value = true;
  try {
    if (spaceId.value) {
      page.value = await visitPublicSpacePage(spaceId.value, wantPage.value);
      state.value = "ok";
      return;
    }
    const res = await visitShare(shareKey.value, {
      page: wantPage.value || undefined,
      unlockToken: readToken() || undefined,
    });
    state.value = res.state;
    page.value = res.page ?? null;
  } catch {
    // The only errors that reach here are a key that was never issued and a
    // deployment with sharing switched off. Both are, from outside, the same
    // thing: there is nothing at this address.
    state.value = "gone";
    page.value = null;
  } finally {
    loading.value = false;
  }
}

async function submitPassword() {
  if (!password.value) return;
  unlocking.value = true;
  wrong.value = false;
  try {
    const res = await unlockShare(shareKey.value, password.value);
    if (res.unlock_token) {
      writeToken(res.unlock_token);
      password.value = "";
      await load();
      return;
    }
    // The link died between loading it and typing the password.
    state.value = res.state;
  } catch {
    wrong.value = true;
  } finally {
    unlocking.value = false;
  }
}

function open(shortId: string) {
  if (spaceId.value) {
    router.push({ name: "docsPublicSpacePage", params: { spaceId: spaceId.value, short: shortId } });
    return;
  }
  router.push({ name: "docsPublicLink", params: { key: shareKey.value }, query: { page: shortId } });
}

watch(
  [shareKey, spaceId, wantPage],
  () => {
    void load();
  },
  { immediate: true },
);
</script>

<style scoped>
/* Server-rendered document markup; scoped host rules only, kept because they
   style HTML this component does not own. */
.doc-body :deep(img),
.doc-body :deep(video) {
  max-width: 100%;
}

.doc-body :deep(table) {
  display: block;
  overflow-x: auto;
  border-collapse: collapse;
}

.doc-body :deep(pre) {
  overflow-x: auto;
  padding: 12px 14px;
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer);
}
</style>
