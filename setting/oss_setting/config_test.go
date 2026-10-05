package oss_setting

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportedOptionKeysMatchAdminSettingsContract(t *testing.T) {
	exported := config.GlobalConfig.ExportAllConfigs()

	expected := map[string]string{
		"oss_setting.enabled":           "false",
		"oss_setting.endpoint":          "",
		"oss_setting.bucket":            "",
		"oss_setting.access_key_id":     "",
		"oss_setting.access_key_secret": "",
		"oss_setting.path_prefix":       "uploads",
		"oss_setting.custom_domain":     "",
		"oss_setting.use_ssl":           "true",
	}

	for key, value := range expected {
		assert.Equal(t, value, exported[key], "unexpected default for %s", key)
	}

	for key := range exported {
		if !strings.HasPrefix(key, "oss_setting.") {
			continue
		}
		_, ok := expected[key]
		assert.True(t, ok, "option %s is not wired to the admin settings page", key)
	}
}

func TestValidateReportsMissingConfiguration(t *testing.T) {
	complete := OSSSetting{
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "mybucket",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	}
	require.NoError(t, (&complete).Validate())

	cases := []struct {
		name    string
		setting OSSSetting
		want    string
	}{
		{
			name:    "missing endpoint",
			setting: OSSSetting{Bucket: "mybucket", AccessKeyID: "ak", AccessKeySecret: "sk"},
			want:    "请填写 OSS Endpoint",
		},
		{
			name:    "missing bucket",
			setting: OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", AccessKeyID: "ak", AccessKeySecret: "sk"},
			want:    "请填写 OSS Bucket",
		},
		{
			name:    "missing access key id",
			setting: OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", AccessKeySecret: "sk"},
			want:    "请填写 OSS AccessKey ID",
		},
		{
			name:    "missing access key secret",
			setting: OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", AccessKeyID: "ak"},
			want:    "请填写 OSS AccessKey Secret",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := (&tc.setting).Validate()
			require.Error(t, err)
			assert.Equal(t, tc.want, err.Error())
		})
	}
}

func TestHostNormalizesConfiguredEndpoint(t *testing.T) {
	cases := []struct {
		name    string
		setting OSSSetting
		want    string
	}{
		{
			name:    "region endpoint",
			setting: OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket"},
			want:    "oss-cn-hangzhou.aliyuncs.com",
		},
		{
			name:    "endpoint with scheme and trailing slash",
			setting: OSSSetting{Endpoint: "https://oss-cn-hangzhou.aliyuncs.com/", Bucket: "mybucket"},
			want:    "oss-cn-hangzhou.aliyuncs.com",
		},
		{
			name:    "virtual hosted endpoint keeps no bucket prefix",
			setting: OSSSetting{Endpoint: "https://mybucket.oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket"},
			want:    "oss-cn-hangzhou.aliyuncs.com",
		},
		{
			name:    "surrounding whitespace is trimmed",
			setting: OSSSetting{Endpoint: "  oss-cn-beijing.aliyuncs.com  ", Bucket: " mybucket "},
			want:    "oss-cn-beijing.aliyuncs.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.setting.Host())
		})
	}
}

func TestSDKEndpointCarriesConfiguredScheme(t *testing.T) {
	secure := OSSSetting{Endpoint: "mybucket.oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", UseSSL: true}
	assert.Equal(t, "https://oss-cn-hangzhou.aliyuncs.com", secure.SDKEndpoint())

	insecure := OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", UseSSL: false}
	assert.Equal(t, "http://oss-cn-hangzhou.aliyuncs.com", insecure.SDKEndpoint())
}

func TestObjectURLPrefersCustomDomain(t *testing.T) {
	const key = "uploads/images/202601/a.png"

	withDefaultDomain := OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", UseSSL: true}
	assert.Equal(t, "https://mybucket.oss-cn-hangzhou.aliyuncs.com/"+key, withDefaultDomain.ObjectURL(key))

	insecure := OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", UseSSL: false}
	assert.Equal(t, "http://mybucket.oss-cn-hangzhou.aliyuncs.com/"+key, insecure.ObjectURL(key))

	customDomain := OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", UseSSL: true, CustomDomain: "cdn.example.com/"}
	assert.Equal(t, "https://cdn.example.com/"+key, customDomain.ObjectURL(key))

	customDomainWithScheme := OSSSetting{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "mybucket", UseSSL: true, CustomDomain: "http://cdn.example.com"}
	assert.Equal(t, "http://cdn.example.com/"+key, customDomainWithScheme.ObjectURL(key))

	assert.Equal(t, "https://mybucket.oss-cn-hangzhou.aliyuncs.com/"+key, withDefaultDomain.ObjectURL("/"+key))
}

func TestImageObjectKeyFollowsPrefixAndMonthlyLayout(t *testing.T) {
	now := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)

	assert.Equal(t, "uploads/images/202601/a.png", (&OSSSetting{PathPrefix: "uploads"}).ImageObjectKey(now, "a.png"))
	assert.Equal(t, "a/b/images/202601/a.png", (&OSSSetting{PathPrefix: "/a/b/"}).ImageObjectKey(now, "a.png"))
	assert.Equal(t, "images/202601/a.png", (&OSSSetting{}).ImageObjectKey(now, "a.png"))
}
