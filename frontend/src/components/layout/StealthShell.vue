<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Top bar -->
    <header
      class="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-gray-100 bg-white/80 px-4 backdrop-blur-md dark:border-dark-800 dark:bg-dark-900/80 sm:px-6"
    >
      <div class="flex items-center gap-3">
        <CommandPaletteTrigger />
      </div>

      <div class="flex items-center gap-2">
        <span
          class="hidden text-sm font-medium text-gray-500 dark:text-gray-400 sm:inline"
        >
          {{ siteName }}
        </span>

        <!-- User menu -->
        <div v-if="user" class="relative" ref="dropdownRef">
          <button
            type="button"
            class="flex items-center gap-2 rounded-full p-1 transition-colors hover:bg-gray-100 dark:hover:bg-dark-800"
            :aria-label="t('stealth.userMenu.label')"
            @click="dropdownOpen = !dropdownOpen"
          >
            <div
              class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-primary-500 to-primary-600 text-sm font-medium text-white"
            >
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                :alt="displayName"
                class="h-full w-full object-cover"
              />
              <span v-else>{{ userInitials }}</span>
            </div>
          </button>

          <transition name="dropdown">
            <div
              v-if="dropdownOpen"
              class="absolute right-0 mt-2 w-56 overflow-hidden rounded-xl border border-gray-100 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-900"
            >
              <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-800">
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ displayName }}
                </div>
                <div class="truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ user.email }}
                </div>
              </div>

              <div class="py-1">
                <button
                  type="button"
                  class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-dark-800"
                  @click="onAccountClick"
                >
                  <Icon name="cog" size="sm" class="text-gray-400" />
                  {{ t('stealth.userMenu.account') }}
                </button>
              </div>

              <div class="border-t border-gray-100 py-1 dark:border-dark-800">
                <button
                  type="button"
                  class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                  @click="onLogout"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15M12 9l-3 3m0 0l3 3m-3-3h12.75"
                    />
                  </svg>
                  {{ t('stealth.userMenu.signOut') }}
                </button>
              </div>
            </div>
          </transition>
        </div>
      </div>
    </header>

    <!-- Main content -->
    <main class="mx-auto max-w-3xl px-4 py-12 sm:px-6">
      <slot />
    </main>

    <!-- Global command palette -->
    <CommandPalette />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import CommandPaletteTrigger from '@/components/stealth/CommandPaletteTrigger.vue'
import CommandPalette from '@/components/stealth/CommandPalette.vue'

interface Emits {
  (e: 'open-account'): void
}
const emit = defineEmits<Emits>()

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const user = computed(() => authStore.user)
const siteName = computed(() => appStore.siteName || 'API Gateway')
const avatarUrl = computed(() => user.value?.avatar_url || '')
const displayName = computed(() => user.value?.username || user.value?.email || '')
const userInitials = computed(() => {
  const name = displayName.value
  if (!name) return '?'
  const parts = name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
})

const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

function closeDropdown() {
  dropdownOpen.value = false
}

function onAccountClick() {
  closeDropdown()
  emit('open-account')
}

async function onLogout() {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function onClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', onClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 0.12s ease, transform 0.12s ease;
}
.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
