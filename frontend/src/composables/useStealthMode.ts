import { computed } from 'vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

/**
 * Stealth Mode（隐身/伪装模式）开关
 *
 * - active：管理员后台开启了 stealth_mode_enabled 公共设置
 * - isUserStealth：active 且当前用户不是管理员；这是组件层应该用的判定，
 *   保证管理员任何时候都看到完整 UI（便于运营），仅普通用户视图被精简和中性化
 *
 * 公共设置由 GET /api/public/settings 返回，注入到 cachedPublicSettings；
 * 因此 SSR/首屏阶段就能读到，不会闪烁。
 */
export function useStealthMode() {
  const appStore = useAppStore()
  const authStore = useAuthStore()

  const active = computed(() => appStore.stealthModeEnabled)
  const isUserStealth = computed(() => active.value && !authStore.isAdmin)

  return { active, isUserStealth }
}
