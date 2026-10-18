package runninghub_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 每个站点各自的上游ID与标识符
//
// 同一个工作流在国内站和国际站是两个不同的上游应用 ID，应用表因此按站点保存
// 一对 "上游ID / 标识符"，提交时用"这次跑在哪个站点"的那一对。这里钉住的是：
// 谁能选到哪一对、主键（模型名 / 价格表键）由谁决定、以及配置错位时必须在保存
// 阶段就被拒绝，而不是等到上游返回一个看不懂的错误。
// ---------------------------------------------------------------------------

// dualSiteAppDTO is an app registered on both sites, each with its own ids.
func dualSiteAppDTO(name, site string) runninghub.AppCreateDTO {
	return runninghub.AppCreateDTO{
		Name: name,
		Kind: runninghub.AppKindAICApp,
		Site: site,
		SiteUpstreams: map[string]runninghub.AppSiteUpstream{
			"cn":   {UpstreamID: "cn-app-id", Slug: "cn-label"},
			"intl": {UpstreamID: "intl-app-id", Slug: "intl-label"},
		},
		Published:          true,
		PerCallBilling:     true,
		FixedQuotaPerCall:  1_000_000,
		ModelBaseRateRatio: 1.0,
	}
}

// TestAppInsert_PrimaryIdentifierFollowsTheAppsOwnSite pins the derived primary
// id: the app is billed and routed under the identifier of the site it runs on,
// while both sites stay available for the submit path to pick from.
func TestAppInsert_PrimaryIdentifierFollowsTheAppsOwnSite(t *testing.T) {
	env := newRHITestEnv(t, "site-upstream-test")

	view, err := runninghub.AppInsert(ptrDTO(dualSiteAppDTO("双站点应用", "cn")))
	require.NoError(t, err)

	assert.Equal(t, "cn-app-id", view.UpstreamID, "主上游ID必须是本应用所在站点的ID")
	assert.Equal(t, "cn-label", view.Slug, "标识符同样跟着站点走")
	assert.Equal(t, "cn-app-id", view.SiteUpstreams["cn"].UpstreamID)
	assert.Equal(t, "intl-app-id", view.SiteUpstreams["intl"].UpstreamID,
		"另一站点的ID必须原样保存，切换站点后才能直接使用")
	_ = env
}

// TestAppUpdate_SwitchingSiteReKeysThePrimary is the same rule on an edit: the
// model name a run is billed under changes with the site, because the app now
// submits to a different upstream app.
func TestAppUpdate_SwitchingSiteReKeysThePrimary(t *testing.T) {
	newRHITestEnv(t, "site-upstream-test")

	created, err := runninghub.AppInsert(ptrDTO(dualSiteAppDTO("换站点应用", "cn")))
	require.NoError(t, err)
	require.Equal(t, "cn-app-id", created.UpstreamID)

	updated, err := runninghub.AppUpdate(created.ID, ptrDTO(dualSiteAppDTO("换站点应用", "intl")))
	require.NoError(t, err)

	assert.Equal(t, "intl-app-id", updated.UpstreamID,
		"站点改成国际站后，主上游ID必须变成国际站的ID，否则请求会带着国内站ID打到国际站")
	assert.Equal(t, "cn-app-id", updated.SiteUpstreams["cn"].UpstreamID,
		"国内站的ID仍然保留")
}

// TestAppUpdate_LegacyClientKeepsTheOtherSitesIdentifiers pins backward
// compatibility: an API client that only knows the historical single
// `upstreamId` field can still update an app, and doing so must not wipe the
// identifiers configured for the other sites.
func TestAppUpdate_LegacyClientKeepsTheOtherSitesIdentifiers(t *testing.T) {
	newRHITestEnv(t, "site-upstream-test")

	created, err := runninghub.AppInsert(ptrDTO(dualSiteAppDTO("旧接口应用", "cn")))
	require.NoError(t, err)

	legacy := runninghub.AppCreateDTO{
		Name:               "旧接口应用",
		Kind:               runninghub.AppKindAICApp,
		Site:               "cn",
		UpstreamID:         "cn-app-id-v2",
		Published:          true,
		PerCallBilling:     true,
		FixedQuotaPerCall:  1_000_000,
		ModelBaseRateRatio: 1.0,
	}
	updated, err := runninghub.AppUpdate(created.ID, &legacy)
	require.NoError(t, err)

	assert.Equal(t, "cn-app-id-v2", updated.UpstreamID, "旧字段仍然按本应用站点生效")
	assert.Equal(t, "intl-app-id", updated.SiteUpstreams["intl"].UpstreamID,
		"只带单个上游ID的调用不能把国际站的配置清掉")
}

