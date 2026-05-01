/**
 * 全局命令面板（⌘K / Ctrl+K）。
 *
 * 单例 reactive state — 所有调用方共享同一组命令。命令在组件 onMounted
 * 时 register、onBeforeUnmount 时 unregister，避免离开页面后旧命令
 * 仍出现在面板里。
 *
 * 仅在 stealth 模式下才会有组件挂载注册命令；admin 永远看不到面板。
 */
import { ref, computed, type Ref, type ComputedRef } from 'vue'

export interface CommandItem {
  /** 命令唯一 id，用于 register/unregister 与高亮选中 */
  id: string
  /** 分组 label，已翻译的字符串（如 "快捷操作"） */
  group: string
  /** 主显示 label，已翻译 */
  label: string
  /** 副信息（如 group 名 / 状态），可选 */
  hint?: string
  /** Icon 名，对应 components/icons/Icon.vue 支持的 name */
  icon?: string
  /** 执行命令；命令面板会先关闭再调用 */
  action: () => void | Promise<void>
  /** 额外搜索关键词（不显示在 UI，但参与 query 匹配） */
  keywords?: string[]
  /** 排序权重，数字越小越靠前。同分组内生效。默认 100 */
  order?: number
}

const open = ref(false)
const query = ref('')
const items = ref<CommandItem[]>([])
const selectedId = ref<string | null>(null)

function normalizeQuery(s: string): string {
  return s.trim().toLowerCase()
}

const filtered: ComputedRef<CommandItem[]> = computed(() => {
  const q = normalizeQuery(query.value)
  const list = q
    ? items.value.filter((item) => {
        const haystack = [item.label, item.group, ...(item.keywords ?? [])]
          .join(' ')
          .toLowerCase()
        return haystack.includes(q)
      })
    : items.value.slice()

  // 稳定排序：先按 order，order 相同按注册顺序（保持 push 顺序）
  return list.sort((a, b) => (a.order ?? 100) - (b.order ?? 100))
})

const groupedItems: ComputedRef<Array<[string, CommandItem[]]>> = computed(() => {
  const groups = new Map<string, CommandItem[]>()
  for (const item of filtered.value) {
    const g = groups.get(item.group) ?? []
    g.push(item)
    groups.set(item.group, g)
  }
  return Array.from(groups.entries())
})

function register(...newItems: CommandItem[]): void {
  for (const item of newItems) {
    const idx = items.value.findIndex((i) => i.id === item.id)
    if (idx >= 0) items.value[idx] = item
    else items.value.push(item)
  }
}

function unregister(...ids: string[]): void {
  items.value = items.value.filter((i) => !ids.includes(i.id))
  if (selectedId.value && ids.includes(selectedId.value)) {
    selectedId.value = filtered.value[0]?.id ?? null
  }
}

function openPalette(): void {
  open.value = true
  query.value = ''
  selectedId.value = filtered.value[0]?.id ?? null
}

function closePalette(): void {
  open.value = false
}

function togglePalette(): void {
  if (open.value) closePalette()
  else openPalette()
}

function moveSelection(delta: 1 | -1): void {
  const list = filtered.value
  if (list.length === 0) {
    selectedId.value = null
    return
  }
  const currentIdx = Math.max(
    0,
    list.findIndex((i) => i.id === selectedId.value)
  )
  const next = (currentIdx + delta + list.length) % list.length
  selectedId.value = list[next].id
}

async function executeSelected(): Promise<void> {
  const target = filtered.value.find((i) => i.id === selectedId.value)
  if (!target) return
  closePalette()
  // 等下一帧让 dialog 关闭动画启动后再执行 action（避免 modal 互相打架）
  await Promise.resolve()
  await target.action()
}

/**
 * Mac 显示 ⌘K，其他平台显示 Ctrl+K。给 trigger 按钮和 hint 用。
 */
export function getShortcutLabel(): string {
  if (typeof navigator === 'undefined') return 'Ctrl+K'
  const isMac = /Mac|iPod|iPhone|iPad/.test(navigator.platform)
  return isMac ? '⌘K' : 'Ctrl+K'
}

/**
 * 是否是触发命令面板的快捷键（⌘K on mac, Ctrl+K elsewhere）。
 */
export function isPaletteShortcut(event: KeyboardEvent): boolean {
  if (event.key !== 'k' && event.key !== 'K') return false
  const isMac = /Mac|iPod|iPhone|iPad/.test(navigator.platform)
  return isMac ? event.metaKey : event.ctrlKey
}

export function useCommandPalette(): {
  open: Ref<boolean>
  query: Ref<string>
  selectedId: Ref<string | null>
  filtered: ComputedRef<CommandItem[]>
  groupedItems: ComputedRef<Array<[string, CommandItem[]]>>
  register: (...items: CommandItem[]) => void
  unregister: (...ids: string[]) => void
  openPalette: () => void
  closePalette: () => void
  togglePalette: () => void
  moveSelection: (delta: 1 | -1) => void
  executeSelected: () => Promise<void>
} {
  return {
    open,
    query,
    selectedId,
    filtered,
    groupedItems,
    register,
    unregister,
    openPalette,
    closePalette,
    togglePalette,
    moveSelection,
    executeSelected,
  }
}
