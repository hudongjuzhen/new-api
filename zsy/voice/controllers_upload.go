package voice

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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

// MaxVoiceUploadBytes caps one sample-audio upload (20MB). It is generous for a
// preview clip while keeping a single request from filling the disk.
const MaxVoiceUploadBytes = 20 << 20

// voiceUploadDir is the storage root, served back by the host's static handler
// at /uploads/voices/... (router.SetRouter mounts uploads/).
const voiceUploadDir = "uploads/voices"

// allowedVoiceAudioExts is the accepted extension allow-list mapped to the
// content type reported to the admin UI. An allow-list (not "whatever the
// browser called it") also keeps an uploaded HTML/SVG payload from being stored
// under an audio name and served back as a same-origin document.
var allowedVoiceAudioExts = map[string]string{
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".opus": "audio/ogg",
	".flac": "audio/flac",
	".webm": "audio/webm",
}

// sniffedAudioExts maps a detected content type to an extension, used only when
// the uploaded filename carries no usable one (a recorder posting a Blob). The
// audio container formats a browser can play are all covered.
var sniffedAudioExts = map[string]string{
	"audio/mpeg":      ".mp3",
	"audio/wave":      ".wav",
	"audio/wav":       ".wav",
	"audio/x-wav":     ".wav",
	"audio/ogg":       ".ogg",
	"application/ogg": ".ogg",
	"audio/flac":      ".flac",
	"audio/aac":       ".aac",
	"audio/mp4":       ".m4a",
	"video/mp4":       ".m4a",
	"audio/webm":      ".webm",
	"video/webm":      ".webm",
}

// uploadVoiceAudio (POST /dashboard/zsy/voice/upload) — stores one sample audio
// file under uploads/voices/YYYYMM/ and returns the URL to put in a voice row.
//
// The endpoint only writes the file; binding it to a voice is the separate
// create/update call, so an admin can upload first and fill the form after.
func uploadVoiceAudio(c *gin.Context) {
	// Reserve room for the multipart boundary on top of the file cap so an
	// oversized body is rejected by the reader, not by filling memory. The
	// Content-Length pre-check below refuses an obviously oversized upload
	// before a single byte of it is buffered.
	const maxBodyBytes = MaxVoiceUploadBytes + (64 << 10)
	tooLarge := fmt.Sprintf("上传失败: 音频文件不能超过 %dMB", MaxVoiceUploadBytes>>20)
	if c.Request.ContentLength > maxBodyBytes {
		common.ApiErrorMsg(c, tooLarge)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)

	header, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			common.ApiErrorMsg(c, tooLarge)
			return
		}
		common.ApiErrorMsg(c, "上传失败: 请通过 multipart 字段 file 选择音频文件")
		return
	}

	ext, err := detectVoiceAudioExt(header)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	now := time.Now()
	dir := filepath.Join(voiceUploadDir, now.Format("200601"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		common.ApiError(c, err)
		return
	}

	filename, err := randomVoiceFilename(ext)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	dstPath := filepath.Join(dir, filename)

	written, err := saveUploadedFile(header, dstPath)
	if err != nil {
		// A partial file must never be left behind for the static handler.
		_ = os.Remove(dstPath)
		common.ApiErrorMsg(c, err.Error())
		return
	}

	urlPath := "/" + filepath.ToSlash(dstPath)
	common.ApiSuccess(c, gin.H{
		"url":          urlPath,
		"filename":     filename,
		"originalName": header.Filename,
		"size":         written,
		"mimeType":     allowedVoiceAudioExts[ext],
	})
}

// detectVoiceAudioExt resolves the stored extension: the uploaded filename's
// extension when it is on the allow-list, otherwise the sniffed content type.
func detectVoiceAudioExt(header *multipart.FileHeader) (string, error) {
	if ext := strings.ToLower(filepath.Ext(header.Filename)); ext != "" {
		if _, ok := allowedVoiceAudioExts[ext]; ok {
			return ext, nil
		}
	}

	file, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("读取上传文件失败: %w", err)
	}
	defer file.Close()

	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("读取上传文件失败: %w", err)
	}
	if ext, ok := sniffedAudioExts[http.DetectContentType(head[:n])]; ok {
		return ext, nil
	}
	return "", errors.New("不支持的音频格式，请上传 mp3 / wav / m4a / aac / ogg / opus / flac / webm 文件")
}

// randomVoiceFilename returns a collision-free name for the stored file. The
// original name is never used on disk, so a crafted filename cannot escape the
// upload directory.
func randomVoiceFilename(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + ext, nil
}

// saveUploadedFile streams the multipart file into dstPath and returns the
// number of bytes stored, refusing to write more than the configured cap even
// if the reader above is bypassed.
func saveUploadedFile(header *multipart.FileHeader, dstPath string) (int64, error) {
	src, err := header.Open()
	if err != nil {
		return 0, fmt.Errorf("读取上传文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return 0, fmt.Errorf("保存音频文件失败: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, io.LimitReader(src, MaxVoiceUploadBytes+1))
	if err != nil {
		return written, fmt.Errorf("保存音频文件失败: %w", err)
	}
	if written > MaxVoiceUploadBytes {
		return written, fmt.Errorf("上传失败: 音频文件不能超过 %dMB", MaxVoiceUploadBytes>>20)
	}
	return written, nil
}
