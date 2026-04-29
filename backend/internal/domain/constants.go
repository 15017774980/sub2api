package domain

// Status constants
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusError    = "error"
	StatusUnused   = "unused"
	StatusUsed     = "used"
	StatusExpired  = "expired"
)

// Role constants
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Platform constants
const (
	PlatformAnthropic   = "anthropic"
	PlatformOpenAI      = "openai"
	PlatformGemini      = "gemini"
	PlatformAntigravity = "antigravity"
	PlatformDeepSeek    = "deepseek"
	PlatformKimi        = "kimi"
	PlatformMiMo        = "mimo"
)

// DeepSeek model constants（仅用于网关 → DeepSeek 上游的真实模型名）
const (
	DeepSeekModelFlash = "deepseek-v4-flash"
	DeepSeekModelPro   = "deepseek-v4-pro"
)

// DeepSeek upstream defaults
const (
	DeepSeekDefaultBaseURL = "https://api.deepseek.com/anthropic"
)

// Kimi (Moonshot) model constants（仅用于网关 → Kimi 上游的真实模型名）
const (
	KimiModelTop      = "kimi-k2.6"          // 主力 (2026/04 GA，SWE-Bench Pro 58.6%)
	KimiModelMain     = "kimi-k2.5"          // 稳定主力
	KimiModelThinking = "kimi-k2-thinking"   // 原生 reasoning + 长程 tool calling
)

// Kimi upstream defaults
const (
	KimiDefaultBaseURL = "https://api.moonshot.ai/anthropic"
)

// Xiaomi MiMo model constants（仅用于网关 → MiMo 上游的真实模型名）
const (
	MiMoModelPro  = "mimo-v2-pro"  // 主力推理(1M context)
	MiMoModelOmni = "mimo-v2-omni" // multimodal(支持 image，仅此一个)
)

// MiMo upstream defaults
const (
	MiMoDefaultBaseURL = "https://api.xiaomimimo.com/anthropic"
)

// OpenAI model constants（用于 Anthropic 协议入站 → OpenAI 上游的默认映射目标）
//
// 实测：ChatGPT Plus 订阅 codex 后端（chatgpt.com/backend-api/codex/responses）
// 接受通用 gpt-5.x 模型名（与 Codex CLI 实际使用一致），无需专门用 gpt-5.3-codex 系列。
// normalizeCodexModel 的兜底默认即为 gpt-5.4（见 openai_codex_transform.go:392）。
const (
	OpenAIModelTop  = "gpt-5.5"      // 顶级，对位 Claude Opus
	OpenAIModelMain = "gpt-5.4"      // 主力，对位 Claude Sonnet（与 Codex CLI 默认一致）
	OpenAIModelMini = "gpt-5.4-mini" // 经济，对位 Claude Haiku
)

// Account type constants
const (
	AccountTypeOAuth      = "oauth"       // OAuth类型账号（full scope: profile + inference）
	AccountTypeSetupToken = "setup-token" // Setup Token类型账号（inference only scope）
	AccountTypeAPIKey     = "apikey"      // API Key类型账号
	AccountTypeUpstream   = "upstream"    // 上游透传类型账号（通过 Base URL + API Key 连接上游）
	AccountTypeBedrock    = "bedrock"     // AWS Bedrock 类型账号（通过 SigV4 签名或 API Key 连接 Bedrock，由 credentials.auth_mode 区分）
)

// Redeem type constants
const (
	RedeemTypeBalance      = "balance"
	RedeemTypeConcurrency  = "concurrency"
	RedeemTypeSubscription = "subscription"
	RedeemTypeInvitation   = "invitation"
)

// PromoCode status constants
const (
	PromoCodeStatusActive   = "active"
	PromoCodeStatusDisabled = "disabled"
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminBalance     = "admin_balance"     // 管理员调整余额
	AdjustmentTypeAdminConcurrency = "admin_concurrency" // 管理员调整并发数
)

