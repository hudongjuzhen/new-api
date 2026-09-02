package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChannelBaseURLsCoverAllChannelTypes guards the count-sentinel contract of
// ChannelTypeDummy: the task-polling path indexes ChannelBaseURLs[ch.Type]
// directly (service/task_polling.go updateVideoSingleTask), so a channel type
// without a matching array entry panics at runtime instead of surfacing an
// error.
func TestChannelBaseURLsCoverAllChannelTypes(t *testing.T) {
	assert.Equal(t, ChannelTypeDummy, len(ChannelBaseURLs),
		"every channel type must have a ChannelBaseURLs entry (see ChannelTypeDummy)")
	assert.NotEmpty(t, ChannelBaseURLs[ChannelTypeRunningHub])
	assert.NotEmpty(t, ChannelBaseURLs[ChannelTypeRunningHubIntl])
	assert.NotEmpty(t, ChannelBaseURLs[ChannelTypeLiblib])
	assert.Equal(t, "https://www.runninghub.ai", ChannelBaseURLs[ChannelTypeRunningHubIntl])
	assert.Equal(t, "https://openapi.liblibai.cloud", ChannelBaseURLs[ChannelTypeLiblib])
}
