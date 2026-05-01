<template>
  <button
    type="button"
    class="group inline-flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm text-gray-500 shadow-sm transition-all hover:border-gray-300 hover:bg-gray-50 hover:text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-400 dark:hover:border-dark-600 dark:hover:bg-dark-800"
    :title="t('stealth.commandPalette.placeholder')"
    @click="openPalette"
  >
    <Icon name="search" size="sm" class="text-gray-400" />
    <span class="hidden sm:inline">{{ t('stealth.commandPalette.placeholder') }}</span>
    <kbd
      class="ml-1 rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400"
    >
      {{ shortcut }}
    </kbd>
  </button>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useCommandPalette, getShortcutLabel } from '@/composables/useCommandPalette'

const { t } = useI18n()
const { openPalette } = useCommandPalette()
const shortcut = getShortcutLabel()
</script>
