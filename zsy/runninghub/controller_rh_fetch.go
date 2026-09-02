package runninghub

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Demo node types (apiCallDemo response)
// ---------------------------------------------------------------------------

// DemoNode mirrors one entry in the apiCallDemo response's nodeInfoList. It
// carries richer metadata than the submit-side NodeInfo: the optional fieldData
// string describes enum/select choices, and description* give the human
// readable field label.
type DemoNode struct {
	NodeID        string `json:"nodeId,omitempty"`
	NodeName      string `json:"nodeName,omitempty"`
	FieldName     string `json:"fieldName,omitempty"`
	FieldValue    string `json:"fieldValue,omitempty"`
	FieldData     string `json:"fieldData,omitempty"`
	Description   string `json:"description,omitempty"`
	DescriptionEn string `json:"descriptionEn,omitempty"`
}

// ApiCallDemoResp is the decoded shape of GET /api/webapp/apiCallDemo.
type ApiCallDemoResp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data ApiDemoData `json:"data"`
}

// ApiDemoData is the data payload of ApiCallDemoResp.
type ApiDemoData struct {
	Curl         string     `json:"curl"`
	WebappName   string     `json:"webappName"`
	NodeInfoList []DemoNode `json:"nodeInfoList"`
}

// ApiCallDemo hits GET /api/webapp/apiCallDemo?apiKey=&webappId= and returns
// the application's authoritative parameter nodes. The key is the channel's
// RH key; webappId is the application's upstream ID.
func (c *Client) ApiCallDemo(webappID string) (*ApiCallDemoResp, error) {
	urlPath := "/api/webapp/apiCallDemo?apiKey=" + url.QueryEscape(c.Key) +
		"&webappId=" + url.QueryEscape(webappID)
	var out ApiCallDemoResp
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("runninghub new demo request: %w", err)
	}
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
		return nil, fmt.Errorf("runninghub apiCallDemo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("runninghub read apiCallDemo: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// RH wraps errors as {code, msg} JSON; surface the message when possible.
		var e struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		_ = common.Unmarshal(raw, &e)
		if e.Msg != "" {
			return nil, fmt.Errorf("runninghub apiCallDemo http %d: %s", resp.StatusCode, e.Msg)
		}
		return nil, fmt.Errorf("runninghub apiCallDemo http %d", resp.StatusCode)
	}
	if err := common.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("runninghub decode apiCallDemo: %w", err)
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Schema inference from demo nodes
// ---------------------------------------------------------------------------

// buildSchemaFromDemoNodes converts apiCallDemo node entries into an inferred
// SchemaParam list. Nodes backed by an enumerable fieldData become "select";
// the rest go through rhparser's BuildSchemaFromNodes heuristics (text/
// textarea/number/image/audio/video). It returns any dropped/ambiguous nodes
// as warnings.
func buildSchemaFromDemoNodes(nodes []DemoNode) ([]rhparser.SchemaParam, []rhparser.ErrSchemaReport) {
	schema := make([]rhparser.SchemaParam, 0, len(nodes))
	warnings := make([]rhparser.ErrSchemaReport, 0)

	plain := make([]rhparser.NodeInfo, 0, len(nodes))
	for _, n := range nodes {
		if opts, ok := parseSelectOptions(n.FieldData); ok {
			schema = append(schema, rhparser.SchemaParam{
				NodeID:    n.NodeID,
				FieldName: n.FieldName,
				Label:     labelForDemoNode(&n),
				Type:      "select",
				Default:   n.FieldValue,
				Required:  true,
				Options:   opts,
			})
			continue
		}
		plain = append(plain, rhparser.NodeInfo{
			NodeID:        n.NodeID,
			FieldName:     n.FieldName,
			FieldValue:    n.FieldValue,
			Description:   n.Description,
			DescriptionEn: n.DescriptionEn,
		})
	}
	if len(plain) > 0 {
		summary := rhparser.BuildSchemaFromNodes(plain)
		schema = append(schema, summary.Params...)
		warnings = append(warnings, summary.Errors...)
	}
	return schema, warnings
}

