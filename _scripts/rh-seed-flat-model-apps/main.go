// One-off seeder: inserts the two flat Model-API RunningHub apps
// (rh-upscale/enhance-frame, minimax-h3-rh-enhanced/ref2va) into the rh_apps
// table through the plugin's own store layer, so validation and the
// ModelPrice sync behave exactly like an admin save from the web UI.
//
// It replicates the host boot sequence (env → ratio settings → InitDB →
// InitOptionMap) because syncAppBillingPrice merges into the LIVE model price
// map read from the options table; seeding without loading it first would
// wipe every other model's price.
//
// Usage:
//
//	go run ./_scripts/rh-seed-flat-model-apps            # insert, skip existing
//	go run ./_scripts/rh-seed-flat-model-apps -dry      # print what would happen
//
// SQL_DSN comes from the process env or .env.
//
// NOTE: the billing numbers below are placeholders — adjust them in the admin
// UI (应用管理 → 编辑) once the real RH prices for these two APIs are known.
// The new apps' prices only become visible to a RUNNING server after it
// reloads options (restart, or an admin options save).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
)

func opt(label, value string) rhparser.SchemaParamOption {
	return rhparser.SchemaParamOption{Label: label, Value: value}
}

func upscaleSchema() []rhparser.SchemaParam {
	return []rhparser.SchemaParam{
		{FieldName: "fileUrl", Label: "fileUrl（待处理视频 URL）", Type: "video", Required: true,
			Placeholder: "填写可公网访问的视频地址"},
		{FieldName: "model", Label: "model（模型）", Type: "select", Required: true, Default: "max",
			Options: []rhparser.SchemaParamOption{opt("max（推荐）", "max")}},
		{FieldName: "resolution", Label: "resolution（目标分辨率）", Type: "select", Default: "1080p",
			Options: []rhparser.SchemaParamOption{opt("1080p", "1080p")}},
		{FieldName: "outputFps", Label: "outputFps（补帧倍率）", Type: "select", Default: "1x",
			Options: []rhparser.SchemaParamOption{opt("1x", "1x")}},
	}
}

func ref2vaSchema() []rhparser.SchemaParam {
	schema := []rhparser.SchemaParam{
		{FieldName: "prompt", Label: "prompt（故事主题 / 叙事线）", Type: "textarea"},
		{FieldName: "aspectRatio", Label: "aspectRatio（画幅）", Type: "select", Default: "9:16 (Portrait Widescreen)",
			Options: []rhparser.SchemaParamOption{opt("9:16 (Portrait Widescreen)", "9:16 (Portrait Widescreen)")}},
		{FieldName: "resolution", Label: "resolution（分辨率）", Type: "select", Required: true, Default: "1080p",
			Options: []rhparser.SchemaParamOption{opt("1080p", "1080p")}},
		{FieldName: "duration", Label: "duration（时长，秒）", Type: "number", Required: true, Default: "10"},
		{FieldName: "audioMode", Label: "audioMode（音频模式）", Type: "select", Required: true, Default: "native（模型自己出声）",
			Options: []rhparser.SchemaParamOption{opt("native（模型自己出声）", "native（模型自己出声）")}},
		{FieldName: "driveAudio", Label: "driveAudio（驱动音频）", Type: "audio"},
	}
	for i := 1; i <= 9; i++ {
		schema = append(schema, rhparser.SchemaParam{
			FieldName: fmt.Sprintf("refImage%d", i), Label: fmt.Sprintf("refImage%d（参考图 %d）", i, i), Type: "image"})
	}
	for i := 1; i <= 3; i++ {
		schema = append(schema, rhparser.SchemaParam{
			FieldName: fmt.Sprintf("refVideo%d", i), Label: fmt.Sprintf("refVideo%d（参考视频 %d）", i, i), Type: "video"})
	}
	for i := 1; i <= 3; i++ {
		schema = append(schema, rhparser.SchemaParam{
			FieldName: fmt.Sprintf("refAudio%d", i), Label: fmt.Sprintf("refAudio%d（参考音频 %d）", i, i), Type: "audio"})
	}
	for i := 1; i <= 2; i++ {
		schema = append(schema, rhparser.SchemaParam{
			FieldName: fmt.Sprintf("refVideoAudio%d", i), Label: fmt.Sprintf("refVideoAudio%d（参考视频音频 %d）", i, i), Type: "audio"})
	}
	return schema
}

func seedApps() []runninghub.AppCreateDTO {
	return []runninghub.AppCreateDTO{
		{
			Name:              "视频超分增强 (RH Upscale)",
			Slug:              "rh-upscale-enhance-frame",
			Kind:              runninghub.AppKindModel,
			UpstreamID:        "/rhart-video/rh-upscale/enhance-frame",
			Description:       "视频超分与插帧一体接口：指定目标分辨率与增强强度，按倍率补帧。fileUrl 传入待处理视频 URL。",
			Published:         true,
			Site:              "cn",
			ParamSchema:       upscaleSchema(),
			PerCallBilling:    true,
			FixedQuotaPerCall: 150000, // ≈$0.30/次 占位价，请按实际价格调整
		},
		{
			Name:             "Minimax H3 多参考视频 (RH Enhanced)",
			Slug:             "minimax-h3-rh-enhanced-ref2va",
			Kind:             runninghub.AppKindModel,
			UpstreamID:       "/rhart-video/minimax-h3-rh-enhanced/ref2va",
			Description:      "基于 MiniMax H3 的多参考视频生成：最多 9 张参考图 + 3 路参考视频 + 多路音频，驱动音频与时长必填。",
			Published:        true,
			Site:             "cn",
			ParamSchema:      ref2vaSchema(),
			PerSecondBilling: true,
			QuotaPerSecond:   75000, // ≈$0.15/秒 占位价，请按实际价格调整
			SecondsExpr:      "@duration",
		},
	}
}

func main() {
	dry := flag.Bool("dry", false, "only print what would be inserted")
	flag.Parse()

	_ = godotenv.Load(".env")
	common.InitEnv()
	logger.SetupLogger()
	ratio_setting.InitRatioSettings()
	if err := model.InitDB(); err != nil {
		fmt.Fprintln(os.Stderr, "init db failed:", err)
		os.Exit(1)
	}
	model.InitOptionMap()

	for _, dto := range seedApps() {
		existing, err := runninghub.AppSearch(runninghub.AppListQuery{
			Keyword: dto.Name, Page: 1, PageSize: 5,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "search apps failed:", err)
			os.Exit(1)
		}
		dup := false
		for _, item := range existing.Items {
			if item.Name == dto.Name {
				dup = true
				break
			}
		}
		if dup {
			fmt.Printf("SKIP %q 已存在\n", dto.Name)
			continue
		}
		if *dry {
			fmt.Printf("DRY 新建 %q kind=%s upstreamId=%s site=%s perCall=%v(%d) perSecond=%v(%d/s, expr=%s)\n",
				dto.Name, dto.Kind, dto.UpstreamID, dto.Site,
				dto.PerCallBilling, dto.FixedQuotaPerCall,
				dto.PerSecondBilling, dto.QuotaPerSecond, dto.SecondsExpr)
			continue
		}
		view, err := runninghub.AppInsert(&dto)
		if err != nil {
			fmt.Fprintf(os.Stderr, "insert %q failed: %v\n", dto.Name, err)
			os.Exit(1)
		}
		fmt.Printf("OK   %q id=%d upstreamId=%s schema字段=%d\n",
			view.Name, view.ID, view.UpstreamID, len(view.ParamSchema))
	}
}
