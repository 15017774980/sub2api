//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

func TestDisguiseDTOGroupPlatform(t *testing.T) {
	// nil 安全
	disguiseDTOGroupPlatform(nil)

	// deepseek → anthropic
	g := &dto.Group{Platform: service.PlatformDeepSeek}
	disguiseDTOGroupPlatform(g)
	require.Equal(t, service.PlatformAnthropic, g.Platform)

	// 其它平台原样
	for _, p := range []string{service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformGemini, service.PlatformAntigravity} {
		g := &dto.Group{Platform: p}
		disguiseDTOGroupPlatform(g)
		require.Equal(t, p, g.Platform, "platform %s should not be disguised", p)
	}
}

func TestDisguiseDTOAPIKey(t *testing.T) {
	disguiseDTOAPIKey(nil)

	// 无 Group 也不 panic
	k := &dto.APIKey{}
	disguiseDTOAPIKey(k)

	// Group.Platform=deepseek → anthropic
	k = &dto.APIKey{Group: &dto.Group{Platform: service.PlatformDeepSeek}}
	disguiseDTOAPIKey(k)
	require.Equal(t, service.PlatformAnthropic, k.Group.Platform)
}

func TestDisguiseDTOUsageLog(t *testing.T) {
	disguiseDTOUsageLog(nil)

	u := &dto.UsageLog{
		Group:        &dto.Group{Platform: service.PlatformDeepSeek},
		APIKey:       &dto.APIKey{Group: &dto.Group{Platform: service.PlatformDeepSeek}},
		Subscription: &dto.UserSubscription{Group: &dto.Group{Platform: service.PlatformDeepSeek}},
	}
	disguiseDTOUsageLog(u)
	require.Equal(t, service.PlatformAnthropic, u.Group.Platform)
	require.Equal(t, service.PlatformAnthropic, u.APIKey.Group.Platform)
	require.Equal(t, service.PlatformAnthropic, u.Subscription.Group.Platform)
}

func TestDisguiseDTOUser_NestedAPIKeysAndSubscriptions(t *testing.T) {
	disguiseDTOUser(nil)

	u := &dto.User{
		APIKeys: []dto.APIKey{
			{Group: &dto.Group{Platform: service.PlatformDeepSeek}},
			{Group: &dto.Group{Platform: service.PlatformAnthropic}},
		},
		Subscriptions: []dto.UserSubscription{
			{Group: &dto.Group{Platform: service.PlatformDeepSeek}},
			{Group: &dto.Group{Platform: service.PlatformOpenAI}},
		},
	}
	disguiseDTOUser(u)
	// deepseek 改写
	require.Equal(t, service.PlatformAnthropic, u.APIKeys[0].Group.Platform)
	require.Equal(t, service.PlatformAnthropic, u.Subscriptions[0].Group.Platform)
	// 非 deepseek 原样
	require.Equal(t, service.PlatformAnthropic, u.APIKeys[1].Group.Platform)
	require.Equal(t, service.PlatformOpenAI, u.Subscriptions[1].Group.Platform)
}

func TestDisguiseDTORedeemCode(t *testing.T) {
	disguiseDTORedeemCode(nil)

	rc := &dto.RedeemCode{Group: &dto.Group{Platform: service.PlatformDeepSeek}}
	disguiseDTORedeemCode(rc)
	require.Equal(t, service.PlatformAnthropic, rc.Group.Platform)
}
