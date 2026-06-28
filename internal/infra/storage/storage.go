// Package storage 是可替换的持久化后端（当前实现：本地磁盘）。
// 接 S3 / OSS / R2 时只需新增同名函数，保持 SaveBytes 签名不变。
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
	Bytes        []byte
	OriginalName string
	ContentType  string
	UploadDir    string
	BaseURL      string
	// Tag 为本次生成 AST 的归档标签，非空时按其建子目录归档（见 sanitizeTag）。
	Tag string
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

// sanitizeTag 把客户端传入的 tag 清洗为安全的单层目录名：仅保留
// [A-Za-z0-9._-]，并拒绝 "." / ".." 等会逃逸出上传目录的结果。
// 返回 "" 表示不建子目录（直接平铺到 UploadDir）。
func sanitizeTag(tag string) string {
	var b strings.Builder
	for _, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" || s == "." || s == ".." || strings.Contains(s, "..") {
		return ""
	}
	return s
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
	// 有合法 tag 时按其建一层子目录归档本次生成的资源；URL 也带上该段。
	tag := sanitizeTag(opts.Tag)
	if tag != "" {
		dir = filepath.Join(dir, tag)
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

	url := fmt.Sprintf("%s/uploads/%s", opts.BaseURL, filename)
	if tag != "" {
		url = fmt.Sprintf("%s/uploads/%s/%s", opts.BaseURL, tag, filename)
	}

	return SavedFile{
		Filename:    filename,
		URL:         url,
		Size:        int64(len(opts.Bytes)),
		ContentType: contentType,
	}, nil
}
