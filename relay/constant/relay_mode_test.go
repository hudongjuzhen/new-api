package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath2RelayMode(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/v1/alpha/search", want: RelayModeAlphaSearch},
		{path: "/v1/alpha/search?foo=1", want: RelayModeAlphaSearch},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
}

// TestAudioGenerationPathMapsToItsOwnRelayMode guards the route/adaptor pairing:
// the audio-generation handler and the doubao-audio adaptor both key off this
// relay mode, so a path that resolved to the TTS mode would build a request the
// upstream audio service rejects.
func TestAudioGenerationPathMapsToItsOwnRelayMode(t *testing.T) {
	assert.Equal(t, RelayModeAudioGenerations, Path2RelayMode("/v1/audio/generations"))

	// The OpenAI TTS surface must keep its own mode: it is a different contract.
	assert.Equal(t, RelayModeAudioSpeech, Path2RelayMode("/v1/audio/speech"))
}
