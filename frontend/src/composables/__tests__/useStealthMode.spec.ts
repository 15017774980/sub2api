/**
 * useStealthMode 组合式函数单元测试
 *
 * 验证 active 与 isUserStealth 派生关系：
 * - active 取自 appStore.stealthModeEnabled
 * - isUserStealth = active && !isAdmin（admin 永远看到完整 UI）
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/stores/app', () => ({
  useAppStore: vi.fn()
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn()
}))

import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useStealthMode } from '../useStealthMode'

const mockedUseAppStore = vi.mocked(useAppStore)
const mockedUseAuthStore = vi.mocked(useAuthStore)

function setStores(stealthEnabled: boolean, isAdmin: boolean) {
  mockedUseAppStore.mockReturnValue({ stealthModeEnabled: stealthEnabled } as ReturnType<typeof useAppStore>)
  mockedUseAuthStore.mockReturnValue({ isAdmin } as ReturnType<typeof useAuthStore>)
}

describe('useStealthMode', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('stealth 开关关闭时，active 与 isUserStealth 都为 false', () => {
    setStores(false, false)
    const { active, isUserStealth } = useStealthMode()
    expect(active.value).toBe(false)
    expect(isUserStealth.value).toBe(false)
  })

  it('stealth 开关开启 + 普通用户：active 与 isUserStealth 都为 true', () => {
    setStores(true, false)
    const { active, isUserStealth } = useStealthMode()
    expect(active.value).toBe(true)
    expect(isUserStealth.value).toBe(true)
  })

  it('stealth 开关开启 + 管理员：active 为 true 但 isUserStealth 为 false（不影响管理员视图）', () => {
    setStores(true, true)
    const { active, isUserStealth } = useStealthMode()
    expect(active.value).toBe(true)
    expect(isUserStealth.value).toBe(false)
  })
})
