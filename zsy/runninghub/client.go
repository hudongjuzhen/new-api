package runninghub

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

// ---------------------------------------------------------------------------
// Shared DTOs for the upstream V2 flat JSON shape (probed in §3.9)
// ---------------------------------------------------------------------------

// SubmitNodeInfo mirrors a single entry in the V2 submit body's nodeInfoList.
type SubmitNodeInfo struct {
	NodeID        string `json:"nodeId,omitempty"`
	FieldName     string `json:"fieldName,omitempty"`
	Field         string `json:"field,omitempty"`
	FieldValue    string `json:"fieldValue,omitempty"`
	Description   string `json:"description,omitempty"`
	DescriptionEn string `json:"descriptionEn,omitempty"`
}

// SubmitBody is the request body for POST /openapi/v2/run/ai-app/{id} and
// /run/workflow/{id}.
type SubmitBody struct {
	InstanceType     string           `json:"instanceType,omitempty"`
	UsePersonalQueue bool             `json:"usePersonalQueue,omitempty"`
	NodeInfoList     []SubmitNodeInfo `json:"nodeInfoList,omitempty"`
	WebhookURL       string           `json:"webhookUrl,omitempty"`
	AccessPassword   string           `json:"accessPassword,omitempty"`
}

// SubmitResp is the flat V2 response shape returned by a successful submit.
type SubmitResp struct {
	TaskID        string           `json:"taskId"`
	Status        string           `json:"status"`
	ClientID      string           `json:"clientId,omitempty"`
	PromptTips    string           `json:"promptTips,omitempty"`
	FailedReason  string           `json:"failedReason,omitempty"`
	ErrorCode     string           `json:"errorCode,omitempty"`
	ErrorMessage  string           `json:"errorMessage,omitempty"`
	Usage         *TaskUsage       `json:"usage,omitempty"`
	TaskUsageList []TaskUsageEntry `json:"taskUsageList,omitempty"`
	Results       []TaskResult     `json:"results,omitempty"`
}

// TaskResult describes a single output item from RH.
type TaskResult struct {
	URL        string `json:"url,omitempty"`
	NodeID     string `json:"nodeId,omitempty"`
	OutputType string `json:"outputType,omitempty"`
	Text       string `json:"text,omitempty"`
}

// TaskUsage is the charge breakdown returned alongside submit/query responses.
// All fields are kept string because RH upstream uses quoted numeric strings
// for precision in different SDKs.
type TaskUsage struct {
	ConsumeMoney           string `json:"consumeMoney,omitempty"`
	ConsumeCoins           string `json:"consumeCoins,omitempty"`
	TaskCostTime           string `json:"taskCostTime,omitempty"`
	ThirdPartyConsumeMoney string `json:"thirdPartyConsumeMoney,omitempty"`
}

// TaskUsageEntry appears in taskUsageList arrays on the flat V2 response.
type TaskUsageEntry struct {
	TaskID       string     `json:"taskId,omitempty"`
	ParentTaskID string     `json:"parentTaskId,omitempty"`
	TaskStatus   string     `json:"taskStatus,omitempty"`
	Usage        *TaskUsage `json:"usage,omitempty"`
}

// QueryResp is the response shape of POST /openapi/v2/query. It matches
// SubmitResp exactly (the V2 spec conflates the two) but we keep a distinct
// type alias to aid readability.
type QueryResp = SubmitResp

// QueryBody is the camelCase body for POST /openapi/v2/query.
type QueryBody struct {
	TaskID string `json:"taskId"`
}

// ---------------------------------------------------------------------------
// Client — the upstream transport layer.
// ---------------------------------------------------------------------------

// Client encapsulates one RH endpoint call. The key comes from the
// channel's Key field, the base URL from the channel's BaseURL.
type Client struct {
	BaseURL    string
	Key        string
	HTTPClient *http.Client

	// UserAgent is reported on every request; defaults to
	// "new-api-runninghub/0.1".
	UserAgent string
}

// DefaultUserAgent is set on requests made by Client when Caller doesn't set
// UserAgent.
const DefaultUserAgent = "new-api-runninghub/0.1"

// NewClient returns a new Client with a sensible default transport.
func NewClient(baseURL, key string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Key: key, HTTPClient: httpClient, UserAgent: DefaultUserAgent}
}

