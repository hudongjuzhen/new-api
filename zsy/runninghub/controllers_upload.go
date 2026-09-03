package runninghub

import (
	"fmt"
	"net/http"
	"strconv"
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

// uploadAppMedia (POST /api/zsy/rh/upload/:channelId) — forwards one
// multipart file to the RunningHub site the bound channel serves and returns
// the upstream fileName (e.g. "openapi/xxxx.png"), which callers then send
// back as nodeInfoList[].fieldValue for image/video/audio inputs.
//
//   - The channel id comes from the route path, not from any user-supplied URL
//     field, so the request can only reach channels the admin configured.
//   - The upstream target is {base_url}/openapi/v2/media/upload/binary derived
//     from the channel's own base URL — never from the request body. This keeps
//     the SSRF surface closed (dev plan §3.6).
//   - Only RunningHub-family channel types (61/62/63) are accepted.
//
// Response: { fileName, url } where url is the fetchable URL on the same RH
// host (RH media files are not reachable through this gateway).
func uploadAppMedia(c *gin.Context) {
	channelID, err := strconv.ParseInt(strings.TrimSpace(c.Param("channelId")), 10, 64)
	if err != nil || channelID <= 0 {
		common.ApiErrorMsg(c, "非法的渠道 ID")
		return
	}
	ch, err := model.GetChannelById(int(channelID), true)
	if err != nil {
		common.ApiErrorMsg(c, "加载渠道失败: "+err.Error())
		return
	}
	if !pluginChannelTypes[ch.Type] {
		common.ApiErrorMsg(c, fmt.Sprintf("渠道 %d 不是 RunningHub 渠道 (type=%d)", channelID, ch.Type))
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
