package handler

import "github.com/Wei-Shaw/sub2api/internal/handler/dto"

// 本文件集中维护 user 视角下"上游身份隐藏"的 DTO 改写逻辑。
// 将 deepseek 等真实上游 platform 在出口处统一改写为对外协议族标识（如 anthropic），
// 避免泄露供应链信息给终端用户。Admin 接口必须使用各自的 *Admin DTO 路径，
// 严禁调用本文件函数（admin 需看到真实上游身份用于排障/运维）。

// disguiseDTOGroupPlatform 改写嵌入式 Group.Platform 字段。nil 安全。
func disguiseDTOGroupPlatform(g *dto.Group) {
	if g == nil {
		return
	}
	g.Platform = disguiseUserVisiblePlatform(g.Platform)
}

// disguiseDTOAPIKey 改写 APIKey 嵌入的 Group.Platform。nil 安全。
func disguiseDTOAPIKey(k *dto.APIKey) {
	if k == nil {
		return
	}
	disguiseDTOGroupPlatform(k.Group)
}

// disguiseDTOSubscription 改写 UserSubscription 嵌入的 Group.Platform。nil 安全。
func disguiseDTOSubscription(s *dto.UserSubscription) {
	if s == nil {
		return
	}
	disguiseDTOGroupPlatform(s.Group)
}

// disguiseDTOUsageLog 改写 UsageLog 内所有嵌入的 Group/APIKey/Subscription。nil 安全。
func disguiseDTOUsageLog(u *dto.UsageLog) {
	if u == nil {
		return
	}
	disguiseDTOGroupPlatform(u.Group)
	disguiseDTOAPIKey(u.APIKey)
	disguiseDTOSubscription(u.Subscription)
}

// disguiseDTORedeemCode 改写 RedeemCode 嵌入的 Group.Platform。nil 安全。
func disguiseDTORedeemCode(r *dto.RedeemCode) {
	if r == nil {
		return
	}
	disguiseDTOGroupPlatform(r.Group)
}

// disguiseDTOUser 改写 User 内嵌的 APIKeys / Subscriptions 中所有 Group.Platform。nil 安全。
// 用于 auth/profile 等返回 dto.User 的接口。
func disguiseDTOUser(u *dto.User) {
	if u == nil {
		return
	}
	for i := range u.APIKeys {
		disguiseDTOAPIKey(&u.APIKeys[i])
	}
	for i := range u.Subscriptions {
		disguiseDTOSubscription(&u.Subscriptions[i])
	}
}
