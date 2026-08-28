package runninghub

// RunningHub upstream API path & field constants.
//
// All values here come from the empirical probe run in §3.9 of the dev plan
// (scripts/rh-probe-*.ps1) and are cross-checked against the official
// RunningHub apifox docs. When RH changes a path or field name the fix is a
// one-line change in this file.

const (
	// openAPIV2Prefix is the path prefix for V2 style endpoints (flat JSON
	// response shape). Both runninghub.cn and runninghub.ai share it.
	openAPIV2Prefix = "/openapi/v2"

	// PathSubmitAICApp is the POST submit path for "AI 应用" (webapp).  The
	// final path component is the webapp id supplied by the admin.
	PathSubmitAICApp = openAPIV2Prefix + "/run/ai-app/"

	// PathSubmitWorkflow is the POST submit path for "工作流" (workflow).
	PathSubmitWorkflow = openAPIV2Prefix + "/run/workflow/"

	// PathQueryTask is the polling endpoint.  Request body shape is
	// {"taskId":"<camelCase id>"}. Header Bearer auth is sufficient.
	PathQueryTask = openAPIV2Prefix + "/query"

	// PathUploadBinary is the file-upload endpoint (multipart/form-data).
	// Upstream docs promise both fileName (workflow-style) and download_url
	// (model-style) in the wrapped response.
	PathUploadBinary = openAPIV2Prefix + "/media/upload/binary"
)

// Standard status strings RH uses in response.status.
const (
	StatusQueued   = "QUEUED"
	StatusRunning  = "RUNNING"
	StatusSuccess  = "SUCCESS"
	StatusFailed   = "FAILED"
	StatusCanceled = "CANCELED"
)

// Error codes RH returns through the flat response shape. When the submit
// itself fails (not the task) the adaptor classifies them into a small set
// for user-friendly rendering.
const (
	ErrCodeParams           = "1007" // request body 解析 / 参数缺失 / 任务不存在 ("must not be null" 等)
	ErrCodeInvalidURL       = "1001" // 路径拼写错误 (Invalid URL)
	ErrCodeNodeInfoMismatch = "803"  // nodeId/fieldName 不在工作流里
	ErrCodeAccessDenied     = "1014" // 个人 Key 调 Standard Model API 被拒等
)

// Field names used for submit bodies (all camelCase to match RH V2 JSON).
const (
	SubmitFieldInstanceType     = "instanceType"
	SubmitFieldUsePersonalQueue = "usePersonalQueue"
	SubmitFieldNodeInfoList     = "nodeInfoList"
	SubmitFieldWebhookURL       = "webhookUrl"
	SubmitFieldAccessPassword   = "accessPassword"
)

// NodeInfo struct fields (matches both V2 submit JSON and the curl parser).
const (
	NodeFieldNodeID    = "nodeId"
	NodeFieldFieldName = "fieldName"
	NodeFieldField     = "field"
	NodeFieldValue     = "fieldValue"
	NodeFieldDesc      = "description"
	NodeFieldDescEN    = "descriptionEn"
)

// Query body fields.
const (
	QueryFieldTaskID = "taskId"
)

// Instance type strings recognised by RH upstream.
const (
	InstanceDefault = "default" // 24GB
	InstancePlus    = "plus"    // 48GB
)

// DefaultBaseURL mirrors constant.ChannelBaseURLs[RunningHub] and is used as a
// fallback when the channel's base URL is empty.
const DefaultBaseURL = "https://www.runninghub.cn"
