package runninghub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// Inline preview of one result file.
//
// Result files live on RunningHub's own storage, which serves no CORS headers,
// so a browser cannot read a .txt/.json/.csv result to display it. This
// endpoint fetches the file server-side and returns its text.
//
// It is deliberately not a general fetch proxy:
//   - the task must belong to the caller (ownership check),
//   - the URL must be one of that task's own result URLs (no arbitrary targets),
//   - only text-ish content types are returned, capped in size,
//   - everything else stays a direct browser download from the upstream URL.
const (
	// resultPreviewMaxBytes caps how much of a text result is buffered and sent.
	resultPreviewMaxBytes = 1 << 20 // 1 MiB
	// resultPreviewTimeout bounds one upstream fetch so a stalled host cannot
	// hold the request open indefinitely.
	resultPreviewTimeout = 30 * time.Second
)

// errResultNotTextual marks a file that has no inline text preview.
var errResultNotTextual = errors.New("result is not a text file")

// taskResultEntry is the subset of a stored `results[]` item needed to verify
// that a requested URL really belongs to the task (the legacy `value` spelling
// is accepted as well).
type taskResultEntry struct {
	URL   string `json:"url"`
	Value string `json:"value"`
}

// resultContentResponse is the payload of a successful preview fetch.
type resultContentResponse struct {
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
	// Truncated reports that the file is longer than resultPreviewMaxBytes.
	Truncated bool `json:"truncated"`
}

// getTaskResultContent returns the text content of one of the caller's result
// files so the app center can render it inline (GET
// /api/zsy/rh/apps/task/:task_id/content?url=...).
func getTaskResultContent(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	rawURL := strings.TrimSpace(c.Query("url"))
	if taskID == "" || rawURL == "" {
		common.ApiErrorMsg(c, "task_id 与 url 不能为空")
		return
	}
	userId := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userId, taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		common.ApiErrorMsg(c, "任务不存在或无权访问")
		return
	}
	if denied := tokenScopeDenied(c, task); denied != "" {
		apiError(c, http.StatusForbidden, ErrorCodeTaskNotOwnedByKey, denied)
		return
	}
	if !taskOwnsResultURL(task, rawURL) {
		common.ApiErrorMsg(c, "仅支持预览该任务自身的结果文件")
		return
	}

	content, contentType, truncated, err := readResultPreview(rawURL)
	if err != nil {
		if errors.Is(err, errResultNotTextual) {
			common.ApiErrorMsg(c, "该文件类型不支持在线预览，请直接下载")
			return
		}
		common.ApiErrorMsg(c, "读取结果文件失败: "+err.Error())
		return
	}
	common.ApiSuccess(c, resultContentResponse{
		Content:     string(content),
		ContentType: contentType,
		Truncated:   truncated,
	})
}

// readResultPreview fetches a bounded slice of the upstream file.
func readResultPreview(rawURL string) ([]byte, string, bool, error) {
	var (
		resp *http.Response
		err  error
	)
	if system_setting.EnableWorker() {
		// The worker proxies the fetch when the deployment cannot reach the
		// upstream storage directly; it validates the URL itself.
		resp, err = service.DoDownloadRequest(rawURL, "rh result preview")
	} else {
		if err = service.ValidateSSRFProtectedFetchURL(rawURL); err != nil {
			return nil, "", false, fmt.Errorf("request reject: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), resultPreviewTimeout)
		defer cancel()
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if reqErr != nil {
			return nil, "", false, reqErr
		}
		resp, err = service.GetSSRFProtectedHTTPClient().Do(req)
	}
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", false, fmt.Errorf("上游返回状态 %d", resp.StatusCode)
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !isTextualResult(contentType, rawURL) {
		return nil, contentType, false, errResultNotTextual
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, resultPreviewMaxBytes+1))
	if err != nil {
		return nil, contentType, false, err
	}
	if len(raw) > resultPreviewMaxBytes {
		return raw[:resultPreviewMaxBytes], contentType, true, nil
	}
	return raw, contentType, false, nil
}

// isTextualResult reports whether a response can be shown as text. The content
// type decides when the upstream sends one; a result whose type is missing or
// generic (octet-stream) still previews when its URL says it is text.
func isTextualResult(contentType, rawURL string) bool {
	if contentType != "" &&
		contentType != "application/octet-stream" &&
		contentType != "binary/octet-stream" {
		if strings.HasPrefix(contentType, "text/") {
			return true
		}
		switch strings.SplitN(contentType, ";", 2)[0] {
		case "application/json",
			"application/xml",
			"application/x-ndjson",
			"application/yaml",
			"application/x-yaml",
			"application/javascript":
			return true
		}
		return false
	}
	return truthyTextExtension(rawURL)
}

var textResultExtensions = map[string]bool{
	"txt": true, "md": true, "markdown": true, "json": true, "csv": true,
	"tsv": true, "log": true, "srt": true, "vtt": true, "xml": true,
	"yml": true, "yaml": true,
}

func truthyTextExtension(rawURL string) bool {
	path := rawURL
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		path = path[:idx]
	}
	dot := strings.LastIndex(path, ".")
	if dot < 0 || dot == len(path)-1 {
		return false
	}
	return textResultExtensions[strings.ToLower(path[dot+1:])]
}

// taskOwnsResultURL reports whether rawURL is one of the task's own result URLs.
// It is the guard that keeps the preview endpoint from becoming an arbitrary
// fetch proxy.
func taskOwnsResultURL(task *model.Task, rawURL string) bool {
	if task == nil {
		return false
	}
	if task.PrivateData.ResultURL != "" && task.PrivateData.ResultURL == rawURL {
		return true
	}
	if len(task.Data) == 0 {
		return false
	}
	var payload struct {
		Results []taskResultEntry `json:"results"`
		URL     string            `json:"url"`
		Value   string            `json:"value"`
	}
	if err := common.Unmarshal(task.Data, &payload); err == nil {
		for _, entry := range payload.Results {
			if resultEntryMatches(entry, rawURL) {
				return true
			}
		}
		if payload.URL == rawURL || payload.Value == rawURL {
			return true
		}
	}
	// Some payloads store the results array at the top level.
	var bare []taskResultEntry
	if err := common.Unmarshal(task.Data, &bare); err == nil {
		for _, entry := range bare {
			if resultEntryMatches(entry, rawURL) {
				return true
			}
		}
	}
	return false
}

func resultEntryMatches(entry taskResultEntry, rawURL string) bool {
	return (entry.URL != "" && entry.URL == rawURL) ||
		(entry.Value != "" && entry.Value == rawURL)
}
