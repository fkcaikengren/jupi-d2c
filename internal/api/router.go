// Package api 是单一 HTTP 服务：对外上传/下载 API、本地配置面板、内嵌前端，
// 全部挂在一个端口、一个 gin 引擎上。本包只负责装配引擎与路由（router），
// 具体处理逻辑在 handlers，领域逻辑在 services。
//
//	GET  /health              健康检查（公开）
//	POST /api/upload          上传（Bearer token）
//	GET  /api/config          读配置（Bearer token）
//	PUT  /api/config          改配置（Bearer token）
//	GET  /api/files           列出上传目录树（Bearer token）
//	POST /api/files/cleanup   清理 N 小时前的旧文件，?hours=1（Bearer token）
//	POST /api/design          保存一次生成的 AST（Bearer token）
//	POST /api/design/cleanup  清理 N 小时前的 design，?hours=1（Bearer token）
//	GET  /api/design          分页查询 design 列表，?page&pageSize&tag（公开）
//	GET  /api/design/tags     列出全部去重 tag（公开）
//	GET  /api/ast/:id         返回某个 design 的 AST JSON 原文（公开，URL 即凭据）
//	GET  /api/project-scheme        分页查询项目适配方案列表，?page&pageSize（公开）
//	GET  /api/project-scheme/detail 按 ?path 返回某项目方案的完整 markdown（公开）
//	POST /api/ai-chat         代理 OpenAI 兼容 API 聊天（Bearer token）
//	GET  /uploads/*relpath    上传目录映射为静态资源（公开，URL 即凭据）
//	ANY  /mcp                 MCP 服务（Streamable HTTP）：query_ast / get_project_scheme / save_project_scheme（公开）
//	/*                        内嵌前端 SPA（webui，NoRoute 兜底）
//
// CORS 全局放行所有来源（见 middleware.CORS），供跨域客户端（如插件）直接调用 /api/upload；
// 预检 OPTIONS 由中间件直接 204，先于路由匹配与 BearerAuth。
package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"jupi-d2c/internal/api/handlers"
	"jupi-d2c/internal/api/mcp"
	"jupi-d2c/internal/api/middleware"
	"jupi-d2c/internal/api/webui"
	"jupi-d2c/internal/config"
	"jupi-d2c/internal/infra/queue"

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
func NewRouter(cfg config.AppConfig, pool *queue.Pool, configPath string, db *sql.DB) *gin.Engine {
	r := NewEngine()
	h := handlers.New(cfg, pool, configPath, db)

	r.GET("/health", h.Health)
	r.POST("/api/upload", middleware.BearerAuth(cfg.Token), h.Upload)
	r.GET("/api/config", middleware.BearerAuth(cfg.Token), h.GetConfig)
	r.PUT("/api/config", middleware.BearerAuth(cfg.Token), h.PutConfig)
	r.GET("/api/files", middleware.BearerAuth(cfg.Token), h.ListFiles)
	r.POST("/api/files/cleanup", middleware.BearerAuth(cfg.Token), h.CleanupFiles)
	// design：保存/清理需鉴权；列表、tag 列表与单个 AST 公开（URL 即凭据）。
	r.POST("/api/design", middleware.BearerAuth(cfg.Token), h.SaveDesign)
	r.POST("/api/design/cleanup", middleware.BearerAuth(cfg.Token), h.CleanupDesigns)
	r.GET("/api/design", h.ListDesigns)
	r.GET("/api/design/tags", h.ListTags)
	r.GET("/api/ast/:id", h.GetAST)
	r.GET("/api/ast/:id/refer-dom", h.GetReferDom)
	r.POST("/api/ast/:id/refer-dom", middleware.BearerAuth(cfg.Token), h.GenerateReferDom)
	// project scheme：列表与详情公开（与 design 一致），数据由 MCP 端写入。
	r.GET("/api/project-scheme", h.ListProjectSchemes)
	r.GET("/api/project-scheme/detail", h.GetProjectScheme)
	r.POST("/api/ai-chat", middleware.BearerAuth(cfg.Token), h.AIChat)
	r.GET("/api/ai/config", middleware.BearerAuth(cfg.Token), h.GetAIConfig)
	r.PUT("/api/ai/config", middleware.BearerAuth(cfg.Token), h.PutAIConfig)
	r.GET("/uploads/*relpath", h.ServeUpload)

	// MCP（Streamable HTTP）：单端点 /mcp，POST 发 JSON-RPC 调用、GET 走 SSE、DELETE 结束会话。
	// 公开（与 /api/ast 一致），方案分析由 AI 端完成，本服务只做持久化。
	mcpHandler := mcp.NewHandler(db, func() int64 { return time.Now().UnixMilli() })
	r.Any("/mcp", gin.WrapH(mcpHandler))

	// 前端托管（静态资源 + SPA 回退）由 webui 包接管 NoRoute；
	// 未知 /api/* 仍返回 404 JSON，其余非 API 路径回退 index.html。
	webui.Register(r)
	return r
}