// NewClientForType returns a new Client whose default base URL matches the
// given channel type (RunningHub → runninghub.cn, RunningHub 国际站 →
// runninghub.ai). Callers that already resolved the channel's base URL should
// pass it in baseURL — the type only matters when baseURL is empty.
func NewClientForType(channelType int, baseURL, key string, httpClient *http.Client) *Client {
	if baseURL == "" {
		switch channelType {
		case constant.ChannelTypeRunningHubIntl:
			baseURL = DefaultBaseURLIntl
		default:
			baseURL = DefaultBaseURL
		}
	}
	return NewClient(baseURL, key, httpClient)
}

// SubmitAICApp posts a nodeInfoList payload to the V2 AI 应用 submit path.
func (c *Client) SubmitAICApp(appID string, nodes []SubmitNodeInfo, webhook, instanceType, accessPassword string) (*SubmitResp, error) {
	if appID == "" {
		return nil, fmt.Errorf("runninghub: empty ai-app id")
	}
	body := SubmitBody{
		InstanceType:   pickInstanceType(instanceType),
		NodeInfoList:   nodes,
		WebhookURL:     webhook,
		AccessPassword: accessPassword,
	}
	out := &SubmitResp{}
	err := c.doJSON(http.MethodPost, PathSubmitAICApp+appID, body, out)
	if err != nil {
		return nil, err
	}
	if out.ErrorCode != "" && (out.Status == "" || out.Status == StatusFailed) {
		return out, fmt.Errorf("runninghub submit ai-app %s failed: code=%s msg=%s", appID, out.ErrorCode, out.ErrorMessage)
	}
	return out, nil
}

// SubmitWorkflow posts a nodeInfoList payload to the V2 workflow submit path.
func (c *Client) SubmitWorkflow(workflowID string, nodes []SubmitNodeInfo, webhook, instanceType, accessPassword string) (*SubmitResp, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("runninghub: empty workflow id")
	}
	body := SubmitBody{
		InstanceType:   pickInstanceType(instanceType),
		NodeInfoList:   nodes,
		WebhookURL:     webhook,
		AccessPassword: accessPassword,
	}
	out := &SubmitResp{}
	err := c.doJSON(http.MethodPost, PathSubmitWorkflow+workflowID, body, out)
	if err != nil {
		return nil, err
	}
	if out.ErrorCode != "" && (out.Status == "" || out.Status == StatusFailed) {
		return out, fmt.Errorf("runninghub submit workflow %s failed: code=%s msg=%s", workflowID, out.ErrorCode, out.ErrorMessage)
	}
	return out, nil
}

// Query hits POST /openapi/v2/query and returns the parsed response.
func (c *Client) Query(taskID string) (*QueryResp, error) {
	if taskID == "" {
		return nil, fmt.Errorf("runninghub: empty taskId")
	}
	body := QueryBody{TaskID: taskID}
	out := &QueryResp{}
	if err := c.doJSON(http.MethodPost, PathQueryTask, body, out); err != nil {
		return nil, err
	}
	// Upstream returns 200 with empty taskId + errorCode="1007" when a task is
	// not found / not visible with this key; that is an HTTP OK but a semantic
	// error for callers.
	if out.TaskID == "" && out.ErrorCode != "" {
		return out, fmt.Errorf("runninghub query failed: code=%s msg=%s", out.ErrorCode, out.ErrorMessage)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Transport helpers
// ---------------------------------------------------------------------------

func pickInstanceType(s string) string {
	if s == InstanceDefault || s == InstancePlus {
		return s
	}
	return InstanceDefault
}

func (c *Client) doJSON(method, path string, reqBody any, respOut any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := common.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("runninghub marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	fullURL := c.BaseURL + path
	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("runninghub new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	if ua := c.UserAgent; ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("runninghub transport %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("runninghub read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to decode errorCode/errorMessage for better diagnostics even
		// when HTTP status != 2xx.
		tmp := &struct {
			ErrorCode    string `json:"errorCode"`
			ErrorMessage string `json:"errorMessage"`
		}{}
		_ = common.Unmarshal(raw, tmp)
		if tmp.ErrorMessage == "" {
			tmp.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateString(string(raw), 240))
		}
		return fmt.Errorf("runninghub %s %s: status=%d code=%s msg=%s", method, path, resp.StatusCode, tmp.ErrorCode, tmp.ErrorMessage)
	}
	if respOut != nil && len(raw) > 0 {
		if err := common.Unmarshal(raw, respOut); err != nil {
			return fmt.Errorf("runninghub decode %s %s: %w (body=%s)", method, path, err, truncateString(string(raw), 240))
		}
	}
	return nil
}

func truncateString(s string, n int) string {
	if n <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
