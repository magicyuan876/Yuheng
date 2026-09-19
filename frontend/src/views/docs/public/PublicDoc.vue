<template>
  <div class="public-doc">
    <main class="doc-shell">
      <t-loading :loading="loading" size="small">
        <!-- A dead link is a thing to render, not an error to throw. Each
             outcome gets its own sentence, because "something went wrong"
             tells the visitor nothing they can act on. -->
        <section v-if="state === 'password'" class="doc-gate">
          <t-icon name="lock-on" size="28px" />
          <h1>{{ t('docs.public.passwordTitle') }}</h1>
          <p>{{ t('docs.public.passwordHint') }}</p>
          <form class="gate-form" @submit.prevent="submitPassword">
            <t-input v-model="password" type="password" :placeholder="t('docs.public.passwordLabel')"
              :status="wrong ? 'error' : undefined" autofocus
              @change="wrong = false" />
            <t-button theme="primary" type="submit" :loading="unlocking" :disabled="!password">
              {{ t('docs.public.unlock') }}
            </t-button>
          </form>
          <p v-if="wrong" class="gate-error">{{ t('docs.public.wrongPassword') }}</p>
        </section>

        <section v-else-if="state && state !== 'ok'" class="doc-gate">
          <t-icon name="link-unlink" size="28px" />
          <h1>{{ t(`docs.public.state.${state}`) }}</h1>
          <p>{{ t('docs.public.stateHint') }}</p>
        </section>

        <article v-else-if="page" class="doc-article">
          <nav v-if="page.breadcrumb.length" class="doc-crumbs">
            <span v-for="crumb in page.breadcrumb" :key="crumb.short_id" class="crumb-item">
              <button type="button" class="crumb" @click="open(crumb.short_id)">
                {{ crumb.title || t('docs.tree.untitled') }}
              </button>
              <span class="crumb-sep" aria-hidden="true">/</span>
            </span>
          </nav>

          <header class="doc-head">
            <span v-if="page.icon" class="doc-icon">{{ page.icon }}</span>
            <h1 class="doc-title">{{ page.title || t('docs.tree.untitled') }}</h1>
            <p class="doc-meta">
              {{ t('docs.public.from', { space: page.space_name }) }}
              <span v-if="page.updated_at"> · {{ t('docs.public.updated', { date: updated }) }}</span>
            </p>
          </header>

          <!-- The server renders the document; it is the same renderer the
               exports use, and it has already dropped everything a visitor
               may not see. -->
          <div class="doc-body prosemirror-host" v-html="page.html" />

          <nav v-if="page.children.length" class="doc-children">
            <h2>{{ t('docs.public.inThisSection') }}</h2>
            <ul>
              <li v-for="c in page.children" :key="c.short_id">
                <button type="button" class="child" @click="open(c.short_id)">
                  <span class="child-icon">{{ c.icon || '📄' }}</span>
                  <span>{{ c.title || t('docs.tree.untitled') }}</span>
                </button>
              </li>
            </ul>
          </nav>
        </article>
      </t-loading>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import {
  unlockShare,
  visitPublicSpacePage,
  visitShare,
  type SharedPage,
  type ShareState,
} from '@/api/docs'

// The page an anonymous visitor lands on.
//
// It serves both anonymous entrances — a share link and a page of a public
// space — because from the visitor's side they are the same thing: a document
// with no application around it. No sidebar, no editor, no comments, no
// account. Whatever the server did not send is not here to leak.

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()

const loading = ref(false)
const unlocking = ref(false)
const wrong = ref(false)
const password = ref('')
const state = ref<ShareState | ''>('')
const page = shallowRef<SharedPage | null>(null)

/** A public space addresses pages by id in the URL; a share link by key. */
const shareKey = computed(() => String(route.params.key ?? ''))
const spaceId = computed(() => String(route.params.spaceId ?? ''))
const wantPage = computed(() => String(route.query.page ?? route.params.short ?? ''))

/**
 * The unlock token lives in sessionStorage, keyed by link.
 *
 * Per tab and gone when the tab closes, which matches what a password on a
 * shared link is for. Wrapped because storage throws in a private window
 * rather than returning nothing.
 */