// TestValidateApp_SiteUpstreams covers the configuration mistakes that would
// otherwise only surface as an upstream error at submit time.
func TestValidateApp_SiteUpstreams(t *testing.T) {
	cases := []struct {
		name    string
		mutator func(dto *runninghub.AppCreateDTO)
		wantErr string
	}{
		{
			name: "站点已选但该站点没有上游ID",
			mutator: func(dto *runninghub.AppCreateDTO) {
				dto.Site = "cn"
				dto.SiteUpstreams = map[string]runninghub.AppSiteUpstream{
					"intl": {UpstreamID: "intl-app-id"},
				}
			},
			wantErr: "必须填写该站点的上游ID",
		},
		{
			name: "未注册的站点键",
			mutator: func(dto *runninghub.AppCreateDTO) {
				dto.SiteUpstreams = map[string]runninghub.AppSiteUpstream{
					"mars": {UpstreamID: "mars-app-id"},
				}
			},
			wantErr: "非法的站点",
		},
		{
			name: "未注册的站点选择",
			mutator: func(dto *runninghub.AppCreateDTO) {
				dto.Site = "mars"
			},
			wantErr: "非法的站点选择",
		},
		{
			name: "站点别名的两种写法合并成同一个站点",
			mutator: func(dto *runninghub.AppCreateDTO) {
				dto.Site = "runninghub.cn"
				dto.SiteUpstreams = map[string]runninghub.AppSiteUpstream{
					"cn": {UpstreamID: "cn-app-id"},
				}
			},
			wantErr: "",
		},
		{
			name: "两个站点都配好",
			mutator: func(dto *runninghub.AppCreateDTO) {
				dto.Site = "intl"
				dto.SiteUpstreams = map[string]runninghub.AppSiteUpstream{
					"cn":   {UpstreamID: "cn-app-id"},
					"intl": {UpstreamID: "intl-app-id"},
				}
			},
			wantErr: "",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dto := validAppDTO(t)
			c.mutator(&dto)
			err := runninghub.TestHookValidateAppCreate(&dto)
			if c.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr, "error=%v", err)
		})
	}
}

// TestResolveUpstreamIDForModel_PicksTheChannelsSite pins the submit-side rule:
// a `rh-app-<id>` model resolves to the identifier of the site the request is
// actually running through, so the same app row can serve both sites' channels.
func TestResolveUpstreamIDForModel_PicksTheChannelsSite(t *testing.T) {
	newRHITestEnv(t, "site-upstream-test")

	view, err := runninghub.AppInsert(ptrDTO(dualSiteAppDTO("双站点应用", "cn")))
	require.NoError(t, err)
	model := runninghub.RhAppModelPrefix + strconv.FormatUint(uint64(view.ID), 10)

	for _, tc := range []struct {
		name        string
		channelType int
		want        string
	}{
		{name: "国内站渠道用国内站ID", channelType: constant.ChannelTypeRunningHub, want: "cn-app-id"},
		{name: "国际站渠道用国际站ID", channelType: constant.ChannelTypeRunningHubIntl, want: "intl-app-id"},
		{name: "没有站点上下文时用主上游ID", channelType: 0, want: "cn-app-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runninghub.TestHookResolveUpstreamIDForModel(model, tc.channelType)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestAppView_LegacyRowReportsItsIdentifierOnItsSite pins the migration read
// shape: a row written before per-site identifiers existed carries one id, and
// it is reported under the site the row runs on — the only site it could have
// been submitted to. Nothing else changes for it, so an un-migrated table keeps
// working.
func TestAppView_LegacyRowReportsItsIdentifierOnItsSite(t *testing.T) {
	env := newRHITestEnv(t, "site-upstream-test")

	row := &runninghub.App{
		Name:               "旧应用",
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         "legacy-app-id",
		Slug:               "legacy-label",
		Site:               "cn",
		Published:          true,
		ModelBaseRateRatio: 1.0,
	}
	require.NoError(t, env.db.Create(row).Error)

	view, err := runninghub.AppGetByID(row.ID)
	require.NoError(t, err)

	assert.Equal(t, "legacy-app-id", view.UpstreamID, "旧行仍然按原来的ID计费与路由")
	assert.Equal(t, "legacy-app-id", view.SiteUpstreams["cn"].UpstreamID,
		"旧行的唯一ID要落在它自己所在站点的字段里，管理员打开表单就能看到")
	assert.Equal(t, "legacy-label", view.SiteUpstreams["cn"].Slug)
	assert.Empty(t, view.SiteUpstreams["intl"].UpstreamID,
		"另一个站点没有配置过，必须留空，等管理员自己填")
}

// TestParseCurlEndpoint_ReportsTheSiteOfTheRequest pins the import hint: the
// pasted request says which site it came from, so the form can file its id
// under that site's identifier instead of guessing.
func TestParseCurlEndpoint_ReportsTheSiteOfTheRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/parse-curl", runninghub.TestHookParseCurl)

	for _, tc := range []struct {
		name string
		host string
		want string
	}{
		{name: "国内站", host: "https://www.runninghub.cn", want: "cn"},
		{name: "国际站", host: "https://www.runninghub.ai", want: "intl"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				"curl": `curl --location '` + tc.host + `/openapi/v2/run/ai-app/1877265245566922753' ` +
					`-H 'Authorization: Bearer x' ` +
					`--data '{"nodeInfoList":[{"nodeId":"122","fieldName":"prompt","fieldValue":"a cat"}]}'`,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/parse-curl", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			data, ok := resp["data"].(map[string]any)
			require.True(t, ok, "resp.data missing; body=%s", w.Body.String())
			assert.Equal(t, tc.want, data["site"])
		})
	}
}

// ptrDTO keeps the call sites above readable: AppInsert/AppUpdate take a pointer.
func ptrDTO(dto runninghub.AppCreateDTO) *runninghub.AppCreateDTO { return &dto }
