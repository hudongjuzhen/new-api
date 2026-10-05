package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/oss_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useTestOSSSetting points the global OSS configuration at a local test server
// and restores the previous configuration when the test finishes.
func useTestOSSSetting(t *testing.T, endpoint string) {
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
	setting.CustomDomain = ""
	setting.UseSSL = false
}

func TestUploadOSSObjectSendsSignedPutRequest(t *testing.T) {
	var (
		method        string
		path          string
		body          string
		contentType   string
		authorization string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		contentType = r.Header.Get("Content-Type")
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	useTestOSSSetting(t, strings.TrimPrefix(server.URL, "http://"))

	err := UploadOSSObject("uploads/images/202601/a.png", strings.NewReader("image-bytes"), "image/png")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPut, method)
	assert.Equal(t, "/test-bucket/uploads/images/202601/a.png", path)
	assert.Equal(t, "image-bytes", body)
	assert.Equal(t, "image/png", contentType)
	assert.True(t, strings.HasPrefix(authorization, "OSS test-ak:"),
		"request should be signed with the configured access key, got %q", authorization)
}

func TestUploadOSSObjectRejectsIncompleteConfigurationWithoutRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	setting := oss_setting.GetOSSSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })

	// Endpoint 与 Bucket 已配置，但密钥缺失
	*setting = oss_setting.OSSSetting{
		Enabled:  true,
		Endpoint: strings.TrimPrefix(server.URL, "http://"),
		Bucket:   "test-bucket",
	}

	err := UploadOSSObject("uploads/images/202601/a.png", strings.NewReader("image-bytes"), "image/png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AccessKey ID")
	assert.Zero(t, requests, "配置不完整时不应向 OSS 发出请求")
}

func TestTestOSSConnectionRemovesProbeObject(t *testing.T) {
	var methods []string
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.Path)
		if r.Method == http.MethodDelete {
			// OSS 删除对象返回 204 No Content
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	useTestOSSSetting(t, strings.TrimPrefix(server.URL, "http://"))

	require.NoError(t, TestOSSConnection())

	assert.Equal(t, []string{http.MethodPut, http.MethodDelete}, methods)
	require.Len(t, paths, 2)
	assert.Equal(t, paths[0], paths[1], "探测对象写入与删除必须使用同一个对象键")
	assert.True(t, strings.HasPrefix(paths[0], "/test-bucket/uploads/.newapi-oss-test-"),
		"探测对象应位于配置的前缀下, got %q", paths[0])
}