const tokenKey = computed(() => `yuheng.docs.share.${shareKey.value}`)

function readToken(): string {
  try {
    return sessionStorage.getItem(tokenKey.value) ?? ''
  } catch {
    return ''
  }
}

function writeToken(token: string) {
  try {
    sessionStorage.setItem(tokenKey.value, token)
  } catch {
    // A visitor in a locked-down browser types the password once per page
    // instead of once per session. That is a worse experience, not a broken
    // one, and it is not worth failing the visit over.
  }
}

const updated = computed(() => {
  if (!page.value?.updated_at) return ''
  const at = new Date(page.value.updated_at)
  return Number.isNaN(at.getTime()) ? '' : at.toLocaleDateString(locale.value)
})

async function load() {
  loading.value = true
  try {
    if (spaceId.value) {
      page.value = await visitPublicSpacePage(spaceId.value, wantPage.value)
      state.value = 'ok'
      return
    }
    const res = await visitShare(shareKey.value, {
      page: wantPage.value || undefined,
      unlockToken: readToken() || undefined,
    })
    state.value = res.state
    page.value = res.page ?? null
  } catch {
    // The only errors that reach here are a key that was never issued and a
    // deployment with sharing switched off. Both are, from outside, the same
    // thing: there is nothing at this address.
    state.value = 'gone'
    page.value = null
  } finally {
    loading.value = false
  }
}

async function submitPassword() {
  if (!password.value) return
  unlocking.value = true
  wrong.value = false
  try {
    const res = await unlockShare(shareKey.value, password.value)
    if (res.unlock_token) {
      writeToken(res.unlock_token)
      password.value = ''
      await load()
      return
    }
    // The link died between loading it and typing the password.
    state.value = res.state
  } catch {
    wrong.value = true
  } finally {
    unlocking.value = false
  }
}

function open(shortId: string) {
  if (spaceId.value) {
    router.push({ name: 'docsPublicSpacePage', params: { spaceId: spaceId.value, short: shortId } })
    return
  }
  router.push({ name: 'docsPublicLink', params: { key: shareKey.value }, query: { page: shortId } })
}

watch([shareKey, spaceId, wantPage], () => {
  void load()
}, { immediate: true })
</script>

<style scoped>
.public-doc {
  min-height: 100vh;
  background: var(--td-bg-color-page);
}

.doc-shell {
  max-width: 820px;
  margin: 0 auto;
  padding-block: 48px 96px;
  padding-inline: 20px;
}

.doc-gate {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding-block: 80px;
  color: var(--td-text-color-secondary);
  text-align: center;
}

.doc-gate h1 {
  margin: 0;
  color: var(--td-text-color-primary);
  font-size: 20px;
}

.doc-gate p {
  margin: 0;
  font-size: 14px;
}

.gate-form {
  display: flex;
  gap: 8px;
  width: 100%;
  max-width: 320px;
  margin-top: 8px;
}

.gate-error {
  color: var(--td-error-color);
  font-size: 13px;
}

.doc-crumbs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  margin-bottom: 16px;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.crumb-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.crumb {
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.crumb:hover {
  color: var(--td-brand-color);
}

.doc-head {
  margin-bottom: 28px;
}

.doc-icon {
  display: block;
  margin-bottom: 8px;
  font-size: 40px;
  line-height: 1;
}

.doc-title {
  margin: 0;
  font-size: 32px;
  font-weight: 700;
  line-height: 1.25;
  text-wrap: balance;
}

.doc-meta {
  margin: 8px 0 0;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.doc-body {
  font-size: 15px;
  line-height: 1.75;
}

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

.doc-children {
  margin-top: 48px;
  padding-top: 20px;
  border-top: 1px solid var(--td-component-stroke);
}

.doc-children h2 {
  margin: 0 0 10px;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.doc-children ul {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.child {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 7px 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font-size: 14px;
  text-align: left;
  cursor: pointer;
}

.child:hover {
  background: var(--td-bg-color-container-hover);
}
</style>
