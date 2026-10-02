package channel

import (
	"io"
	"net/http"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

type Adaptor interface {
	// Init IsStream bool
	Init(info *relaycommon.RelayInfo)
	GetRequestURL(info *relaycommon.RelayInfo) (string, error)
	SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
	ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error)
	ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error)
	ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error)
	ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error)
	ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error)
	ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError)
	GetModelList() []string
	GetChannelName() string
	ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error)
	ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error)
}

type TaskAdaptor interface {
	Init(info *relaycommon.RelayInfo)

	ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError

	// ── Billing ──────────────────────────────────────────────────────

	// EstimateBilling returns OtherRatios for pre-charge based on user request.
	// Called after ValidateRequestAndSetAction, before price calculation.
	// Adaptors should extract duration, resolution, etc. from the parsed request
	// and return them as ratio multipliers (e.g. {"seconds": 5, "size": 1.666}).
	// Return nil to use the base model price without extra ratios.
	EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64

	// AdjustBillingOnSubmit returns adjusted OtherRatios from the upstream
	// submit response. Called after a successful DoResponse.
	// If the upstream returned actual parameters that differ from the estimate
	// (e.g. actual seconds), return updated ratios so the caller can recalculate
	// the quota and settle the delta with the pre-charge.
	// Return nil if no adjustment is needed.
	AdjustBillingOnSubmit(info *relaycommon.RelayInfo, taskData []byte) map[string]float64

	// AdjustBillingOnComplete returns the actual quota when a task reaches a
	// terminal state (success/failure) during polling.
	// Called by the polling loop after ParseTaskResult.
	// Return a positive value to trigger delta settlement (supplement / refund).
	// Return 0 to keep the pre-charged amount unchanged.
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int

	// ── Optional: metered (per-unit) billing ─────────────────────────

	// MeteredBillingBasis declares, per metered dimension, the unit price the
	// submit-time estimate was priced at and the estimated quantity it priced.
	//
	// It exists for tasks whose charge follows a quantity the upstream only
	// reveals after the task finishes: a model billed per minute of generated
	// audio announces {"audio_minutes": {quantity: 2, unitPrice: <quota per
	// minute>}} for a two-minute estimate. The polling layer freezes both numbers
	// on the task and later re-prices it as
	//
	//	pre-charge × min(actual, estimated) / estimated
	//
	// so a price or group-ratio change between submit and completion never
	// re-prices a task that is already running.
	//
	// unitPrice is in the same quota currency as PriceData.Quota and already
	// includes the group ratio, so quantity × unitPrice must equal the quota the
	// pre-charge actually reserved for that dimension — not more, not less.
	//
	// Return nil for tasks with no metered dimension.
	MeteredBillingBasis(info *relaycommon.RelayInfo) map[string]relaycommon.MeteredBasis

	// MeteredUsage reads the actually-produced quantities a successful task
	// reported, keyed by the dimensions declared above, in the same unit as the
	// estimated quantity. A dimension that is missing, unparseable or out of
	// range is omitted, and the polling layer then keeps the pre-charged amount.
	//
	// Implementations must bound every value before returning: these numbers come
	// from the upstream and must never reach quota arithmetic unbounded.
	//
	// Return nil when the result carries no usable measurement.
	MeteredUsage(task *model.Task, taskResult *relaycommon.TaskInfo) map[string]float64

	// ── Request / Response ───────────────────────────────────────────

	BuildRequestURL(info *relaycommon.RelayInfo) (string, error)
	BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error
	BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error)

	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, err *taskdto.TaskError)

	GetModelList() []string
	GetChannelName() string

	// ── Polling ──────────────────────────────────────────────────────

	FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error)
}

type OpenAIVideoConverter interface {
	ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error)
}

// TaskMeteredBilling is the submit-time half of metered (per-unit) task
// billing. A task adaptor implements it when the charge follows a quantity the
// upstream only reveals after the task finishes — for example a model billed
// per minute of generated audio.
//
// The submit path freezes the returned basis on the task's billing context; the
// polling path later re-prices the task from that frozen record plus the
// quantity TaskMeteredUsageReporter reports, so a price or group-ratio change
// between submit and completion never re-prices a running task.
type TaskMeteredBilling interface {
	MeteredBillingBasis(info *relaycommon.RelayInfo) map[string]relaycommon.MeteredBasis
}
