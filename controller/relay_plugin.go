package controller

import (
	"fmt"
	"net/http"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Relay helpers exported for plugin packages that need to re-use the host's
// retry / channel-selection / channel-error plumbing without reimplementing
// the semantics (especially the cross-database and ban/affinity rules).

// AddUsedChannel records a channel id onto the request's use_channel trace,
// exactly as the host's own task-relay loop does.
func AddUsedChannel(c *gin.Context, channelId int) {
	addUsedChannel(c, channelId)
}

// GetChannelForRelay runs the host's standard "pick a channel for this
// (model, token_group)" flow, including retry-parameter accounting and
// multi-key / auto-ban handling.
func GetChannelForRelay(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	return getChannel(c, info, retryParam)
}

// ProcessChannelError reports an upstream error into the channel's failure
// counter and (optionally) auto-ban pipeline. If err is nil a default
// NewAPIError is synthesised from the ChannelError identity so the logging
// pipeline still sees a properly shaped preview payload.
func ProcessChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError) {
	if err == nil {
		err = types.NewError(
			fmt.Errorf("channel #%d error", channelError.ChannelId),
			types.ErrorCodeDoRequestFailed,
			types.ErrOptionWithStatusCode(http.StatusBadGateway),
		)
	}
	processChannelError(c, channelError, err)
}

// ShouldRetryTaskRelay returns true when the host's task-relay retry rules
// indicate another submission attempt should be made after the given error.
func ShouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *taskdto.TaskError, retryTimes int) bool {
	return shouldRetryTaskRelay(c, channelId, taskErr, retryTimes)
}
