package channel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

// maxMeasuredAudioBytes bounds how much of a produced audio file a metered
// adaptor reads back to measure its duration. Header-only containers (wav, mp3,
// m4a) need far less; the ceiling exists so a hostile or mislabelled response
// cannot stream without limit.
const maxMeasuredAudioBytes = 4 << 20

// MeasureAudioDuration fetches a produced audio file and returns its duration
// in seconds.
//
// It serves tasks whose charge follows a quantity the upstream never reports:
// a model billed per minute of generated audio can read back what it produced
// and bill the real length instead of the submit-time estimate.
//
// The fetch goes through the SSRF-protected download path and reads at most
// maxMeasuredAudioBytes. Any failure — unreachable URL, unsupported container,
// unparseable header — returns an error, and the caller must then keep its
// previous charge: a measurement failure may never silently become a different
// amount.
func MeasureAudioDuration(rawURL string) (float64, error) {
	if rawURL == "" {
		return 0, fmt.Errorf("empty audio url")
	}
	resp, err := service.DoDownloadRequest(rawURL, "measure metered audio duration")
	if err != nil {
		return 0, fmt.Errorf("fetch audio: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("fetch audio: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMeasuredAudioBytes))
	if err != nil {
		return 0, fmt.Errorf("read audio: %w", err)
	}
	duration, err := common.GetAudioDuration(
		context.Background(),
		bytes.NewReader(body),
		normalizeAudioExt(rawURL, resp.Header.Get("Content-Type")),
	)
	if err != nil {
		return 0, err
	}
	if !(duration > 0) {
		return 0, fmt.Errorf("measured duration is not positive: %v", duration)
	}
	return duration, nil
}

// normalizeAudioExt picks the container extension common.GetAudioDuration
// dispatches on, preferring the URL's own extension and falling back to the
// response content type.
func normalizeAudioExt(rawURL string, contentType string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		if ext := strings.ToLower(path.Ext(parsed.Path)); ext != "" {
			return ext
		}
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return strings.TrimPrefix(mimeType, "audio/")
}
