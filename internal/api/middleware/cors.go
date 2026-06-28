package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// CORS 复刻 Node(Hono)版 cors() 的契约：放行所有来源，
// 方法 GET/POST/OPTIONS，允许头 Content-Type/Authorization/PRIVATE-TOKEN，
// 暴露 Content-Length，max-age 86400，预检直接返回 204。
//
// 注意：此处刻意不使用 gin-contrib/cors。该库在 Origin 与请求 Host 同源
// （如 Origin: http://example.com 且 Host: example.com，httptest 默认即如此）
// 时会把请求当作非 CORS 请求并提前 return，不写 Access-Control-Allow-Origin，
// 破坏"allows all origins 总是返回 *"的契约。手写中间件无条件写 *，与原版一致。
const corsMaxAge = 24 * time.Hour

func CORS() gin.HandlerFunc {
	maxAge := strconv.Itoa(int(corsMaxAge / time.Second))
	return func(c *gin.Context) {
		h := c.Writer.Header()
		// 与 Hono cors() 一致：实际响应只带 Allow-Origin / Expose-Headers，
		// Allow-Methods / Allow-Headers / Max-Age 仅在 OPTIONS 预检时下发。
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Expose-Headers", "Content-Length")

		if c.Request.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type,Authorization,PRIVATE-TOKEN")
			h.Set("Access-Control-Max-Age", maxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}