// Group subscription type constants
const (
	SubscriptionTypeStandard     = "standard"     // 标准计费模式（按余额扣费）
	SubscriptionTypeSubscription = "subscription" // 订阅模式（按限额控制）
)

// Subscription status constants
const (
	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusSuspended = "suspended"
)

// DefaultAntigravityModelMapping 是 Antigravity 平台的默认模型映射
// 当账号未配置 model_mapping 时使用此默认值
// 与前端 useModelWhitelist.ts 中的 antigravityDefaultMappings 保持一致
var DefaultAntigravityModelMapping = map[string]string{
	// Claude 白名单
	"claude-opus-4-7":            "claude-opus-4-7",          // 官方模型
	"claude-opus-4-6-thinking":   "claude-opus-4-6-thinking", // 官方模型
	"claude-opus-4-6":            "claude-opus-4-6-thinking", // 简称映射
	"claude-opus-4-5-thinking":   "claude-opus-4-6-thinking", // 迁移旧模型
	"claude-sonnet-4-6":          "claude-sonnet-4-6",
	"claude-sonnet-4-5":          "claude-sonnet-4-5",
	"claude-sonnet-4-5-thinking": "claude-sonnet-4-5-thinking",
	// Claude 详细版本 ID 映射
	"claude-opus-4-5-20251101":   "claude-opus-4-6-thinking", // 迁移旧模型
	"claude-sonnet-4-5-20250929": "claude-sonnet-4-5",
	// Claude Haiku → Sonnet（无 Haiku 支持）
	"claude-haiku-4-5":          "claude-sonnet-4-6",
	"claude-haiku-4-5-20251001": "claude-sonnet-4-6",
	// Gemini 2.5 白名单
	"gemini-2.5-flash":               "gemini-2.5-flash",
	"gemini-2.5-flash-image":         "gemini-2.5-flash-image",
	"gemini-2.5-flash-image-preview": "gemini-2.5-flash-image",
	"gemini-2.5-flash-lite":          "gemini-2.5-flash-lite",
	"gemini-2.5-flash-thinking":      "gemini-2.5-flash-thinking",
	"gemini-2.5-pro":                 "gemini-2.5-pro",
	// Gemini 3 白名单
	"gemini-3-flash":    "gemini-3-flash",
	"gemini-3-pro-high": "gemini-3-pro-high",
	"gemini-3-pro-low":  "gemini-3-pro-low",
	// Gemini 3 preview 映射
	"gemini-3-flash-preview": "gemini-3-flash",
	"gemini-3-pro-preview":   "gemini-3-pro-high",
	// Gemini 3.1 白名单
	"gemini-3.1-pro-high": "gemini-3.1-pro-high",
	"gemini-3.1-pro-low":  "gemini-3.1-pro-low",
	// Gemini 3.1 preview 映射
	"gemini-3.1-pro-preview": "gemini-3.1-pro-high",
	// Gemini 3.1 image 白名单
	"gemini-3.1-flash-image": "gemini-3.1-flash-image",
	// Gemini 3.1 image preview 映射
	"gemini-3.1-flash-image-preview": "gemini-3.1-flash-image",
	// Gemini 3 image 兼容映射（向 3.1 image 迁移）
	"gemini-3-pro-image":         "gemini-3.1-flash-image",
	"gemini-3-pro-image-preview": "gemini-3.1-flash-image",
	// 其他官方模型
	"gpt-oss-120b-medium":    "gpt-oss-120b-medium",
	"tab_flash_lite_preview": "tab_flash_lite_preview",
}

