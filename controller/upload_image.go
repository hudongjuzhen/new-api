package controller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// MaxImageUploadBytes is the maximum accepted size of a single uploaded image (10MB).
const MaxImageUploadBytes = 10 << 20

// uploadImageDir is the root directory for uploaded images on local disk.
const uploadImageDir = "uploads/images"

// allowedImageMimeTypes maps detected content types to canonical file extensions.
var allowedImageMimeTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/bmp":  ".bmp",
}

// monthlyImageDir returns the per-month storage sub-directory (YYYYMM) for uploaded images.
func monthlyImageDir(now time.Time) string {
	return filepath.Join(uploadImageDir, now.Format("200601"))
}

func randomImageFilename(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + ext, nil
}

func detectImageExtension(file multipart.File) (string, error) {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	contentType := http.DetectContentType(head[:n])
	ext, ok := allowedImageMimeTypes[contentType]
	if !ok {
		return "", fmt.Errorf("不支持的图片类型: %s", contentType)
	}
	return ext, nil
}

// UploadImage handles POST /api/upload/image for playground image attachments.
// Files are stored under uploads/images/YYYYMM/ on local disk.
func UploadImage(c *gin.Context) {
	// Reserve some room for multipart boundary overhead on top of the image size cap.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxImageUploadBytes+(64<<10))

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "上传失败: 请选择不超过 10MB 的图片文件")
		return
	}
	defer file.Close()

	ext, err := detectImageExtension(file)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	now := time.Now()
	dir := monthlyImageDir(now)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		common.ApiError(c, err)
		return
	}

	filename, err := randomImageFilename(ext)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	dstPath := filepath.Join(dir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(dstPath)
		common.ApiError(c, err)
		return
	}

	urlPath := "/" + filepath.ToSlash(dstPath)
	common.ApiSuccess(c, gin.H{
		"url":       urlPath,
		"filename":  filename,
		"size":      fileSizeOrZero(dstPath),
		"mime_type": mimeFromExt(ext),
	})
}

func fileSizeOrZero(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func mimeFromExt(ext string) string {
	for mime, mappedExt := range allowedImageMimeTypes {
		if strings.EqualFold(mappedExt, ext) {
			return mime
		}
	}
	return ""
}
