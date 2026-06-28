package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SavedFile 是写盘后返回给客户端的元信息。JSON tag 必须与 Node 版一致。
type SavedFile struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}

// SaveOptions 是一次写盘的全部入参。
type SaveOptions struct {
	Bytes         []byte
	OriginalName  string
	ContentType   string
	UploadDir     string
	PublicBaseURL string
}

var extByContentType = map[string]string{
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/jpg":     ".jpg",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/svg+xml": ".svg",
	"image/x-icon":  ".ico",
	"image/bmp":     ".bmp",
	"image/avif":    ".avif",
}

func pickExtension(originalName, contentType string) string {
	if originalName != "" {
		e := strings.ToLower(filepath.Ext(originalName))
		if e != "" && len(e) <= 6 {
			return e
		}
	}
	if contentType != "" {
		if e, ok := extByContentType[strings.ToLower(contentType)]; ok {
			return e
		}
	}
	return ".bin"
}

// SaveBytes 把字节写入 UploadDir，返回元信息。这是唯一直接写本地磁盘的函数，
// 之后接入 S3 / OSS / R2 只需替换它（保持签名不变）。
func SaveBytes(opts SaveOptions) (SavedFile, error) {
	dir, err := filepath.Abs(opts.UploadDir)
	if err != nil {
		return SavedFile{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SavedFile{}, err
	}

	ext := pickExtension(opts.OriginalName, opts.ContentType)

	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return SavedFile{}, err
	}
	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), hex.EncodeToString(buf), ext)

	if err := os.WriteFile(filepath.Join(dir, filename), opts.Bytes, 0o644); err != nil {
		return SavedFile{}, err
	}

	contentType := opts.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return SavedFile{
		Filename:    filename,
		URL:         fmt.Sprintf("%s/uploads/%s", opts.PublicBaseURL, filename),
		Size:        int64(len(opts.Bytes)),
		ContentType: contentType,
	}, nil
}
