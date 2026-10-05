package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/oss_setting"

	"github.com/gin-gonic/gin"
)

// TestOSSConnection handles POST /api/option/oss/test by writing and removing a
// probe object, so the operator can verify credentials and write permission
// before enabling Aliyun OSS storage.
func TestOSSConnection(c *gin.Context) {
	setting := oss_setting.GetOSSSetting()
	if err := setting.Validate(); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := service.TestOSSConnection(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"endpoint": setting.SDKEndpoint(),
		"bucket":   setting.BucketName(),
	})
}
