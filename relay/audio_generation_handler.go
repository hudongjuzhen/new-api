package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// AudioGenerationRequestConverter 是音频创作适配器需要额外实现的方法。
//
// 它没有放进 channel.Adaptor：那个接口是全渠道共用的，为一个渠道加方法会让所有
// 适配器都要补一个空实现。这里按"可选能力"声明，由处理函数断言。
type AudioGenerationRequestConverter interface {
	ConvertAudioGenerationRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioGenerationRequest) (any, error)
}

// AudioGenerationResultKey 是适配器存放上游解析结果的上下文键，必须与
// relay/channel/doubaoaudio 中的定义一致。
const AudioGenerationResultKey = "doubao_audio_result"

// audioGenerationResponse 是返回给调用方的结构。
//
// audio 是 base64 音频，url 是上游的临时地址（2 小时有效），original_duration
// 是本次计费的依据。三个都返回，调用方按自己的时效需求取用。
type audioGenerationResponse struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Data    []audioGenerationResult `json:"data"`
}

type audioGenerationResult struct {
	Audio            string  `json:"audio,omitempty"`
	URL              string  `json:"url,omitempty"`
	Duration         float64 `json:"duration,omitempty"`
	OriginalDuration float64 `json:"original_duration,omitempty"`
	Format           string  `json:"format,omitempty"`
}

// AudioGenerationHelper 处理同步音频生成请求。
//
// 与其它同步媒体接口的区别只有一处：**计费依据来自响应**。模型配的是"每分钟
// 单价"，而真实产出时长只有上游返回后才知道，所以预扣按文本长度估一个保守值
// （由请求的 GetTokenCountMeta 放进 BillingRatios），这里再用上游上报的
// original_duration 重算并结算差额——估多了退，估少了补。
func AudioGenerationHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)

	request, ok := info.Request.(*dto.AudioGenerationRequest)
	if !ok {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.AudioGenerationRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	converter, ok := adaptor.(AudioGenerationRequestConverter)
	if !ok {
		return types.NewError(fmt.Errorf("api type %d does not support audio generation", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	mapped := *request
	if err := helper.ModelMappedHelper(c, info, &mapped); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	converted, err := converter.ConvertAudioGenerationRequest(c, info, mapped)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err := common.Marshal(converted)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, converted)

	var requestBody io.Reader
	body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	requestBody = body

	statusCodeMappingStr := c.GetString("status_code_mapping")

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResp, _ := resp.(*http.Response)

	// 状态码判断留给适配器：上游把业务结果放在 body 里，200 也可能是一次失败。
	if _, newAPIError := adaptor.DoResponse(c, httpResp, info); newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	parsed, ok := c.Get(AudioGenerationResultKey)
	if !ok {
		return types.NewError(errors.New("doubao audio response missing from context"), types.ErrorCodeBadResponseBody)
	}
	result, ok := parsed.(dto.AudioGenerationResponse)
	if !ok {
		return types.NewError(fmt.Errorf("unexpected doubao audio result type %T", parsed), types.ErrorCodeBadResponseBody)
	}

	format := ""
	if mapped.AudioConfig != nil {
		format = mapped.AudioConfig.Format
	}
	responseBody, err := common.Marshal(audioGenerationResponse{
		Code:    "success",
		Message: "ok",
		Data: []audioGenerationResult{{
			Audio:            result.Audio,
			URL:              result.URL,
			Duration:         result.Duration,
			OriginalDuration: result.OriginalDuration,
			Format:           format,
		}},
	})
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	quota, clamp := service.MeteredDurationQuota(info, result.OriginalDuration)
	if clamp != nil && info.QuotaClamp == nil {
		info.QuotaClamp = clamp
	}
	service.SettleMeteredDurationQuota(c, info, quota, result.OriginalDuration)

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	if _, err := c.Writer.Write(responseBody); err != nil {
		logger.LogError(c, "failed to write audio generation response: "+err.Error())
	}
	return nil
}
