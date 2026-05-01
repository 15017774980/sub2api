<template>
  <div class="space-y-10">
    <!-- Hero -->
    <section>
      <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">
        {{ greeting }}
      </h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('stealth.home.balanceLabel') }} ·
        <span class="font-medium text-gray-900 dark:text-gray-100">${{ balance.toFixed(2) }}</span>
      </p>
    </section>

    <!-- Loading state -->
    <div v-if="loading" class="flex items-center justify-center py-16 text-gray-400">
      <Icon name="refresh" size="md" class="animate-spin" />
    </div>

    <!-- No groups available -->
    <div
      v-else-if="groups.length === 0"
      class="rounded-2xl border border-amber-200 bg-amber-50 px-6 py-8 text-center dark:border-amber-900/40 dark:bg-amber-900/10"
    >
      <p class="text-sm text-amber-800 dark:text-amber-300">
        {{ t('stealth.home.noGroups') }}
      </p>
    </div>

    <!-- Group selector (chips) + Key card -->
    <template v-else>
      <section class="space-y-3">
        <label class="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
          {{ t('stealth.home.groupLabel') }}
        </label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="g in groups"
            :key="g.id"
            type="button"
            :class="[
              'rounded-xl border px-3 py-2 text-left transition-all',
              g.id === selectedGroupId
                ? 'border-primary-500 bg-primary-50 ring-1 ring-primary-500 dark:bg-primary-900/20'
                : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600 dark:hover:bg-dark-800',
            ]"
            @click="selectGroup(g.id)"
          >
            <GroupBadge
              :name="g.name"
              :platform="g.platform"
              :subscription-type="g.subscription_type"
              :rate-multiplier="g.rate_multiplier"
              :user-rate-multiplier="userGroupRates[g.id] ?? null"
              always-show-rate
            />
          </button>
        </div>
      </section>

      <!-- Key card -->
      <section class="space-y-3">
        <label class="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
          {{ t('stealth.home.keyLabel') }}
        </label>

        <div
          class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
        >
          <div v-if="creatingKey" class="flex items-center gap-3 py-3 text-gray-400">
            <Icon name="refresh" size="sm" class="animate-spin" />
            <span class="text-sm">{{ t('stealth.home.generating') }}</span>
          </div>

          <template v-else-if="currentKey">
            <code
              class="block break-all rounded-lg bg-gray-50 px-4 py-3 font-mono text-sm text-gray-800 dark:bg-dark-800 dark:text-gray-100"
            >
              {{ currentKey.key }}
            </code>

            <div class="mt-4 flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="inline-flex items-center gap-1.5 rounded-lg bg-primary-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-700"
                @click="copyKey"
              >
                <Icon :name="justCopied ? 'check' : 'copy'" size="sm" />
                {{ justCopied ? t('stealth.home.copied') : t('stealth.home.copy') }}
              </button>

              <button
                type="button"
                class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200 dark:hover:bg-dark-800"
                @click="openTutorial"
              >
                <Icon name="book" size="sm" class="text-gray-400" />
                {{ t('stealth.home.tutorial') }}
              </button>

              <button
                type="button"
                class="ml-auto inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200 dark:hover:bg-dark-800"
                :disabled="refreshing"
                @click="onRefreshClick"
              >
                <Icon name="refresh" size="sm" :class="['text-gray-400', refreshing ? 'animate-spin' : '']" />
                {{ t('stealth.home.refresh') }}
              </button>
            </div>
          </template>
        </div>
      </section>

      <!-- Redeem section -->
      <section class="space-y-3">
        <label class="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
          {{ t('stealth.home.redeemLabel') }}
        </label>
        <form class="flex flex-col gap-2 sm:flex-row" @submit.prevent="onRedeem">
          <input
            v-model="redeemCode"
            type="text"
            :placeholder="t('stealth.home.redeemPlaceholder')"
            class="flex-1 rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-mono text-gray-900 outline-none placeholder:font-sans placeholder:text-gray-400 focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-100"
            :disabled="redeeming"
            spellcheck="false"
          />
          <button
            type="submit"
            class="inline-flex items-center justify-center gap-1.5 rounded-lg bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:opacity-50 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
            :disabled="!redeemCode.trim() || redeeming"
          >
            <Icon v-if="redeeming" name="refresh" size="sm" class="animate-spin" />
            {{ t('stealth.home.redeem') }}
          </button>
        </form>
      </section>

      <!-- Footer hint -->
      <p class="pt-4 text-center text-xs text-gray-400 dark:text-gray-500">
        {{ t('stealth.home.hint', { shortcut }) }}
      </p>
    </template>

    <!-- Refresh confirm dialog -->
    <ConfirmDialog
      :show="confirmRefresh"
      :title="t('stealth.home.refreshConfirmTitle')"
      :message="t('stealth.home.refreshConfirmMessage')"
      :confirm-text="t('stealth.home.refresh')"
      danger
      @confirm="doRefreshKey"
      @cancel="confirmRefresh = false"
    />

    <!-- Use key tutorial modal -->
    <UseKeyModal
      v-if="currentKey"
      :show="showTutorial"
      :api-key="currentKey.key"
      :base-url="apiBaseUrl"
      :platform="currentGroupPlatform"
      :allow-messages-dispatch="currentGroup?.allow_messages_dispatch || false"
      @close="showTutorial = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import keysAPI from '@/api/keys'
