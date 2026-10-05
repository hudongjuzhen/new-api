package service

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/oss_setting"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSS 访问超时（秒）：连接 15 秒，读写 120 秒
const (
	ossConnectTimeout   = 15
	ossReadWriteTimeout = 120
)

// ossBucket 按系统设置构造 OSS Bucket 句柄
func ossBucket() (*oss.Bucket, error) {
	setting := oss_setting.GetOSSSetting()
	if err := setting.Validate(); err != nil {
		return nil, err
	}
	client, err := oss.New(
		setting.SDKEndpoint(),
		setting.AccessKeyID,
		setting.AccessKeySecret,
		oss.Timeout(ossConnectTimeout, ossReadWriteTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化 OSS 客户端失败: %w", err)
	}
	bucket, err := client.Bucket(setting.BucketName())
	if err != nil {
		return nil, fmt.Errorf("获取 OSS Bucket 失败: %w", err)
	}
	return bucket, nil
}

// UploadOSSObject 将 body 写入 OSS 的 objectKey；contentType 为空时由 SDK 按对象键推断
func UploadOSSObject(objectKey string, body io.Reader, contentType string) error {
	bucket, err := ossBucket()
	if err != nil {
		return err
	}
	options := make([]oss.Option, 0, 1)
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}
	if err := bucket.PutObject(objectKey, body, options...); err != nil {
		return fmt.Errorf("上传到 OSS 失败: %w", err)
	}
	return nil
}

// TestOSSConnection 通过写入并删除一个探测对象，验证 OSS 配置与读写权限是否可用
func TestOSSConnection() error {
	setting := oss_setting.GetOSSSetting()
	objectKey := setting.ObjectKey(fmt.Sprintf(".newapi-oss-test-%d", time.Now().UnixNano()))
	if err := UploadOSSObject(objectKey, strings.NewReader("new-api oss connectivity test"), "text/plain"); err != nil {
		return err
	}
	bucket, err := ossBucket()
	if err != nil {
		return err
	}
	if err := bucket.DeleteObject(objectKey); err != nil {
		return fmt.Errorf("探测对象 %s 写入成功但删除失败: %w", objectKey, err)
	}
	return nil
}
