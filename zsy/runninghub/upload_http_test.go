package runninghub_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newUploadTestRouter builds a gin router exposing only the upload proxy
// handler. The proxy reads the pinned channel from the DB only *after* the
// route parameter parses; a malformed channel id short-circuits before any
// store or upstream access, keeping this test DB-free.
func newUploadTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/zsy/rh/upload/:channelId", runninghub.TestHookUploadAppMedia)
	return r
}

// multipartBody builds a single-file multipart body for the given field/value.
func multipartBody(t *testing.T, field, filename, value string) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	fw, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = fw.Write([]byte(value))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &b, mw.FormDataContentType()
}

// TestUploadAppMediaRejectsMalformedChannelID verifies the route's first
// guard: a non-numeric channel id is rejected with a business error before any
// channel lookup or upstream connection.
func TestUploadAppMediaRejectsMalformedChannelID(t *testing.T) {
	r := newUploadTestRouter()
	body, contentType := multipartBody(t, "file", "a.png", "payload")
	req := httptest.NewRequest(http.MethodPost, "/api/zsy/rh/upload/not-a-number", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.False(t, parseAPIEnvelope(t, w.Body.Bytes()).Success)
}