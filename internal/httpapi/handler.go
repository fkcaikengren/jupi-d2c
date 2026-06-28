package httpapi

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"d2c-manager/internal/config"
	"d2c-manager/internal/queue"
	"d2c-manager/internal/storage"

	"github.com/gin-gonic/gin"
)

// Handler 持有处理请求所需的配置与 worker 池。
type Handler struct {
	cfg  config.AppConfig
	pool *queue.Pool
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Health 健康检查。time 用带毫秒的 ISO8601，对齐 Node 的 toISOString()。
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"time":        time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		"maxFileSize": h.cfg.MaxFileSize,
	})
}

// ServeUpload 公开返回已上传文件；URL 本身就是凭据，不鉴权。
func (h *Handler) ServeUpload(c *gin.Context) {
	name := c.Param("filename")
	// 防目录穿越：路由参数不含 '/'，再排除 '\\' 与 '.'/'..'。
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\") {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
		return
	}
	full := filepath.Join(h.cfg.UploadDir, name)
	if info, err := os.Stat(full); err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
		return
	}
	c.File(full)
}

// Upload 接受 multipart(file 字段) 或原始二进制，落盘后返回 { data: SavedFile }。
func (h *Handler) Upload(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")

	log.Printf("[upload] ⬆ 收到上传请求 content-type=%q origin=%q ua=%q",
		contentType, dash(c.GetHeader("Origin")), dash(c.GetHeader("User-Agent")))

	var (
		data            []byte
		originalName    string
		fileContentType string
	)

	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		fh, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "multipart 字段 file 缺失或不是文件"})
			return
		}
		f, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
			return
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
			return
		}
		data = b
		originalName = fh.Filename
		fileContentType = fh.Header.Get("Content-Type")
	} else {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
			return
		}
		data = b
		fileContentType = contentType
	}

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

	job := &queue.PersistJob{
		Options: storage.SaveOptions{
			Bytes:         data,
			OriginalName:  originalName,
			ContentType:   fileContentType,
			UploadDir:     h.cfg.UploadDir,
			PublicBaseURL: h.cfg.PublicBaseURL,
		},
		Result: make(chan queue.PersistResult, 1),
	}

	if err := h.pool.Submit(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server unavailable", "message": err.Error()})
		return
	}

	select {
	case res := <-job.Result:
		if res.Err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": res.Err.Error()})
			return
		}
		log.Printf("[upload] ✓ 上传成功 %d bytes -> %s (原名 %q)", res.Saved.Size, res.Saved.URL, dash(originalName))
		c.JSON(http.StatusOK, gin.H{"data": res.Saved})
	case <-c.Request.Context().Done():
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server unavailable", "message": c.Request.Context().Err().Error()})
	}
}
