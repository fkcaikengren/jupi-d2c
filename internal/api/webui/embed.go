// Package webui 内嵌前端构建产物并把它挂到 gin 引擎上托管（静态资源 + SPA 回退）。
// 与配置 API（ui 包）解耦：ui 只管 /api/*，前端托管全在这里。
package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// webuiFS 内嵌前端构建产物（web/ 经 vite build 输出到 dist/）。
// 用 all: 前缀（而非裸 //go:embed dist）是关键：裸模式会跳过 . 开头的文件，
// 目录仅含 .gitkeep 时报 "no matching files found" 而编译失败。all: 把 .gitkeep
// 也纳入，于是未跑 pnpm build 时也能编译通过；运行时再按 index.html 是否存在
// 决定托管真实 UI 还是占位页。
//
//go:embed all:dist
var webuiFS embed.FS

// 未构建前端时返回的占位页，提示构建步骤。
const placeholderHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="UTF-8"><title>Jupi D2C · 未构建</title>
<style>body{font-family:system-ui,-apple-system,sans-serif;max-width:40rem;margin:4rem auto;padding:0 1rem;color:#334155;line-height:1.6}code{background:#e2e8f0;padding:.1rem .35rem;border-radius:.25rem}</style>
</head><body>
<h1>Jupi D2C 控制面板</h1>
<p>前端尚未构建，当前仅内嵌了占位锚点。请先构建前端再重新编译二进制：</p>
<pre><code>cd web &amp;&amp; pnpm install &amp;&amp; pnpm build &amp;&amp; cd ..
go build -o jupi-d2c ./cmd/jupi-d2c</code></pre>
<p>配置 API（<code>/api/config</code>、<code>/api/health</code>）不依赖前端，已可用。</p>
</body></html>`

// Register 把内嵌前端挂到 NoRoute 兜底：已注册的 /api/* 路由优先匹配，
// 其余路径走静态资源；找不到的非 API 路径回退 index.html（SPA 客户端路由）。
func Register(r *gin.Engine) {
	sub, err := fs.Sub(webuiFS, "dist")
	if err != nil {
		// embed.FS 的子目录恒定存在；理论不可达。
		panic(fmt.Sprintf("内嵌 webui 子目录失败: %v", err))
	}

	// 未构建前端：dist 仅含 .gitkeep，index.html 缺失，返回占位页。
	if !fileExists(sub, "index.html") {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": c.Request.URL.Path})
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(placeholderHTML))
		})
		return
	}

	fileServer := http.FileServer(http.FS(sub))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": p})
			return
		}
		// 请求的文件不存在（含 "/"）→ 回退 index.html，由前端路由接管。
		name := strings.TrimPrefix(path.Clean(p), "/")
		if name == "" || !fileExists(sub, name) {
			c.Request.URL.Path = "/" // FileServer 对 "/" 自动返回 index.html
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

// fileExists 报告 fsys 中是否存在该名字（文件或目录）。
func fileExists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
