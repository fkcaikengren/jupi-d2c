package ui

import (
	"fmt"
	"net/http"

	"d2c-manager/internal/api/middleware"
	"d2c-manager/internal/api/ui/webui"
	"d2c-manager/internal/config"

	"github.com/gin-gonic/gin"
)

// NewRouter 构建面板引擎：配置 REST API + 内嵌单页应用。
// /api/config 因为能改写配置（含 token 与端口）需要 Bearer token；
// UI 资源与 /api/health 公开访问。
func NewRouter(cfg config.AppConfig, configPath string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	// 与 public 一致的 500 JSON 形状。
	r.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": fmt.Sprintf("%v", err),
		})
	}))

	h := &Handler{cfg: cfg, configPath: configPath}

	// /api/config：读写配置——鉴权（能改 token / 端口，必须持有 STORAGE_TOKEN）。
	auth := middleware.BearerAuth(cfg.Token)
	api := r.Group("/api")
	{
		api.GET("/config", auth, h.GetConfig)
		api.PUT("/config", auth, h.PutConfig)
		// 健康检查公开（给 UI 探活用）。
		api.GET("/health", h.Health)
	}

	// SPA 必须最后挂载——它接管 NoRoute。
	webui.Mount(r)
	return r
}