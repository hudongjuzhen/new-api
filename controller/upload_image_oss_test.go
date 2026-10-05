package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/oss_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useTestOSSSettingForUpload points the global OSS configuration at a local test
// server and restores the previous configuration when the test finishes.
func useTestOSSSettingForUpload(t *testing.T, endpoint string) {
	t.Helper()

	setting := oss_setting.GetOSSSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })

	setting.Enabled = true
	setting.Endpoint = endpoint
	setting.Bucket = "test-bucket"
	setting.AccessKeyID = "test-ak"
	setting.AccessKeySecret = "test-sk"
	setting.PathPrefix = "uploads"
	setting.CustomDomain = "cdn.example.com"
	setting.UseSSL = false
}

func TestUploadImageStoresFileOnOSSWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	localDir := t.TempDir()
	t.Chdir(localDir)

	var (
		path        string
		body        []byte
		contentType string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		body, _ = io.ReadAll(r.Body)
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	useTestOSSSettingForUpload(t, strings.TrimPrefix(server.URL, "http://"))

	png := []byte(testPngMagic + "oss-image-payload")
	success, message, data := performUploadImage(t, newUploadImageRequest(t, "file", png, "image/png"))

	require.True(t, success, message)
	url, ok := data["url"].(string)
	require.True(t, ok)

	monthDir := time.Now().Format("200601")
	assert.True(t, strings.HasPrefix(url, "http://cdn.example.com/uploads/images/"+monthDir+"/"),
		"url should use the configured custom domain, got %s", url)
	assert.True(t, strings.HasPrefix(path, "/test-bucket/uploads/images/"+monthDir+"/"),
		"object should be uploaded under the configured prefix, got %s", path)
	assert.Equal(t, png, body)
	assert.Equal(t, "image/png", contentType)
	assert.Equal(t, float64(len(png)), data["size"])

	entries, err := os.ReadDir(localDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "启用 OSS 后不应再写入本地磁盘")
}

func TestUploadImageFailsWhenOSSConfigurationIsIncomplete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	localDir := t.TempDir()
	t.Chdir(localDir)

	setting := oss_setting.GetOSSSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })
	*setting = oss_setting.OSSSetting{Enabled: true}

	png := []byte(testPngMagic + "oss-image-payload")
	success, message, _ := performUploadImage(t, newUploadImageRequest(t, "file", png, "image/png"))

	assert.False(t, success)
	assert.Contains(t, message, "OSS 配置不完整")

	entries, err := os.ReadDir(localDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "OSS 配置不完整时不应静默回退到本地磁盘")
}
