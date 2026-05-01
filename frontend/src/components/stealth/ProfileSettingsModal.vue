<template>
  <BaseDialog
    :show="show"
    :title="t('stealth.profile.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-6">
      <ProfileInfoCard
        :user="user"
        :linuxdo-enabled="linuxdoOAuthEnabled"
        :oidc-enabled="oidcOAuthEnabled"
        :oidc-provider-name="oidcOAuthProviderName"
        :wechat-enabled="wechatOAuthEnabled"
        :wechat-open-enabled="wechatOAuthOpenEnabled"
        :wechat-mp-enabled="wechatOAuthMPEnabled"
      />

      <ProfilePasswordForm />

      <ProfileBalanceNotifyCard
        v-if="user && balanceLowNotifyEnabled"
        :enabled="user.balance_notify_enabled ?? true"
        :threshold="user.balance_notify_threshold"
        :extra-emails="user.balance_notify_extra_emails ?? []"
        :system-default-threshold="systemDefaultThreshold"
        :user-email="user.email"
      />

      <ProfileTotpCard />
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileBalanceNotifyCard from '@/components/user/profile/ProfileBalanceNotifyCard.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import { isWeChatWebOAuthEnabled } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

interface Props {
  show: boolean
}
interface Emits {
  (e: 'close'): void
}
const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const balanceLowNotifyEnabled = ref(false)
const systemDefaultThreshold = ref(0)
const linuxdoOAuthEnabled = ref(false)
const wechatOAuthEnabled = ref(false)
const wechatOAuthOpenEnabled = ref<boolean | undefined>(undefined)
const wechatOAuthMPEnabled = ref<boolean | undefined>(undefined)
const oidcOAuthEnabled = ref(false)
const oidcOAuthProviderName = ref('OIDC')

let loaded = false
async function loadSettings() {
  if (loaded) return
  loaded = true
  await Promise.all([
    authStore.refreshUser().catch((error) => {
      console.error('Failed to refresh profile:', error)
    }),
    appStore
      .fetchPublicSettings()
      .then((settings) => {
        if (!settings) return
        balanceLowNotifyEnabled.value = settings.balance_low_notify_enabled ?? false
        systemDefaultThreshold.value = settings.balance_low_notify_threshold ?? 0
        linuxdoOAuthEnabled.value = settings.linuxdo_oauth_enabled ?? false
        wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
        wechatOAuthOpenEnabled.value =
          typeof settings.wechat_oauth_open_enabled === 'boolean'
            ? settings.wechat_oauth_open_enabled
            : undefined
        wechatOAuthMPEnabled.value =
          typeof settings.wechat_oauth_mp_enabled === 'boolean'
            ? settings.wechat_oauth_mp_enabled
            : undefined
        oidcOAuthEnabled.value = settings.oidc_oauth_enabled ?? false
        oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
      })
      .catch((error) => {
        console.error('Failed to load settings:', error)
      }),
  ])
}

// 首次打开时再拉数据，避免 mount 时无谓请求
watch(
  () => props.show,
  (open) => {
    if (open) loadSettings()
  }
)
</script>
