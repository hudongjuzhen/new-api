package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

// 新注册账号自动获得的默认密钥：名称与分组都固定为「官方渠道」，其余字段全部
// 取面板新建 Key 的默认值（启用、无限额度、永不过期、不限制模型与 IP）。
const (
	defaultUserTokenName  = "官方渠道"
	defaultUserTokenGroup = "官方渠道"
)

// provisionDefaultUserKey 为新注册账号创建默认密钥，所有自助注册入口
// （账号密码注册、OAuth 注册、微信注册）共用同一条逻辑。
//
// 建键失败不会让注册失败：账号已经落库，用户仍可在面板自行创建 Key，因此这里
// 只记录后端日志。当「官方渠道」分组对用户所属分组不可用时同样只告警——该 Key
// 的中继请求会被鉴权拒绝（403），需要管理员补齐分组配置。
func provisionDefaultUserKey(userId int, userGroup string) {
	token, err := model.CreateDefaultUserToken(userId, defaultUserTokenName, defaultUserTokenGroup)
	if err != nil {
		common.SysError(fmt.Sprintf("为新用户 %d 创建默认密钥失败: %s", userId, err.Error()))
		return
	}
	if !service.IsUserSelectableGroup(userGroup, token.Group) {
		common.SysError(fmt.Sprintf(
			"新用户 %d 的默认密钥分组 %q 对用户分组 %q 不可用：请确认「分组倍率」中存在该分组并在「用户可用分组」中放开，否则该 Key 的中继请求会返回 403",
			userId, token.Group, userGroup,
		))
	}
}