// parseSelectOptions parses a RunningHub fieldData JSON array into select
// options. A non-enumerated (or unparseable) fieldData returns ok=false so the
// caller falls back to the text/image/... heuristics.
func parseSelectOptions(fieldData string) ([]rhparser.SchemaParamOption, bool) {
	ft := strings.TrimSpace(fieldData)
	if ft == "" || ft == "[]" || !strings.HasPrefix(ft, "[") {
		return nil, false
	}
	var raw []struct {
		Name        string `json:"name"`
		Index       string `json:"index"`
		Description string `json:"description"`
	}
	if err := common.Unmarshal([]byte(ft), &raw); err != nil || len(raw) == 0 {
		return nil, false
	}
	opts := make([]rhparser.SchemaParamOption, 0, len(raw))
	for _, r := range raw {
		value := r.Index
		if value == "" {
			value = r.Name
		}
		if value == "" {
			continue
		}
		label := r.Name
		if label == "" {
			label = r.Description
		}
		if label == "" {
			label = value
		}
		opts = append(opts, rhparser.SchemaParamOption{Label: label, Value: value})
	}
	return opts, len(opts) > 0
}

// labelForDemoNode returns the best human-readable label for a demo node.
func labelForDemoNode(n *DemoNode) string {
	if n.Description != "" {
		return n.Description
	}
	if n.DescriptionEn != "" {
		return n.DescriptionEn
	}
	if strings.TrimSpace(n.NodeName) != "" {
		return n.NodeName
	}
	return n.FieldName
}

// ---------------------------------------------------------------------------
// Admin handler
// ---------------------------------------------------------------------------

// fetchAppTemplate is the admin "one-click fetch parameter template" handler.
// It reuses the channel's RunningHub key/pin and calls the upstream
// apiCallDemo endpoint, then infers a SchemaParam list plus any warnings for
// the admin to review before saving.
//
// Request: { channelId: int, upstreamId: string }
//
//	Response (ApiSuccess): {
//	  appName: string, upstreamId: string,
//	  schema: []rhparser.SchemaParam, schemaErrors: []rhparser.ErrSchemaReport
//	}
func fetchAppTemplate(c *gin.Context) {
	var req struct {
		ChannelID  int64  `json:"channelId"`
		UpstreamID string `json:"upstreamId"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.ChannelID <= 0 {
		common.ApiErrorMsg(c, "channelId 必须为正整数")
		return
	}
	if strings.TrimSpace(req.UpstreamID) == "" {
		common.ApiErrorMsg(c, "upstreamId 不能为空")
		return
	}

	ch, err := model.GetChannelById(int(req.ChannelID), false)
	if err != nil {
		common.ApiError(c, fmt.Errorf("加载 RunningHub 渠道失败: %w", err))
		return
	}
	if !pluginChannelTypes[ch.Type] {
		common.ApiErrorMsg(c, fmt.Sprintf("渠道 %d (%s) 不是 RunningHub 渠道 (type=%d)", req.ChannelID, ch.Name, ch.Type))
		return
	}

	baseURL := ""
	if ch.BaseURL != nil {
		baseURL = *ch.BaseURL
	}
	client := NewClientForType(ch.Type, baseURL, ch.Key, nil)
	demo, err := client.ApiCallDemo(req.UpstreamID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	schema, warnings := buildSchemaFromDemoNodes(demo.Data.NodeInfoList)
	if schema == nil {
		schema = []rhparser.SchemaParam{}
	}
	if warnings == nil {
		warnings = []rhparser.ErrSchemaReport{}
	}
	common.ApiSuccess(c, gin.H{
		"appName":      demo.Data.WebappName,
		"upstreamId":   req.UpstreamID,
		"schema":       schema,
		"schemaErrors": warnings,
	})
}