// DefaultBedrockModelMapping 是 AWS Bedrock 平台的默认模型映射
// 将 Anthropic 标准模型名映射到 Bedrock 模型 ID
// 注意：此处的 "us." 前缀仅为默认值，ResolveBedrockModelID 会根据账号配置的
// aws_region 自动调整为匹配的区域前缀（如 eu.、apac.、jp. 等）
var DefaultBedrockModelMapping = map[string]string{
	// Claude Opus
	"claude-opus-4-7":          "us.anthropic.claude-opus-4-7-v1",
	"claude-opus-4-6-thinking": "us.anthropic.claude-opus-4-6-v1",
	"claude-opus-4-6":          "us.anthropic.claude-opus-4-6-v1",
	"claude-opus-4-5-thinking": "us.anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-5-20251101": "us.anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-1":          "us.anthropic.claude-opus-4-1-20250805-v1:0",
	"claude-opus-4-20250514":   "us.anthropic.claude-opus-4-20250514-v1:0",
	// Claude Sonnet
	"claude-sonnet-4-6-thinking": "us.anthropic.claude-sonnet-4-6",
	"claude-sonnet-4-6":          "us.anthropic.claude-sonnet-4-6",
	"claude-sonnet-4-5":          "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-5-thinking": "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-5-20250929": "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-20250514":   "us.anthropic.claude-sonnet-4-20250514-v1:0",
	// Claude Haiku
	"claude-haiku-4-5":          "us.anthropic.claude-haiku-4-5-20251001-v1:0",
	"claude-haiku-4-5-20251001": "us.anthropic.claude-haiku-4-5-20251001-v1:0",
}

// DefaultDeepSeekModelMapping 是 DeepSeek 平台的默认模型映射（Anthropic 协议入站 → DeepSeek 真实模型名）。
// 端点：https://api.deepseek.com/anthropic/v1/messages
//
// 映射策略（按价位对位）：
//   - Haiku  → flash（廉价对廉价）
//   - Sonnet → flash（默认主力，保留成本上限；如需提质改 pro）
//   - Opus   → pro（高端对高端）
//
// 价格风险提醒：deepseek-v4-pro 当前为 2.5 折优惠期（截止 2026/05/31 23:59），
// 6/1 起恢复原价（input 缓存未命中 12 元/M、output 24 元/M），
// 届时 Opus 流量成本将翻 4 倍，需重新评估映射或定价。
//
// [1m] 后缀为 1M context 变体，模型本身一致，统一映射到同一上游模型。
var DefaultDeepSeekModelMapping = map[string]string{
	// Claude Opus → pro
	"claude-opus-4-7":          DeepSeekModelPro,
	"claude-opus-4-7[1m]":      DeepSeekModelPro,
	"claude-opus-4-6":          DeepSeekModelPro,
	"claude-opus-4-6[1m]":      DeepSeekModelPro,
	"claude-opus-4-6-thinking": DeepSeekModelPro,
	"claude-opus-4-5":          DeepSeekModelPro,
	"claude-opus-4-5[1m]":      DeepSeekModelPro,
	"claude-opus-4-5-thinking": DeepSeekModelPro,
	"claude-opus-4-5-20251101": DeepSeekModelPro,
	"claude-opus-4-1":          DeepSeekModelPro,
	"claude-opus-4-20250514":   DeepSeekModelPro,
	// Claude Sonnet → flash（保守默认；如需提质，逐条改为 DeepSeekModelPro）
	"claude-sonnet-4-6":          DeepSeekModelFlash,
	"claude-sonnet-4-6[1m]":      DeepSeekModelFlash,
	"claude-sonnet-4-6-thinking": DeepSeekModelFlash,
	"claude-sonnet-4-5":          DeepSeekModelFlash,
	"claude-sonnet-4-5[1m]":      DeepSeekModelFlash,
	"claude-sonnet-4-5-thinking": DeepSeekModelFlash,
	"claude-sonnet-4-5-20250929": DeepSeekModelFlash,
	"claude-sonnet-4-20250514":   DeepSeekModelFlash,
	// Claude Haiku → flash
	"claude-haiku-4-5":          DeepSeekModelFlash,
	"claude-haiku-4-5[1m]":      DeepSeekModelFlash,
	"claude-haiku-4-5-20251001": DeepSeekModelFlash,
}

