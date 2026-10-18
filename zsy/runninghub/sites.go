package runninghub

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// ---------------------------------------------------------------------------
// Site registry
// ---------------------------------------------------------------------------
//
// A "site" is one RunningHub deployment this gateway can reach: the domestic
// site (runninghub.cn), the international site (runninghub.ai), and any site
// added later. The registry below is the ONLY place that knows which sites
// exist, what they are called, and which channel type serves each of them — the
// admin form, the app's per-site upstream identifiers, the submit routing and the
// channel-pool selection all read it instead of hard-coding site tokens.
//
// Adding a site therefore means:
//
//  1. append one SiteSpec here (key + label + channel type + base URL),
//  2. make sure a channel of that type exists on /channels,
//
// and every app row automatically grows an "upstream id / identifier" pair for
// it, both in the admin form and in the store.

// SiteSpec describes one RunningHub site.
type SiteSpec struct {
	// Key is the canonical token stored in App.Site and as the key of the app's
	// per-site identifiers. It never changes once shipped.
	Key string
	// Label is the user-facing site name. It doubles as an i18n key on the
	// frontend, so it must match an entry in the locale files.
	Label string
	// ChannelType is the host channel type that serves this site; it is what
	// the submit path uses to draw a channel from the site's pool.
	ChannelType int
	// DefaultBaseURL is the site's default upstream endpoint. It mirrors
	// constant.ChannelBaseURLs[ChannelType] and is used when a channel carries
	// no base URL of its own.
	DefaultBaseURL string
	// aliases are the extra spellings accepted for this site (historical values,
	// user-facing labels, host names). Lookup is case-insensitive.
	aliases []string
}

// siteRegistry is the ordered site table. Order matters: it is the order the
// admin form shows the per-site identifier blocks in, and the order
// primaryIdentifiers falls back through.
var siteRegistry = []SiteSpec{
	{
		Key:            "cn",
		Label:          "RunningHub",
		ChannelType:    constant.ChannelTypeRunningHub,
		DefaultBaseURL: DefaultBaseURL,
		aliases: []string{
			"runninghub", "runninghub.cn", "runninghub_cn", "www.runninghub.cn",
			"国内", "国内站",
		},
	},
	{
		Key:            "intl",
		Label:          "RunningHub Intl",
		ChannelType:    constant.ChannelTypeRunningHubIntl,
		DefaultBaseURL: DefaultBaseURLIntl,
		aliases: []string{
			"zh-tw", "runninghub.intl", "runninghubintl", "runninghub_intl",
			"runninghub.ai", "runninghub-ai", "www.runninghub.ai",
			"国际", "国际站",
			// LiblibAI was routed through the intl channel pool before it had its
			// own site entry; it speaks the same RH V2 protocol, so the historical
			// value keeps resolving to intl rather than dropping to "".
			"liblib", "liblibai",
		},
	},
}

// defaultSiteKey is the site a stored identifier is attributed to when the app
// carries no site of its own: the single "Upstream ID" an app had before
// per-site identifiers existed is the international site's (see
// appSiteUpstreams).
const defaultSiteKey = "intl"

// SiteSpecView is the read shape of a SiteSpec, sent to the admin UI so the app
// form can render one identifier block per site without hard-coding the list.
type SiteSpecView struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	ChannelType int    `json:"channelType"`
}

// SiteSpecList returns every registered site in display order.
func SiteSpecList() []SiteSpecView {
	out := make([]SiteSpecView, 0, len(siteRegistry))
	for _, spec := range siteRegistry {
		out = append(out, SiteSpecView{
			Key:         spec.Key,
			Label:       spec.Label,
			ChannelType: spec.ChannelType,
		})
	}
	return out
}

// lookupSite resolves a site token, label, host name or historical alias to its
// registry entry. The second result is false for anything unrecognised.
func lookupSite(token string) (SiteSpec, bool) {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return SiteSpec{}, false
	}
	for _, spec := range siteRegistry {
		if token == spec.Key || strings.EqualFold(token, spec.Label) {
			return spec, true
		}
		for _, alias := range spec.aliases {
			if token == alias {
				return spec, true
			}
		}
	}
	return SiteSpec{}, false
}

// normalizeSite canonicalises the app's site selector to a stable registry key.
// It accepts the user-facing labels used on the /channels page (RunningHub,
// RunningHub Intl) as well as the internal tokens (cn/intl) and maps them to the
// key this plugin stores. Anything unrecognised collapses to "".
func normalizeSite(s string) string {
	if spec, ok := lookupSite(s); ok {
		return spec.Key
	}
	return ""
}

// siteToChannelType maps an app's site value to the channel type used on the
// /channels page. The 0 value means "no site-scoped routing": the legacy
// model->channel selection stays untouched.
func siteToChannelType(site string) int {
	if spec, ok := lookupSite(site); ok {
		return spec.ChannelType
	}
	return 0
}

// siteForChannelType maps a channel type back to its site key ("") when the
// channel is not a site this registry knows.
func siteForChannelType(channelType int) string {
	if channelType == 0 {
		return ""
	}
	for _, spec := range siteRegistry {
		if spec.ChannelType == channelType {
			return spec.Key
		}
	}
	return ""
}

// siteName returns the display name used in messages and the apps table for a
// site value.
func siteName(site string) string {
	if spec, ok := lookupSite(site); ok {
		return spec.Label
	}
	return ""
}

// siteDefaultBaseURL returns the default upstream endpoint of a site, or "" for
// an unknown one.
func siteDefaultBaseURL(site string) string {
	if spec, ok := lookupSite(site); ok {
		return spec.DefaultBaseURL
	}
	return ""
}

// baseURLForChannelType returns the upstream endpoint a channel type falls back
// to when the channel carries no base URL of its own. A type belonging to no
// registered site keeps the historical domestic default, so a channel that is
// merely misconfigured still has an endpoint.
func baseURLForChannelType(channelType int) string {
	if base := siteDefaultBaseURL(siteForChannelType(channelType)); base != "" {
		return base
	}
	return DefaultBaseURL
}

// siteForBaseURL maps an upstream base URL (a channel's base URL, or the URL of
// an imported curl request) back to its site key, so an imported request fills
// in the identifier block of the site it actually came from. It returns "" when
// the host belongs to no registered site.
func siteForBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	host := strings.ToLower(raw)
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		host = strings.ToLower(parsed.Host)
	}
	for _, spec := range siteRegistry {
		if defaultHost := baseURLHost(spec.DefaultBaseURL); defaultHost != "" && host == defaultHost {
			return spec.Key
		}
	}
	// A channel may point at a regional variant of the same deployment
	// (runninghub.cn vs www.runninghub.cn); match on the site's registrable
	// host as a fallback.
	for _, spec := range siteRegistry {
		if defaultHost := baseURLHost(spec.DefaultBaseURL); defaultHost != "" &&
			strings.HasSuffix(host, "."+defaultHost) {
			return spec.Key
		}
	}
	return ""
}

// baseURLHost extracts the lower-cased host of a base URL, or "" when it cannot
// be parsed.
func baseURLHost(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Host)
}
