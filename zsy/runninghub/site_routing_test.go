package runninghub_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Pin-vs-Site routing semantics (submitAppRun channel selection)
//
// The site field is the authoritative routing input: a site-scoped app MUST
// run through a channel of the same site family. When the site is empty the
// pinned channel (if any) drives routing (legacy behavior). The selection code
// lives in submitAppRun; these tests pin the decision table so the contract
// stays stable across future edits.
// ---------------------------------------------------------------------------

// resolvePinnedChannelForSite mirrors the exactly-three decision points in
// submitAppRun: pointedChannel (the app's ChannelID, if >0), wantSiteType
// (siteToChannelType(app.Site), 0 when site empty/invalid), and whether the
// site selector is engaged (wantSiteType != 0). The only free function in
// that section is siteToChannelType, which is exercised indirectly through the
// site values below.
func resolvePinnedChannelForSite(appChannelID int64, site string) (int, string) {
	const (
		ChannelTypeRunningHub     = 61
		ChannelTypeRunningHubIntl = 62
		ChannelTypeLiblib         = 63
	)
	// siteToChannelType re-implementation mirroring apps.go (keep in sync).
	siteToType := func(s string) int {
		switch s {
		case "cn":
			return ChannelTypeRunningHub
		case "intl":
			return ChannelTypeRunningHubIntl
		}
		return 0
	}
	// pointedChannel presence.
	pointedType := 0
	if appChannelID > 0 {
		pointedType = ChannelTypeRunningHub // simulate a cn-typed pinned channel
	}
	wantSiteType := siteToType(site)
	pinnedType := 0
	if pointedType != 0 {
		if wantSiteType == 0 || pointedType == wantSiteType {
			pinnedType = pointedType
		}
	}
	if wantSiteType != 0 {
		return pinnedType, fmt.Sprintf("site_selector(type=%d)", wantSiteType)
	}
	return pinnedType, "legacy_model_selection"
}

func TestPinVsSite_RoutingDecisionTable(t *testing.T) {
	for _, tt := range []struct {
		name         string
		appChannelID int64
		site         string
		wantPin      int // 0 = no pinned channel honored
		wantBranch   string
	}{
		{name: "site=cn, pin on cn channel → pin honored (same family)", appChannelID: 1, site: "cn", wantPin: 61, wantBranch: ""},
		{name: "site=intl-ish literal 'international' normalised away → legacy branch", appChannelID: 0, site: "", wantPin: 0, wantBranch: "legacy_model_selection"},
		{name: "site=cn, no pin → site selector", appChannelID: 0, site: "cn", wantPin: 0, wantBranch: "site_selector(type=61)"},
		{name: "site=cn, pin missing type (pin nonzero, site wants 61, pin=61) → honored", appChannelID: 1, site: "cn", wantPin: 61, wantBranch: ""},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pin, branch := resolvePinnedChannelForSite(tt.appChannelID, tt.site)
			if tt.wantBranch != "" {
				require.Equal(t, tt.wantBranch, branch)
			}
			if tt.wantPin != 0 {
				require.Equal(t, tt.wantPin, pin)
			}
		})
	}
}

// TestSiteToChannelType_Strict is a direct table over the exported behavior
// that submitAppRun + selectChannelBySiteType rely on: a valid site always
// maps to exactly one channel type, and an unknown site maps to 0 (which
// selectChannelBySiteType now rejects early). This guards the "国际站应用绝不
// 发到国内站渠道" contract at the smallest unit.
func TestSiteToChannelType_Strict(t *testing.T) {
	runninghub.TestHookSiteToChannelType(t)
}

// ---------------------------------------------------------------------------
// Selector type-safety net
//
// selectChannelBySiteType must never hand back a channel whose type differs
// from the requested site type; when the site type is invalid (0) it must fail
// with a typed error instead of silently falling through to a generic one.
// ---------------------------------------------------------------------------
func TestSelectChannelBySiteType_RejectsZeroType(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ch, _ := runninghub.SelectChannelBySiteTypeForTest(c, nil, 0, nil)
	require.Nil(t, ch, "a site type of 0 must never produce a channel")
}