import userGroupsAPI from '@/api/groups'
import redeemAPI from '@/api/redeem'
import type { ApiKey, Group, GroupPlatform } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import UseKeyModal from '@/components/keys/UseKeyModal.vue'
import {
  useCommandPalette,
  getShortcutLabel,
  type CommandItem,
} from '@/composables/useCommandPalette'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { register, unregister, openPalette } = useCommandPalette()

const shortcut = getShortcutLabel()
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || '')

// === State ===
const loading = ref(true)
const groups = ref<Group[]>([])
const keys = ref<ApiKey[]>([])
const userGroupRates = ref<Record<number, number>>({})
const selectedGroupId = ref<number | null>(null)
const creatingKey = ref(false)
const refreshing = ref(false)
const confirmRefresh = ref(false)
const justCopied = ref(false)
const redeemCode = ref('')
const redeeming = ref(false)
const showTutorial = ref(false)

// === Derived ===
const greeting = computed(() => {
  const name = authStore.user?.username || authStore.user?.email || ''
  return t('stealth.home.welcome', { name })
})
const balance = computed(() => authStore.user?.balance ?? 0)

const currentGroup = computed<Group | undefined>(() =>
  groups.value.find((g) => g.id === selectedGroupId.value)
)

const currentGroupPlatform = computed<GroupPlatform | null>(
  () => currentGroup.value?.platform ?? null
)

const currentKey = computed<ApiKey | null>(() => {
  if (!selectedGroupId.value) return null
  // 该 group 下用户最近一条 key（按 created_at 降序）
  const list = keys.value
    .filter((k) => k.group_id === selectedGroupId.value)
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  return list[0] ?? null
})

// === Lifecycle ===
async function loadGroupsAndKeys(): Promise<void> {
  loading.value = true
  try {
    const [groupList, keyResp, rates] = await Promise.all([
      userGroupsAPI.getAvailable(),
      keysAPI.list(1, 100, { sort_by: 'created_at', sort_order: 'desc' }),
      userGroupsAPI.getUserGroupRates(),
    ])
    groups.value = groupList
    keys.value = keyResp.items
    userGroupRates.value = rates
    if (groupList.length > 0 && !selectedGroupId.value) {
      // 默认选第一个 group
      selectedGroupId.value = groupList[0].id
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('stealth.home.loadFailed')))
  } finally {
    loading.value = false
  }
}

// 切 group 后若该 group 下没有 key，自动创建一个
watch(selectedGroupId, async (newId) => {
  if (newId == null) return
  if (creatingKey.value) return
  const existing = keys.value.find((k) => k.group_id === newId)
  if (!existing) {
    await autoCreateKey(newId)
  }
})

async function autoCreateKey(groupId: number): Promise<void> {
  if (creatingKey.value) return
  creatingKey.value = true
  try {
    const created = await keysAPI.create(`auto-${Date.now()}`, groupId)
    keys.value.push(created)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('stealth.home.createFailed')))
  } finally {
    creatingKey.value = false
  }
}

function selectGroup(id: number) {
  selectedGroupId.value = id
}

// === Copy ===
async function copyKey(): Promise<void> {
  if (!currentKey.value) return
  try {
    await navigator.clipboard.writeText(currentKey.value.key)
    justCopied.value = true
    setTimeout(() => (justCopied.value = false), 1500)
  } catch (error) {
    appStore.showError(t('stealth.home.copyFailed'))
  }
}