// DefaultKimiModelMapping 是 Kimi (Moonshot) 平台的默认模型映射（Anthropic 协议入站 → Kimi 真实模型名）。
// 端点：https://api.moonshot.ai/anthropic/v1/messages
//
// 价位映射策略（K2 系列只有少数变体）：
//   - Haiku   → kimi-k2.5（轻量主力，价格不变 $0.60/M input）
//   - Sonnet  → kimi-k2.5（主力）
//   - Opus    → kimi-k2.6（最新顶级，SWE-Bench Pro 58.6%）
//   - thinking → kimi-k2-thinking（原生 reasoning 模型，与 Claude opus thinking 对位）
//
// 价格：$0.60/M input / $2.50/M output / cached input $0.15/M (75% 折扣)
//
// [1m] 后缀为 1M context 变体，统一映射到同一上游模型。
var DefaultKimiModelMapping = map[string]string{
	// Claude Opus → kimi-k2.6
	"claude-opus-4-7":          KimiModelTop,
	"claude-opus-4-7[1m]":      KimiModelTop,
	"claude-opus-4-6":          KimiModelTop,
	"claude-opus-4-6[1m]":      KimiModelTop,
	"claude-opus-4-6-thinking": KimiModelThinking,
	"claude-opus-4-5":          KimiModelTop,
	"claude-opus-4-5[1m]":      KimiModelTop,
	"claude-opus-4-5-thinking": KimiModelThinking,
	"claude-opus-4-5-20251101": KimiModelTop,
	"claude-opus-4-1":          KimiModelTop,
	"claude-opus-4-20250514":   KimiModelTop,
	// Claude Sonnet → kimi-k2.5
	"claude-sonnet-4-6":          KimiModelMain,
	"claude-sonnet-4-6[1m]":      KimiModelMain,
	"claude-sonnet-4-6-thinking": KimiModelThinking,
	"claude-sonnet-4-5":          KimiModelMain,
	"claude-sonnet-4-5[1m]":      KimiModelMain,
	"claude-sonnet-4-5-thinking": KimiModelThinking,
	"claude-sonnet-4-5-20250929": KimiModelMain,
	"claude-sonnet-4-20250514":   KimiModelMain,
	// Claude Haiku → kimi-k2.5
	"claude-haiku-4-5":          KimiModelMain,
	"claude-haiku-4-5[1m]":      KimiModelMain,
	"claude-haiku-4-5-20251001": KimiModelMain,
}

// DefaultMiMoModelMapping 是 Xiaomi MiMo 平台的默认模型映射（Anthropic 协议入站 → MiMo 真实模型名）。
// 端点：https://api.xiaomimimo.com/anthropic/v1/messages
//
// MiMo 当前公开两个可用模型，没有明显的 mid/mini 价位变体，统一映射到 mimo-v2-pro：
//   - Opus / Sonnet / Haiku → mimo-v2-pro（主力，1M context）
//   - mimo-v2-omni 仅在用户明确想用 multimodal 时手动指定，不参与默认映射
//
// 价格：$1/M input, $3/M output, cached input $0.20-$0.40/M。
//
// [1m] 后缀为 1M context 变体，统一映射到同一上游模型。
var DefaultMiMoModelMapping = map[string]string{
	// Claude Opus → mimo-v2-pro
	"claude-opus-4-7":          MiMoModelPro,
	"claude-opus-4-7[1m]":      MiMoModelPro,
	"claude-opus-4-6":          MiMoModelPro,
	"claude-opus-4-6[1m]":      MiMoModelPro,
	"claude-opus-4-6-thinking": MiMoModelPro,
	"claude-opus-4-5":          MiMoModelPro,
	"claude-opus-4-5[1m]":      MiMoModelPro,
	"claude-opus-4-5-thinking": MiMoModelPro,
	"claude-opus-4-5-20251101": MiMoModelPro,
	"claude-opus-4-1":          MiMoModelPro,
	"claude-opus-4-20250514":   MiMoModelPro,
	// Claude Sonnet → mimo-v2-pro
	"claude-sonnet-4-6":          MiMoModelPro,
	"claude-sonnet-4-6[1m]":      MiMoModelPro,
	"claude-sonnet-4-6-thinking": MiMoModelPro,
	"claude-sonnet-4-5":          MiMoModelPro,
	"claude-sonnet-4-5[1m]":      MiMoModelPro,
	"claude-sonnet-4-5-thinking": MiMoModelPro,
	"claude-sonnet-4-5-20250929": MiMoModelPro,
	"claude-sonnet-4-20250514":   MiMoModelPro,
	// Claude Haiku → mimo-v2-pro
	"claude-haiku-4-5":          MiMoModelPro,
	"claude-haiku-4-5[1m]":      MiMoModelPro,
	"claude-haiku-4-5-20251001": MiMoModelPro,
}

