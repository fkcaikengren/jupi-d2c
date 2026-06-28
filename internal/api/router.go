// Package api 是单一 HTTP 服务：对外上传/下载 API、本地配置面板、内嵌前端，
// 全部挂在一个端口、一个 gin 引擎上。本包只负责装配引擎与路由（router），
// 具体处理逻辑在 handlers，领域逻辑在 services。
//
//	GET  /health              健康检查（公开）
//	POST /api/upload          上传（Bearer token）
//	GET  /api/config          读配置（Bearer token）
//	PUT  /api/config          改配置（Bearer token）
//	GET  /uploads/*relpath    上传目录映射为静态资源（公开，URL 即凭据）
//	/*                        内嵌前端 SPA（webui，NoRoute 兜底）
//
// CORS 全局放行所有来源（见 middleware.CORS），供跨域客户端（如插件）直接调用 /api/upload；
// 预检 OPTIONS 由中间件直接 204，先于路由匹配与 BearerAuth。
package api

import (
	"fmt"
	"net/http"

	"d2c-manager/internal/api/handlers"
	"d2c-manager/internal/api/middleware"
	"d2c-manager/internal/api/webui"
	"d2c-manager/internal/config"
	"d2c-manager/internal/infra/queue"

	"github.com/gin-gonic/gin"
)

// NewEngine 返回装好公共中间件的 gin 引擎：Logger、统一 500 JSON 的 recovery。
// 不注册任何路由——路由由 NewRouter 接。
func NewEngine() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	// 全局放行跨域：预检 OPTIONS 在此直接 204，先于路由与 BearerAuth。
	r.Use(middleware.CORS())
	// 自定义 recovery，保证 500 的 JSON 形状稳定。
	r.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "internal error",
			"message": fmt.Sprintf("%v", err),
		})
	}))
	return r
}

// NewRouter 构建整个服务的引擎：上传/下载 API、配置 API、上传目录静态托管、内嵌前端。
// cfg 是启动期快照；configPath 指向 config.yml，配置 PUT 写回到此。
func NewRouter(cfg config.AppConfig, pool *queue.Pool, configPath string) *gin.Engine {
	r := NewEngine()
	h := handlers.New(cfg, pool, configPath)

	r.GET("/health", h.Health)
	r.POST("/api/upload", middleware.BearerAuth(cfg.Token), h.Upload)
	r.GET("/api/config", middleware.BearerAuth(cfg.Token), h.GetConfig)
	r.PUT("/api/config", middleware.BearerAuth(cfg.Token), h.PutConfig)
	r.GET("/uploads/*relpath", h.ServeUpload)

	// 前端托管（静态资源 + SPA 回退）由 webui 包接管 NoRoute；
	// 未知 /api/* 仍返回 404 JSON，其余非 API 路径回退 index.html。
	webui.Register(r)
	return r
}
