import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import './style.css'
import './styles/theme-tokens.css'

function initThemeClass() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark =
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', shouldUseDark)
}

async function bootstrap() {
  // Apply theme class globally before app mount to keep all routes consistent.
  initThemeClass()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Initialize settings from injected config BEFORE mounting (prevents flash)
  // This must happen after pinia is installed but before router and i18n
  const appStore = useAppStore()
  appStore.initFromInjectedConfig()

  // Hydrate auth from localStorage *before* mount so that the very first
  // render of the routed view evaluates `isUserStealth` against the correct
  // `isAdmin` state. Otherwise admin users F5-flicker into stealth shell
  // and back; non-admin users F5 may render <AppLayout> for one frame.
  const authStore = useAuthStore()
  authStore.checkAuth()

  // 同步注入 stealth-user 根 class（color CSS 变量切换），
  // 让首屏 paint 就走正确的色调。后续仍由 App.vue watchEffect 维护。
  const isUserStealthInitial =
    !!appStore.cachedPublicSettings?.stealth_mode_enabled &&
    authStore.user?.role !== 'admin'
  document.documentElement.classList.toggle('stealth-user', isUserStealthInitial)

  // Set document title immediately after config is loaded.
  // 不再硬编码 "Sub2API - AI API Gateway" 后缀，直接使用 site_name；
  // admin 设置 site_name 后即可完全控制浏览器 tab 标题。
  if (appStore.siteName) {
    document.title = appStore.siteName
  }

  await initI18n()

  app.use(router)
  app.use(i18n)

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()
  app.mount('#app')
}

bootstrap()
