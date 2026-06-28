// Package webui 通过 go:embed 把前端构建产物内嵌进二进制。
// 它只被 ui router 调用——Mount 必须放在所有 /api 路由注册之后，
// 因为它会接管 NoRoute。
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// distFS 内嵌前端构建产物。dist/ 下提交了 .gitkeep 作为锚点，使得在前端尚未构建时
// go build 依然成功（此时无 index.html，Mount 回退到下方占位页）；`pnpm build` 会把
// 真实产物写入 dist/（产物本身被 .gitignore 忽略）。
// 使用 all: 前缀以包含 Vite 产物里以 _ / . 开头的资源。
//
//go:embed all:dist
var distFS embed.FS

// fallbackHTML 在前端尚未构建（dist 下只有 .gitkeep）时返回，提示如何构建。
const fallbackHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="UTF-8"><title>D2C Manager · Panel</title></head>
<body style="font-family:system-ui,sans-serif;max-width:40rem;margin:4rem auto;padding:0 1rem">
<h1>D2C Manager</h1>
<p>Web UI 尚未构建。请在 <code>web/</code> 目录运行 <code>pnpm install &amp;&amp; pnpm build</code>，再重新 <code>go build</code>。</p>
</body></html>`

// Mount 把内嵌的单页应用挂到 gin 引擎：命中真实静态文件就返回文件，
// 其余非 /api 路径回退到 index.html（支持前端路由）。
// 必须在所有 /api 路由注册之后调用——它接管 NoRoute。
func Mount(r *gin.Engine) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // dist 一定存在（占位文件已提交）
	}
	indexHTML, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		indexHTML = []byte(fallbackHTML) // 前端尚未构建
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": p})
			return
		}
		rel := strings.TrimPrefix(p, "/")
		if rel != "" {
			if st, statErr := fs.Stat(sub, rel); statErr == nil && !st.IsDir() {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		// 未知路径（含 "/"）回退到 SPA 入口。直接写出 index.html 字节，
		// 避免 http.FileServer 对 "/index.html" 触发 301 规范化跳转。
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
}