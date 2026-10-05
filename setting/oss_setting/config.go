package oss_setting

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
)

// OSSSetting 阿里云对象存储（OSS）配置
type OSSSetting struct {
	// Enabled 是否启用 OSS 对象存储；启用后上传的文件写入 OSS，不再落本地磁盘
	Enabled bool `json:"enabled"`
	// Endpoint OSS 访问域名，可带协议头，例如 oss-cn-hangzhou.aliyuncs.com
	Endpoint string `json:"endpoint"`
	// Bucket 存储空间名称
	Bucket string `json:"bucket"`
	// AccessKeyID 阿里云访问密钥 ID
	AccessKeyID string `json:"access_key_id"`
	// AccessKeySecret 阿里云访问密钥 Secret
	AccessKeySecret string `json:"access_key_secret"`
	// PathPrefix 对象键前缀（相当于 OSS 中的目录），留空表示写入 Bucket 根目录
	PathPrefix string `json:"path_prefix"`
	// CustomDomain 自定义域名或 CDN 域名；留空则返回 OSS 默认域名
	CustomDomain string `json:"custom_domain"`
	// UseSSL 是否通过 HTTPS 访问 OSS
	UseSSL bool `json:"use_ssl"`
}

var ossSetting = OSSSetting{
	Enabled:         false,
	Endpoint:        "",
	Bucket:          "",
	AccessKeyID:     "",
	AccessKeySecret: "",
	PathPrefix:      "uploads",
	CustomDomain:    "",
	UseSSL:          true,
}

func init() {
	config.GlobalConfig.Register("oss_setting", &ossSetting)
}

// GetOSSSetting 获取 OSS 配置
func GetOSSSetting() *OSSSetting {
	return &ossSetting
}

// Scheme 返回访问 OSS 使用的协议
func (s *OSSSetting) Scheme() string {
	if s.UseSSL {
		return "https"
	}
	return "http"
}

// Host 返回规范化后的 OSS 访问域名：去除协议头、桶名前缀与结尾斜杠。
// 这样无论管理员填写 oss-cn-hangzhou.aliyuncs.com 还是
// https://mybucket.oss-cn-hangzhou.aliyuncs.com/ 都能正确解析。
func (s *OSSSetting) Host() string {
	host := strings.TrimSpace(s.Endpoint)
	if idx := strings.Index(host, "://"); idx >= 0 {
		host = host[idx+3:]
	}
	host = strings.TrimRight(host, "/")
	if bucket := s.BucketName(); bucket != "" {
		host = strings.TrimPrefix(host, bucket+".")
	}
	return host
}

// BucketName 返回去除空白后的存储空间名称
func (s *OSSSetting) BucketName() string {
	return strings.TrimSpace(s.Bucket)
}

// SDKEndpoint 返回 OSS SDK 需要的带协议访问域名
func (s *OSSSetting) SDKEndpoint() string {
	return s.Scheme() + "://" + s.Host()
}

// Prefix 返回对象键前缀，无首尾斜杠
func (s *OSSSetting) Prefix() string {
	return strings.Trim(strings.TrimSpace(s.PathPrefix), "/")
}

// ObjectKey 返回带前缀的对象键
func (s *OSSSetting) ObjectKey(name string) string {
	objectKey := strings.TrimLeft(name, "/")
	if prefix := s.Prefix(); prefix != "" {
		return prefix + "/" + objectKey
	}
	return objectKey
}

// ImageObjectKey 返回图片的对象键：<前缀>/images/YYYYMM/<文件名>
func (s *OSSSetting) ImageObjectKey(now time.Time, filename string) string {
	return s.ObjectKey("images/" + now.Format("200601") + "/" + filename)
}

// ObjectURL 返回对象的公网访问地址
func (s *OSSSetting) ObjectURL(objectKey string) string {
	key := strings.TrimLeft(objectKey, "/")
	if domain := strings.TrimRight(strings.TrimSpace(s.CustomDomain), "/"); domain != "" {
		if strings.Contains(domain, "://") {
			return domain + "/" + key
		}
		return s.Scheme() + "://" + domain + "/" + key
	}
	return fmt.Sprintf("%s://%s.%s/%s", s.Scheme(), s.BucketName(), s.Host(), key)
}

// Validate 校验启用 OSS 所需的必填配置
func (s *OSSSetting) Validate() error {
	switch {
	case s.Host() == "":
		return fmt.Errorf("请填写 OSS Endpoint")
	case s.BucketName() == "":
		return fmt.Errorf("请填写 OSS Bucket")
	case strings.TrimSpace(s.AccessKeyID) == "":
		return fmt.Errorf("请填写 OSS AccessKey ID")
	case strings.TrimSpace(s.AccessKeySecret) == "":
		return fmt.Errorf("请填写 OSS AccessKey Secret")
	}
	return nil
}
