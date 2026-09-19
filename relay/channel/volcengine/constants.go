package volcengine

// ModelList 是方舟（VolcEngine）渠道的模型建议列表，供管理端"获取模型列表"填
// 入渠道模型字段。方舟同一账号能调用的模型取决于开通情况，这里按能力分组列出
// 当前在售的常用模型（依据 2026-09 的模型列表文档与 /api/v3/models 目录整理；
// 文档中标记 Shutdown 的历史模型不再列出）。
var ModelList = []string{
	// 图片生成：/api/v3/images/generations（Seedream 系列）
	"doubao-seedream-5-0-260128",
	"doubao-seedream-5-0-pro-260628",
	"doubao-seedream-4-5-251128",
	"doubao-seedream-4-0-250828",
	"doubao-seedream-4-0-20260415",
	// 视频生成：/api/v3/contents/generations/tasks（Seedance 系列）
	"doubao-seedance-2-5-260628",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-mini-260615",
	"doubao-seedance-2-0-fast-260128",
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-pro-fast-251015",
	// 文本 / 多模态：/api/v3/chat/completions
	"doubao-seed-2-1-pro-260915",
	"doubao-seed-2-1-pro-260628",
	"doubao-seed-2-1-turbo-260628",
	"doubao-seed-2-0-pro-260215",
	"doubao-seed-2-0-lite-260428",
	"doubao-seed-2-0-mini-260428",
	"doubao-seed-character-260628",
	// 向量化：/api/v3/embeddings
	"doubao-embedding-vision-251215",
	"doubao-embedding-large-text-250515",
}

var ChannelName = "volcengine"