// === Tutorial ===
function openTutorial() {
  if (!currentKey.value) return
  showTutorial.value = true
}

// === Refresh ===
function onRefreshClick() {
  if (!currentKey.value) return
  confirmRefresh.value = true
}

async function doRefreshKey(): Promise<void> {
  confirmRefresh.value = false
  if (!currentKey.value || !selectedGroupId.value) return
  refreshing.value = true
  const oldId = currentKey.value.id
  const groupId = selectedGroupId.value
  try {
    await keysAPI.delete(oldId)
    keys.value = keys.value.filter((k) => k.id !== oldId)
    const created = await keysAPI.create(`auto-${Date.now()}`, groupId)
    keys.value.push(created)
    appStore.showSuccess(t('stealth.home.refreshed'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('stealth.home.refreshFailed')))
    // 失败时尝试重新拉取列表，避免本地状态错乱
    await loadGroupsAndKeys()
  } finally {
    refreshing.value = false
  }
}

// === Redeem ===
async function onRedeem(): Promise<void> {
  const code = redeemCode.value.trim()
  if (!code || redeeming.value) return
  redeeming.value = true
  try {
    const resp = await redeemAPI.redeem(code)
    if (typeof resp.new_balance === 'number' && authStore.user) {
      authStore.user.balance = resp.new_balance
    }
    redeemCode.value = ''
    appStore.showSuccess(resp.message || t('stealth.home.redeemSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('stealth.home.redeemFailed')))
  } finally {
    redeeming.value = false
  }
}

// === Command palette wiring ===
function buildCommands(): CommandItem[] {
  const items: CommandItem[] = [
    {
      id: 'cmd:copy-key',
      group: t('stealth.commandPalette.groups.actions'),
      label: t('stealth.commandPalette.commands.copyKey'),
      icon: 'copy',
      order: 10,
      action: () => copyKey(),
    },
    {
      id: 'cmd:tutorial',
      group: t('stealth.commandPalette.groups.actions'),
      label: t('stealth.commandPalette.commands.tutorial'),
      icon: 'book',
      order: 20,
      action: () => openTutorial(),
    },
    {
      id: 'cmd:refresh',
      group: t('stealth.commandPalette.groups.actions'),
      label: t('stealth.commandPalette.commands.refresh'),
      icon: 'refresh',
      order: 30,
      action: () => onRefreshClick(),
    },
    {
      id: 'cmd:redeem-focus',
      group: t('stealth.commandPalette.groups.actions'),
      label: t('stealth.commandPalette.commands.redeem'),
      icon: 'gift',
      order: 40,
      action: () => {
        // 滚动到 redeem 输入框并 focus
        const input = document.querySelector<HTMLInputElement>('input[placeholder]')
        input?.focus()
      },
    },
  ]
  // 切 group 命令（每个 group 一条）
  for (const g of groups.value) {
    items.push({
      id: `cmd:group:${g.id}`,
      group: t('stealth.commandPalette.groups.switchGroup'),
      label: g.name,
      hint: g.id === selectedGroupId.value ? t('stealth.commandPalette.current') : undefined,
      order: 50 + g.id,
      keywords: [g.name, 'group'],
      action: () => {
        selectedGroupId.value = g.id
      },
    })
  }
  return items
}

function syncCommands() {
  // 先全清后注册（简单可靠，命令量小）
  const all = buildCommands()
  register(...all)
}

watch([groups, selectedGroupId], syncCommands)

// === Mount / Unmount ===
onMounted(async () => {
  await loadGroupsAndKeys()
  // 首次：如果 selectedGroup 下没有 key，自动创建
  if (selectedGroupId.value != null && !currentKey.value) {
    await autoCreateKey(selectedGroupId.value)
  }
  syncCommands()
})

onBeforeUnmount(() => {
  // 清理本组件注册的命令
  const ids = [
    'cmd:copy-key',
    'cmd:tutorial',
    'cmd:refresh',
    'cmd:redeem-focus',
    ...groups.value.map((g) => `cmd:group:${g.id}`),
  ]
  unregister(...ids)
})

// 暴露给父组件（StealthShell 的 open-account 事件）— 实际未用，暂保留接口
defineExpose({ openPalette })
</script>
