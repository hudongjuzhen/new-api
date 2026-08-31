package controller

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPngMagic = "\x89PNG\r\n\x1a\n"

func newUploadImageRequest(t *testing.T, fieldName string, content []byte, contentType string) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if fieldName != "" {
		part, err := writer.CreateFormFile(fieldName, "test-image.png")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	request, err := http.NewRequest(http.MethodPost, "/api/upload/image", body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	_ = contentType
	return request
}

func performUploadImage(t *testing.T, request *http.Request) (bool, string, map[string]any) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	UploadImage(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload.Success, payload.Message, payload.Data
}

func TestUploadImageStoresFileInMonthlyFolder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())

	png := []byte(testPngMagic + "rest-of-image-payload")
	request := newUploadImageRequest(t, "file", png, "image/png")
	success, _, data := performUploadImage(t, request)

	require.True(t, success)
	url, ok := data["url"].(string)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(url, "/uploads/images/"), "url should live under /uploads/images/, got %s", url)

	monthDir := time.Now().Format("200601")
	assert.Contains(t, url, "/uploads/images/"+monthDir+"/")

	stored := filepath.FromSlash(strings.TrimPrefix(url, "/"))
	written, err := os.ReadFile(stored)
	require.NoError(t, err)
	assert.Equal(t, png, written)
}

func TestUploadImageRejectsNonImageContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	t.Chdir(dir)

	request := newUploadImageRequest(t, "file", []byte("plain text, not an image"), "")
	success, message, _ := performUploadImage(t, request)

	assert.False(t, success)
	assert.NotEmpty(t, message)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no file should be written for rejected uploads")
}

func TestUploadImageRejectsMissingFileField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())

	request := newUploadImageRequest(t, "", nil, "")
	success, message, _ := performUploadImage(t, request)

	assert.False(t, success)
	assert.NotEmpty(t, message)
}
