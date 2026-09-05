package runninghub

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Upload proxy (user-side)
// ---------------------------------------------------------------------------

// maxUploadBytes is the accepted single-file size for the media upload proxy.
// Dev plan §3.6 set a 50MB ceiling; keep the limit identical to what the
// upstream RunningHub endpoint documents.
const maxUploadBytes = 50 << 20 // 50MB

// resolveUploadChannel picks the RunningHub channel that serves the app's
// site. Apps no longer pin a channel: the site field (cn/intl) is the routing
// source of truth, so the first enabled channel of the site's type wins. A
// nil result with a nil error means no enabled channel exists for the site.
func resolveUploadChannel(site string) (*model.Channel, error) {
	channelType := siteToChannelType(site)
	if channelType == 0 {
		return nil, fmt.Errorf("非法的站点选择: %q", site)
	}
	ch, err := model.GetFirstEnabledChannelByType(channelType)
	if err != nil {
		return nil, fmt.Errorf("查询站点渠道失败: %w", err)
	}
	return ch, nil
}

// uploadAppMedia (POST /api/zsy/rh/upload) — forwards one multipart file to
// the RunningHub site the app's `site` field declares and returns the upstream
// fileName (e.g. "openapi/xxxx.png"), which callers then send back as
// nodeInfoList[].fieldValue for image/video/audio inputs.
//
//   - The upstream target is {base_url}/openapi/v2/media/upload/binary derived
//     from the resolved channel's base URL — never from the request body. This
//     keeps the SSRF surface closed (dev plan §3.6).
//   - The caller passes ?site=cn|intl; the first enabled channel of that
//     site's type is used (apps do not bind channels any more).
//   - Only RunningHub-family channel types (61/62/63) are accepted.
//
// Response: { fileName, url } where url is the fetchable URL on the same RH
// host (RH media files are not reachable through this gateway).
func uploadAppMedia(c *gin.Context) {
	site := strings.TrimSpace(c.Query("site"))
	ch, err := resolveUploadChannel(site)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if ch == nil {
		common.ApiErrorMsg(c, "该站点暂无可用渠道，无法上传")
		return
	}

	// Bound the request body before reading the file part.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes+(64<<10))
	file, header, err := c.Request.FormFile(UploadFieldWorkflow)
	if err != nil {
		common.ApiErrorMsg(c, "上传失败: 缺少 file 字段或文件超过 50MB")
		return
	}
	defer func() { _ = file.Close() }()

	client := NewClientForType(ch.Type, ch.GetBaseURL(), ch.Key, nil)
	resp, err := client.UploadBinary(UploadFieldWorkflow, header.Filename, file)
	if err != nil {
		common.ApiErrorMsg(c, "上传到 RunningHub 失败: "+err.Error())
		return
	}
	fileName := strings.TrimSpace(resp.Data.FileName)
	downloadURL := strings.TrimSpace(resp.Data.DownloadURL)
	if fileName == "" && downloadURL == "" {
		common.ApiErrorMsg(c, "RunningHub 未返回有效的上传结果")
		return
	}
	if fileName == "" {
		// Model-API style responses only carry download_url; the workflow still
		// needs a fileName-style value, so fall back to the URL.
		fileName = downloadURL
	}
	common.ApiSuccess(c, gin.H{
		"fileName": fileName,
		"url":      downloadURL,
	})
}

// getUploadChannelStatus (GET /api/zsy/rh/upload-channel?site=cn|intl) —
// reports whether at least one enabled RunningHub channel exists for the
// site, so the portal can enable the media upload zone without probing a
// per-app channel binding.
func getUploadChannelStatus(c *gin.Context) {
	site := strings.TrimSpace(c.Query("site"))
	channelType := siteToChannelType(site)
	if channelType == 0 {
		common.ApiErrorMsg(c, fmt.Sprintf("非法的站点选择: %q", site))
		return
	}
	count, err := model.CountEnabledChannelsByType(channelType)
	if err != nil {
		common.ApiError(c, fmt.Errorf("查询站点渠道失败: %w", err))
		return
	}
	common.ApiSuccess(c, gin.H{
		"available": count > 0,
		"count":     count,
	})
}
