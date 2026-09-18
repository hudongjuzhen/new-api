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
	//
	// The RH docs call this endpoint in two subtly different shapes depending
	// on the consuming workflow:
	//
	//   - Workflow style (what apiCallDemo returns after uploading a file):
	//     multipart field name "file", response { data: { fileName: "…" }} —
	//     the fileName is what nodeInfoList[].fieldValue expects.
	//   - Model API style: multipart field name "text", response
	//     { data: { download_url: "…" }} — the URL is what the node expects.
	//
	// Both flavours accept any binary content; RH does not restrict by mime.
	// The gateway handler forwards workflow-style uploads (field "file") and
	// surfaces fileName, which is the value running workflows reference.
	PathUploadBinary = openAPIV2Prefix + "/media/upload/binary"

	// UploadFieldWorkflow is the multipart field name workflows expect.
	UploadFieldWorkflow = "file"

	// PathCancelTask is the task-cancel endpoint. It is NOT part of the v2
	// prefix: /openapi/v2/* has no cancel route (every v2 guess answers
	// 1001 Invalid URL), while the legacy path below accepts both the key and an
	// AI-application taskId. Verified against a working integration; the body
	// carries {"apiKey": …, "taskId": …} and the response is the flat
	// {code, msg, data} envelope.
	PathCancelTask = "/task/openapi/cancel"
)

// Standard status strings RH uses in response.status.
const (
	StatusQueued   = "QUEUED"
	StatusRunning  = "RUNNING"
	StatusSuccess  = "SUCCESS"
	StatusFailed   = "FAILED"
	StatusCanceled = "CANCELED"
	// StatusCancelled is the same terminal state spelled with a double L —
	// upstream uses both forms depending on the endpoint, so both must map to a
	// terminal local state (otherwise a cancelled task keeps occupying a
	// concurrency slot forever).
	StatusCancelled = "CANCELLED"
)

// Error codes RH returns through the flat response shape. When the submit
// itself fails (not the task) the adaptor classifies them into a small set
// for user-friendly rendering.
const (
	ErrCodeParams           = "1007" // request body 解析 / 参数缺失 / 任务不存在 ("must not be null" 等)
	ErrCodeInvalidURL       = "1001" // 路径拼写错误 (Invalid URL)
	ErrCodeNodeInfoMismatch = "803"  // nodeId/fieldName 不在工作流里
	ErrCodeAccessDenied     = "1014" // 个人 Key 调 Standard Model API 被拒等
	// ErrCodeCancelNotAllowed is what the cancel endpoint answers when the task
	// can no longer be interrupted (usually already finished). It is a normal
	// outcome to report, not a transport failure.
	ErrCodeCancelNotAllowed = "817" // APIKEY_TASK_CANCEL_NOT_ALLOWED
)

// Field names used for submit bodies (all camelCase to match RH V2 JSON).
const (
	SubmitFieldInstanceType     = "instanceType"
	SubmitFieldUsePersonalQueue = "usePersonalQueue"
	SubmitFieldNodeInfoList     = "nodeInfoList"
	SubmitFieldWebhookURL       = "webhookUrl"
	SubmitFieldAccessPassword   = "accessPassword"
)

// Stable machine-readable error codes the plugin returns to API-key callers, so
// a third-party client can branch on `code` instead of matching message text.
const (
	// ErrorCodeTokenSelectionNotAllowed: the run body carried a tokenId that is
	// not the calling key. A key can only spend its own quota.
	ErrorCodeTokenSelectionNotAllowed = "token_selection_not_allowed"
	// ErrorCodeTaskNotOwnedByKey: the task exists on the account but was not
	// submitted by the calling key.
	ErrorCodeTaskNotOwnedByKey = "task_not_owned_by_key"
	// ErrorCodeKeyScopedListUnsupported: listing the account's history is a
	// dashboard operation; a key must poll the task ids it created.
	ErrorCodeKeyScopedListUnsupported = "key_scoped_list_unsupported"
	// ErrorCodeTokenModelForbidden: the key's model allow-list does not cover the
	// app being run.
	ErrorCodeTokenModelForbidden = "token_model_forbidden"
)

// contextKeyControllerResponds marks a submit request whose handler writes the
// API response itself (the plugin's own run endpoint), so the task adaptor does
// not write a second body onto the same response.
const contextKeyControllerResponds = "rh_controller_responds"


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

// DefaultBaseURLIntl mirrors constant.ChannelBaseURLs[RunningHubIntl] and is
// used as a fallback for RunningHub 国际站 channels.
const DefaultBaseURLIntl = "https://www.runninghub.ai"
