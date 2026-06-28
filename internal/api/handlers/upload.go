package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"d2c-manager/internal/api/services"
	"d2c-manager/internal/infra/storage"

	"github.com/gin-gonic/gin"
)

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ServeUpload 公开返回已上传文件；URL 本身就是凭据，不鉴权。
// 通配路由既匹配扁平文件 /uploads/<file>，也匹配按 tag 归档的 /uploads/<tag>/<file>。
func (h *Handlers) ServeUpload(c *gin.Context) {
	rel := strings.TrimPrefix(c.Param("relpath"), "/")
	clean := filepath.Clean(rel)
	// 防目录穿越：拒绝空、当前目录、以及任何含 ".." 的路径。
	if rel == "" || clean == "." || clean == ".." ||
		strings.HasPrefix(clean, "..") || strings.Contains(clean, "..") {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
		return
	}
	base, err := filepath.Abs(h.cfg.UploadDir)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
		return
	}
	full := filepath.Join(base, clean)
	// 二次校验：清洗后的绝对路径必须仍在上传目录内。
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
		return
	}
	if info, err := os.Stat(full); err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
		return
	}
	c.File(full)
}

// uploadInput 是从请求体解析出的上传素材（与传输形式无关）。
type uploadInput struct {
	data         []byte
	originalName string
	contentType  string
	tag          string
}

// readUploadInput 把 multipart(file 字段) 或原始二进制统一解析为 uploadInput。
// 返回的 status>0 表示已确定 HTTP 错误码（含人类可读消息），调用方据此回写。
func readUploadInput(c *gin.Context) (uploadInput, int, string) {
	contentType := c.GetHeader("Content-Type")
	var in uploadInput

	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		fh, err := c.FormFile("file")
		if err != nil {
			return in, http.StatusBadRequest, "multipart 字段 file 缺失或不是文件"
		}
		f, err := fh.Open()
		if err != nil {
			return in, http.StatusInternalServerError, err.Error()
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		if err != nil {
			return in, http.StatusInternalServerError, err.Error()
		}
		in.data = b
		in.originalName = fh.Filename
		in.contentType = fh.Header.Get("Content-Type")
		in.tag = c.PostForm("tag")
	} else {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return in, http.StatusInternalServerError, err.Error()
		}
		in.data = b
		in.contentType = contentType
	}
	// 兜底：原始二进制上传无表单字段，从 query 取 tag。
	if in.tag == "" {
		in.tag = c.Query("tag")
	}
	return in, 0, ""
}

// Upload 接受 multipart(file 字段) 或原始二进制，落盘后返回 { data: SavedFile }。
func (h *Handlers) Upload(c *gin.Context) {
	log.Printf("[upload] ⬆ 收到上传请求 content-type=%q origin=%q ua=%q",
		c.GetHeader("Content-Type"), dash(c.GetHeader("Origin")), dash(c.GetHeader("User-Agent")))

	in, status, msg := readUploadInput(c)
	if status != 0 {
		// 4xx 用 error 字段，5xx 额外带 message，沿用原有形状。
		if status == http.StatusBadRequest {
			c.JSON(status, gin.H{"error": msg})
		} else {
			c.JSON(status, gin.H{"error": "internal error", "message": msg})
		}
		return
	}
	data, originalName := in.data, in.originalName

	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体为空"})
		return
	}
	if int64(len(data)) > h.cfg.MaxFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error":  fmt.Sprintf("文件超过最大限制 %d 字节", h.cfg.MaxFileSize),
			"actual": len(data),
		})
		return
	}

	saved, err := h.uploads.Save(c.Request.Context(), storage.SaveOptions{
		Bytes:         data,
		OriginalName:  originalName,
		ContentType:   in.contentType,
		UploadDir:     h.cfg.UploadDir,
		PublicBaseURL: h.cfg.PublicBaseURL,
		Tag:           in.tag,
	})
	if err != nil {
		if errors.Is(err, services.ErrUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server unavailable", "message": errors.Unwrap(err).Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		}
		return
	}

	log.Printf("[upload] ✓ 上传成功 %d bytes -> %s (原名 %q)", saved.Size, saved.URL, dash(originalName))
	c.JSON(http.StatusOK, gin.H{"data": saved})
}
