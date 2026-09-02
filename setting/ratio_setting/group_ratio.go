package ratio_setting

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

// hexColorRegex 匹配 6 位十六进制颜色值（#RRGGBB）
var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
	GroupColor              *types.RWMap[string, string]             `json:"group_color"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupColor := types.NewRWMap[string, string]()

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupColor:              groupColor,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.GroupColor == nil {
		groupRatioSetting.GroupColor = types.NewRWMap[string, string]()
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}

func GetGroupColorCopy() map[string]string {
	groupRatioSetting := GetGroupRatioSetting()
	return groupRatioSetting.GroupColor.ReadAll()
}

func GroupColor2JSONString() string {
	return GetGroupRatioSetting().GroupColor.MarshalJSONString()
}

func UpdateGroupColorByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(GetGroupRatioSetting().GroupColor, jsonStr)
}

// CheckGroupColor 校验分组颜色配置：JSON 对象 { "group": "#hex" }，
// 每个值必须是 6 位十六进制颜色（#RRGGBB）或空字符串（空串表示恢复自动配色）。
func CheckGroupColor(jsonStr string) error {
	checkGroupColor := make(map[string]json.RawMessage)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupColor)
	if err != nil {
		return err
	}
	for name, raw := range checkGroupColor {
		trimmed := strings.TrimSpace(string(raw))
		// null 表示恢复自动配色
		if trimmed == "null" {
			continue
		}
		var color string
		if err := json.Unmarshal([]byte(trimmed), &color); err != nil {
			return errors.New("invalid group color for " + name + ": " + trimmed)
		}
		// 空字符串表示恢复自动配色
		if color == "" {
			continue
		}
		if !hexColorRegex.MatchString(color) {
			return errors.New("invalid group color for " + name + ": " + color)
		}
	}
	return nil
}
