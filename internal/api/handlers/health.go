package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Health 健康检查。time 用带毫秒的 ISO8601，对齐 Node 的 toISOString()。
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"time":        time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		"maxFileSize": h.cfg.MaxFileSize,
	})
}
