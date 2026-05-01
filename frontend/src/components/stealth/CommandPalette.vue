<template>
  <Teleport to="body">
    <Transition name="cmd-fade">
      <div
        v-if="open"
        class="fixed inset-0 z-[100] flex items-start justify-center bg-gray-900/40 px-4 pt-[16vh] backdrop-blur-sm dark:bg-black/60"
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        @click.self="closePalette"
      >
        <div
          class="w-full max-w-xl overflow-hidden rounded-2xl bg-white shadow-2xl ring-1 ring-gray-200 dark:bg-dark-900 dark:ring-dark-700"
          @click.stop
        >
          <!-- Search input -->
          <div class="flex items-center gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-800">
            <Icon name="search" size="md" class="text-gray-400" />
            <input
              ref="inputRef"
              v-model="query"
              type="text"
              :placeholder="t('stealth.commandPalette.placeholder')"
              autocomplete="off"
              spellcheck="false"
              class="flex-1 bg-transparent text-sm text-gray-900 placeholder-gray-400 outline-none dark:text-gray-100"
              @keydown.down.prevent="moveSelection(1)"
              @keydown.up.prevent="moveSelection(-1)"
              @keydown.enter.prevent="executeSelected()"
              @keydown.esc.prevent="closePalette"
            />
            <kbd class="rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400">
              Esc
            </kbd>
          </div>

          <!-- Command list -->
          <ul
            v-if="filtered.length > 0"
            class="max-h-[60vh] overflow-y-auto py-2"
            role="listbox"
          >
            <template v-for="[group, list] in groupedItems" :key="group">
              <li
                class="px-4 pb-1 pt-2 text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500"
              >
                {{ group }}
              </li>
              <li
                v-for="item in list"
                :key="item.id"
                role="option"
                :aria-selected="item.id === selectedId"
                :class="[
                  'mx-2 flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors',
                  item.id === selectedId
                    ? 'bg-primary-50 text-primary-900 dark:bg-primary-900/30 dark:text-primary-100'
                    : 'text-gray-700 dark:text-gray-200',
                ]"
                @click="executeItem(item)"
                @mousemove="selectedId = item.id"
              >
                <Icon
                  v-if="item.icon"
                  :name="item.icon as any"
                  size="sm"
                  :class="
                    item.id === selectedId
                      ? 'text-primary-600 dark:text-primary-300'
                      : 'text-gray-400'
                  "
                />
                <span class="flex-1 truncate">{{ item.label }}</span>
                <span
                  v-if="item.hint"
                  class="text-xs text-gray-400 dark:text-gray-500"
                >
                  {{ item.hint }}
                </span>
              </li>
            </template>
          </ul>

          <!-- Empty state -->
          <div
            v-else
            class="px-4 py-10 text-center text-sm text-gray-400 dark:text-gray-500"
          >
            {{ t('stealth.commandPalette.empty') }}
          </div>

          <!-- Footer hint -->
          <div
            class="flex items-center justify-between border-t border-gray-100 bg-gray-50/50 px-4 py-2 text-[11px] text-gray-400 dark:border-dark-800 dark:bg-dark-800/50 dark:text-gray-500"
          >
            <span class="flex items-center gap-1">
              <kbd class="rounded border border-gray-200 bg-white px-1 py-0.5 text-[10px] dark:border-dark-700 dark:bg-dark-900">↑</kbd>
              <kbd class="rounded border border-gray-200 bg-white px-1 py-0.5 text-[10px] dark:border-dark-700 dark:bg-dark-900">↓</kbd>
              {{ t('stealth.commandPalette.hint.navigate') }}
            </span>
            <span class="flex items-center gap-1">
              <kbd class="rounded border border-gray-200 bg-white px-1 py-0.5 text-[10px] dark:border-dark-700 dark:bg-dark-900">⏎</kbd>
              {{ t('stealth.commandPalette.hint.select') }}
            </span>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import {
  useCommandPalette,
  isPaletteShortcut,
  type CommandItem,
} from '@/composables/useCommandPalette'

const { t } = useI18n()
const {
  open,
  query,
  selectedId,
  filtered,
  groupedItems,
  moveSelection,
  executeSelected,
  closePalette,
  togglePalette,
} = useCommandPalette()

const inputRef = ref<HTMLInputElement | null>(null)

// 打开时聚焦 input；输入时随匹配结果重置选中
watch(open, async (isOpen) => {
  if (!isOpen) return
  await nextTick()
  inputRef.value?.focus()
})

watch(query, () => {
  selectedId.value = filtered.value[0]?.id ?? null
})

watch(filtered, (list) => {
  if (!list.find((i) => i.id === selectedId.value)) {
    selectedId.value = list[0]?.id ?? null
  }
})

async function executeItem(item: CommandItem): Promise<void> {
  selectedId.value = item.id
  closePalette()
  await Promise.resolve()
  await item.action()
}

// 全局快捷键监听（⌘K / Ctrl+K toggle）
function onGlobalKey(event: KeyboardEvent) {
  if (isPaletteShortcut(event)) {
    event.preventDefault()
    togglePalette()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onGlobalKey)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onGlobalKey)
})
</script>

<style scoped>
.cmd-fade-enter-active,
.cmd-fade-leave-active {
  transition: opacity 0.15s ease;
}
.cmd-fade-enter-from,
.cmd-fade-leave-to {
  opacity: 0;
}
.cmd-fade-enter-active > div,
.cmd-fade-leave-active > div {
  transition: transform 0.15s ease;
}
.cmd-fade-enter-from > div,
.cmd-fade-leave-to > div {
  transform: translateY(-8px);
}
</style>
