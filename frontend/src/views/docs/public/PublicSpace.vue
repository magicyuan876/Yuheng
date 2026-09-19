<template>
  <div class="public-space">
    <main class="space-shell">
      <t-loading :loading="loading" size="small">
        <section v-if="missing" class="space-gone">
          <t-icon name="link-unlink" size="28px" />
          <h1>{{ t('docs.public.state.gone') }}</h1>
          <p>{{ t('docs.public.stateHint') }}</p>
        </section>

        <template v-else-if="space">
          <header class="space-head">
            <span v-if="space.icon" class="space-icon">{{ space.icon }}</span>
            <h1>{{ space.name }}</h1>
            <p v-if="space.description" class="space-desc">{{ space.description }}</p>
          </header>

          <ul v-if="space.pages.length" class="space-pages">
            <li v-for="p in space.pages" :key="p.short_id">
              <button type="button" class="space-page" @click="open(p.short_id)">
                <span class="page-icon">{{ p.icon || '📄' }}</span>
                <span>{{ p.title || t('docs.tree.untitled') }}</span>
              </button>
            </li>
          </ul>
          <p v-else class="space-empty">{{ t('docs.public.spaceEmpty') }}</p>
        </template>
      </t-loading>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { visitPublicSpace, type PublicSpaceView } from '@/api/docs'

// The front door of a space published to anyone.
//
// Like the document view next to it, there is no application around this: a
// visitor gets the space's name and its top-level pages, and nothing that
// would only make sense to somebody with an account.

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const loading = ref(false)
const missing = ref(false)
const space = shallowRef<PublicSpaceView | null>(null)

const spaceId = computed(() => String(route.params.spaceId ?? ''))

async function load() {
  loading.value = true
  missing.value = false
  space.value = null
  const id = spaceId.value
  try {
    const view = await visitPublicSpace(id)
    if (spaceId.value === id) space.value = view
  } catch {
    // A private space, a space that does not exist, and a deployment with
    // sharing switched off are all the same thing from out here.
    if (spaceId.value === id) missing.value = true
  } finally {
    if (spaceId.value === id) loading.value = false
  }
}

function open(shortId: string) {
  router.push({ name: 'docsPublicSpacePage', params: { spaceId: spaceId.value, short: shortId } })
}

watch(spaceId, () => {
  void load()
}, { immediate: true })
</script>

<style scoped>
.public-space {
  min-height: 100vh;
  background: var(--td-bg-color-page);
}

.space-shell {
  max-width: 720px;
  margin: 0 auto;
  padding-block: 64px 96px;
  padding-inline: 20px;
}

.space-gone {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding-block: 80px;
  color: var(--td-text-color-secondary);
  text-align: center;
}

.space-gone h1 {
  margin: 0;
  color: var(--td-text-color-primary);
  font-size: 20px;
}

.space-gone p {
  margin: 0;
  font-size: 14px;
}

.space-head {
  margin-bottom: 32px;
}

.space-icon {
  display: block;
  margin-bottom: 10px;
  font-size: 44px;
  line-height: 1;
}

.space-head h1 {
  margin: 0;
  font-size: 30px;
  font-weight: 700;
  text-wrap: balance;
}

.space-desc {
  margin: 10px 0 0;
  color: var(--td-text-color-secondary);
  font-size: 15px;
  line-height: 1.6;
}

.space-pages {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.space-page {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  font-size: 15px;
  text-align: left;
  cursor: pointer;
}

.space-page:hover {
  background: var(--td-bg-color-container-hover);
}

.space-empty {
  color: var(--td-text-color-placeholder);
  font-size: 14px;
}
</style>
