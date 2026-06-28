package public

import (
	"fmt"
	"net/http"

	"d2c-manager/internal/api/middleware"
	"d2c-manager/internal/config"
	"d2c-manager/internal/infra/queue"

	"github.com/gin-gonic/gin"
)

// NewRouter 构建并接线 gin 引擎：中间件、CORS、路由、兜底。
func NewRouter(cfg config.AppConfig, pool *queue.Pool) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	// 自定义 recovery，保证 500 的 JSON 形状与 Node 版一致。
	r.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": fmt.Sprintf("%v", err),
		})
	}))
	r.Use(middleware.CORS())

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
	})

	h := &Handler{cfg: cfg, pool: pool}
	r.GET("/health", h.Health)
	r.GET("/uploads/:filename", h.ServeUpload)
	r.POST("/api/upload", middleware.BearerAuth(cfg.Token), h.Upload)

	return r
}