// DefaultOpenAIModelMapping 是 OpenAI 平台的默认模型映射（Anthropic 协议入站 → OpenAI 真实模型名）。
//
// 适用场景：ChatGPT Plus 订阅 OAuth 类型账号（codex 后端）以及官方 OpenAI API。
// codex 后端实测接受通用 gpt-5.x 模型名（normalizeCodexModel 兜底默认即为 gpt-5.4），
// 与 Codex CLI 实际使用一致，无需专门 gpt-5.3-codex 系列。
//
// 价位映射策略（与 DeepSeek 对位一致）：
//   - Haiku  → gpt-5.4-mini（经济对经济）
//   - Sonnet → gpt-5.4（主力，性价比，Codex CLI 默认）
//   - Opus   → gpt-5.5（高端对高端）
//
// 注意：本 map **当前并未挂在后端 forward 路径上**（保留 normalizeCodexModel 兜底为 gpt-5.4
// 的既有 design 约束，避免破坏 TestResolveOpenAIForwardModel_PreventsClaudeModelFromFallingBackToGpt54）。
// 本 map 用于前端创建 OpenAI 账号时"一键填充 model_mapping"模板，让用户少配置。
//
// OpenAI 协议入站（model=gpt-*）的请求不会被本 map 影响——key 全部是 claude-*，与 gpt-* 不重叠。
//
// [1m] 后缀为 1M context 变体，模型本身一致，统一映射到同一上游模型。
var DefaultOpenAIModelMapping = map[string]string{
	// Claude Opus → gpt-5.5
	"claude-opus-4-7":          OpenAIModelTop,
	"claude-opus-4-7[1m]":      OpenAIModelTop,
	"claude-opus-4-6":          OpenAIModelTop,
	"claude-opus-4-6[1m]":      OpenAIModelTop,
	"claude-opus-4-6-thinking": OpenAIModelTop,
	"claude-opus-4-5":          OpenAIModelTop,
	"claude-opus-4-5[1m]":      OpenAIModelTop,
	"claude-opus-4-5-thinking": OpenAIModelTop,
	"claude-opus-4-5-20251101": OpenAIModelTop,
	"claude-opus-4-1":          OpenAIModelTop,
	"claude-opus-4-20250514":   OpenAIModelTop,
	// Claude Sonnet → gpt-5.4
	"claude-sonnet-4-6":          OpenAIModelMain,
	"claude-sonnet-4-6[1m]":      OpenAIModelMain,
	"claude-sonnet-4-6-thinking": OpenAIModelMain,
	"claude-sonnet-4-5":          OpenAIModelMain,
	"claude-sonnet-4-5[1m]":      OpenAIModelMain,
	"claude-sonnet-4-5-thinking": OpenAIModelMain,
	"claude-sonnet-4-5-20250929": OpenAIModelMain,
	"claude-sonnet-4-20250514":   OpenAIModelMain,
	// Claude Haiku → gpt-5.4-mini
	"claude-haiku-4-5":          OpenAIModelMini,
	"claude-haiku-4-5[1m]":      OpenAIModelMini,
	"claude-haiku-4-5-20251001": OpenAIModelMini,
}
