package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// putAIConfigInput 是 PUT /api/ai/config 的请求体。
type putAIConfigInput struct {
	URL   string `json:"url"`
	Key   string `json:"key"`
	Model string `json:"model"`
}

// maskKey 将 API key 中间部分替换为 *，仅保留前 8 位，用于展示。
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:8] + strings.Repeat("*", len(key)-8)
}

// GetAIConfig 返回当前的 AI 配置（key 已 mask），用于前端展示。
func (h *Handlers) GetAIConfig(c *gin.Context) {
	cfg, err := h.aiConfigs.Get()
	if err != nil {
		// 未配置时返回空配置，方便前端表单初始化
		c.JSON(http.StatusOK, gin.H{"data": gin.H{
			"url":       "",
			"key":       "",
			"model":     "",
			"updatedAt": 0,
		}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"url":       cfg.URL,
		"key":       maskKey(cfg.Key),
		"model":     cfg.Model,
		"updatedAt": cfg.UpdatedAt,
	}})
}

// PutAIConfig 保存 AI 配置到后端 SQLite。
func (h *Handlers) PutAIConfig(c *gin.Context) {
	var in putAIConfigInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON", "message": err.Error()})
		return
	}

	// 校验必填字段
	var missing []string
	if strings.TrimSpace(in.URL) == "" {
		missing = append(missing, "url")
	}
	if strings.TrimSpace(in.Key) == "" {
		missing = append(missing, "key")
	}
	if strings.TrimSpace(in.Model) == "" {
		missing = append(missing, "model")
	}
	if len(missing) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必填字段", "missing": missing})
		return
	}

	nowMs := time.Now().UnixMilli()
	if err := h.aiConfigs.Update(in.URL, in.Key, in.Model, nowMs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 AI 配置失败", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"url":       in.URL,
		"key":       maskKey(in.Key),
		"model":     in.Model,
		"updatedAt": nowMs,
	}})
}
