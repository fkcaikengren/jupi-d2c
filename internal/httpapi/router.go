package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"d2c-manager/internal/auth"
	"d2c-manager/internal/config"
	"d2c-manager/internal/queue"

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
	r.Use(corsMiddleware())

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
	})

	h := &Handler{cfg: cfg, pool: pool}
	r.GET("/health", h.Health)
	r.GET("/uploads/:filename", h.ServeUpload)
	r.POST("/api/upload", auth.BearerAuth(cfg.Token), h.Upload)

	return r
}

// corsMiddleware 复刻 Node(Hono)版 cors() 的契约：放行所有来源，
// 方法 GET/POST/OPTIONS，允许头 Content-Type/Authorization/PRIVATE-TOKEN，
// 暴露 Content-Length，max-age 86400，预检直接返回 204。
//
// 注意：此处刻意不使用 gin-contrib/cors。该库在 Origin 与请求 Host 同源
// （如 Origin: http://example.com 且 Host: example.com，httptest 默认即如此）
// 时会把请求当作非 CORS 请求并提前 return，不写 Access-Control-Allow-Origin，
// 破坏“allows all origins 总是返回 *”的契约。手写中间件无条件写 *，与原版一致。
const corsMaxAge = 24 * time.Hour

func corsMiddleware() gin.HandlerFunc {
	maxAge := strconv.Itoa(int(corsMaxAge / time.Second))
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type,Authorization,PRIVATE-TOKEN")
		h.Set("Access-Control-Expose-Headers", "Content-Length")
		h.Set("Access-Control-Max-Age", maxAge)

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
