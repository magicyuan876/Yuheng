<template>
  <div class="share-panel">
    <t-loading :loading="loading" size="small">
      <t-alert v-if="!sharingAvailable" theme="info" class="off-alert">
        <template #message>{{ t('docs.share.disabled') }}</template>
      </t-alert>

      <template v-else>
        <ul v-if="links.length" class="link-list">
          <li v-for="link in links" :key="link.id" class="link-row">
            <div class="link-main">
              <button type="button" class="link-url" :title="urlOf(link)" @click="copy(link)">
                <t-icon name="link" size="14px" />
                <span class="link-text">{{ urlOf(link) }}</span>
              </button>
              <div class="link-facts">
                <!-- A link that resolves to nothing is the one fact the owner
                     most needs; it leads. -->
                <t-tag v-if="!link.live" size="small" theme="warning" variant="light">
                  {{ t('docs.share.notLive') }}
                </t-tag>
                <span v-if="link.has_password" class="fact">
                  <t-icon name="lock-on" size="12px" /> {{ t('docs.share.hasPassword') }}
                </span>
                <span v-if="link.include_children" class="fact">
                  <t-icon name="tree-square-dot" size="12px" /> {{ t('docs.share.withChildren') }}
                </span>
                <span v-if="link.expires_at" class="fact">
                  <t-icon name="time" size="12px" /> {{ t('docs.share.expires', { date: dateOf(link.expires_at) }) }}
                </span>
                <span v-if="link.allow_search_index" class="fact">
                  <t-icon name="search" size="12px" /> {{ t('docs.share.indexed') }}
                </span>
                <span class="fact">{{ t('docs.share.views', { n: link.view_count }) }}</span>
              </div>
            </div>
            <t-button v-if="canManage" variant="text" size="small" theme="danger"
              :loading="busy === link.id" @click="confirmRevoke(link)">
              {{ t('docs.share.revoke') }}
            </t-button>
          </li>
        </ul>
        <p v-else-if="!loading" class="link-empty">{{ t('docs.share.none') }}</p>

        <!-- Restricted pages cannot be published at all, and saying why is
             more useful than a disabled button with no explanation. -->
        <t-alert v-if="restricted" theme="warning" class="off-alert">
          <template #message>{{ t('docs.share.restrictedPage') }}</template>
        </t-alert>

        <form v-else-if="canManage" class="new-link" @submit.prevent="create">
          <div class="new-options">
            <t-checkbox v-model="includeChildren">{{ t('docs.share.optionChildren') }}</t-checkbox>
            <t-checkbox v-model="allowIndex">{{ t('docs.share.optionIndex') }}</t-checkbox>
          </div>
          <div class="new-row">
            <t-input v-model="password" type="password" size="small" class="new-password"
              :placeholder="t('docs.share.optionPassword')" :maxlength="64" />
            <t-date-picker v-model="expiresAt" size="small" class="new-expiry" clearable
              :placeholder="t('docs.share.optionExpiry')" :disable-date="disablePast" />
            <t-button theme="primary" size="small" type="submit" :loading="creating">
              {{ t('docs.share.create') }}
            </t-button>
          </div>
        </form>
      </template>
    </t-loading>

    <t-dialog v-model:visible="revokeVisible" :header="t('docs.share.revokeHeader')" theme="danger"
      width="440px" :confirm-btn="{ content: t('docs.share.revoke'), theme: 'danger' }"
      :cancel-btn="t('common.cancel')" @confirm="revoke">
      <p>{{ t('docs.share.revokeBody') }}</p>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import { createShare, listShares, revokeShare, type ShareView } from '@/api/docs'
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities'

// Publishing a page to the internet, and seeing what is already published.
//
// The list is shown to every reader of the page, not only to the people who
// may change it: "this is on the internet" is exactly the kind of fact whose
// audience should be as wide as possible, because the person most likely to
// notice a mistake is not always the person who made it.

const props = defineProps<{ pageId: string; canManage: boolean; restricted: boolean }>()

const { t, locale } = useI18n()
const capabilities = useDeploymentCapabilitiesStore()

/** Whether this deployment allows anything to be published at all. Hiding
 * the control beats offering one whose only outcome is a refusal. */
const sharingAvailable = computed(() => capabilities.isSupported('docs.public_sharing'))

const loading = ref(false)
const creating = ref(false)
const busy = ref('')
const links = shallowRef<ShareView[]>([])

const includeChildren = ref(false)
const allowIndex = ref(false)
const password = ref('')
const expiresAt = ref('')

const revokeVisible = ref(false)
const pending = ref<ShareView | null>(null)

const canManage = computed(() => props.canManage)

/** The address a visitor opens. Built here rather than returned by the
 * server, which does not know how this deployment is addressed from outside. */
function urlOf(link: ShareView): string {
  return `${window.location.origin}/d/${link.key}`
}

function dateOf(iso?: string): string {
  if (!iso) return ''
  const at = new Date(iso)
  return Number.isNaN(at.getTime()) ? '' : at.toLocaleDateString(locale.value)
}

const disablePast = (date: Date) => date.getTime() < Date.now() - 86400000

function fail(err: unknown, fallback: string) {
  const msg = (err as { message?: string } | null)?.message
  void MessagePlugin.error(msg ? `${fallback}: ${msg}` : fallback)
}

async function load() {
  if (!props.pageId) return
  loading.value = true
  const id = props.pageId
  try {
    const rows = await listShares(id)
    if (props.pageId === id) links.value = rows
  } catch (err) {
    fail(err, t('docs.share.loadFailed'))
  } finally {
    if (props.pageId === id) loading.value = false
  }
}

async function create() {
  creating.value = true
  try {
    const made = await createShare(props.pageId, {
      include_children: includeChildren.value,
      allow_search_index: allowIndex.value,
      password: password.value || undefined,
      expires_at: expiresAt.value ? new Date(expiresAt.value).toISOString() : undefined,
    })
    links.value = [made, ...links.value]
    password.value = ''
    expiresAt.value = ''
    await copy(made)
  } catch (err) {
    fail(err, t('docs.share.createFailed'))
  } finally {
    creating.value = false
  }
}

function confirmRevoke(link: ShareView) {
  pending.value = link
  revokeVisible.value = true
}

async function revoke() {
  const link = pending.value
  revokeVisible.value = false
  if (!link) return
  busy.value = link.id
  try {
    await revokeShare(props.pageId, link.id)
    links.value = links.value.filter((l) => l.id !== link.id)
  } catch (err) {
    fail(err, t('docs.share.revokeFailed'))
  } finally {
    busy.value = ''
  }
}

async function copy(link: ShareView) {
  try {
    await navigator.clipboard.writeText(urlOf(link))
    void MessagePlugin.success(t('docs.share.copied'))
  } catch {
    // Clipboard access is refused in plenty of ordinary situations (an
    // insecure origin, a permissions policy). The URL is on screen and
    // selectable, so this is a convenience that failed, not the feature.
    void MessagePlugin.info(t('docs.share.copyManually'))
  }
}

watch(() => props.pageId, () => {
  void load()
}, { immediate: true })
</script>

<style scoped>
.share-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 80px;
}

.off-alert {
  margin: 0;
}

.link-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.link-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.link-main {
  flex: 1;
  min-width: 0;
}

.link-url {
  display: flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--td-brand-color);
  font-size: 13px;
  cursor: pointer;
}

.link-text {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.link-facts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.fact {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.link-empty {
  margin: 0;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.new-link {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--td-component-stroke);
}

.new-options {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.new-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.new-password {
  flex: 1;
  min-width: 140px;
}

.new-expiry {
  width: 160px;
}
</style>
