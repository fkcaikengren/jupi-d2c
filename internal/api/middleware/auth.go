// Package middleware 集中放置 HTTP 中间件：鉴权、CORS 等。
package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BearerAuth 接受两种写法，匹配任一即放行：
//
//	Authorization: Bearer <token>   （去掉前缀后比较）
//	Authorization: <token>          （直接比较）
//	PRIVATE-TOKEN: <token>          （GitLab 风格）
func BearerAuth(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		privateToken := c.GetHeader("PRIVATE-TOKEN")

		var candidates []string
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				candidates = append(candidates, strings.TrimSpace(authHeader[len("Bearer "):]))
			} else {
				candidates = append(candidates, strings.TrimSpace(authHeader))
			}
		}
		if privateToken != "" {
			candidates = append(candidates, strings.TrimSpace(privateToken))
		}

		if len(candidates) == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing auth header"})
			return
		}
		for _, t := range candidates {
			if safeEqual(t, expectedToken) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid token"})
	}
}

// safeEqual 是恒定时间比较；长度不同时 subtle 直接返回 0。
func safeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}