package handlers

import (
	"errors"
	"log"
	"net/http"

	"jupi-d2c/internal/api/services"

	"github.com/gin-gonic/gin"
)

// chatRequest 来自前端的 AI 聊天请求：messages 为 OpenAI 标准 Chat 格式（role/content）。
// url/key/model 由服务端配置提供，不再由前端传入。
type chatRequest struct {
	Messages []chatMessageDTO `json:"messages"`
}

type chatMessageDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse 返回给前端的完整响应。
type chatResponse struct {
	Message chatMessageDTO `json:"message"`
	Usage   *usageDTO      `json:"usage,omitempty"`
}

type usageDTO struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// AIChat 使用服务端保存的 AI 配置，代理 OpenAI 兼容 API，以一次非流式交互返回结果。
func (h *Handlers) AIChat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON", "message": err.Error()})
		return
	}

	// 校验必填字段
	var missing []string
	if len(req.Messages) == 0 {
		missing = append(missing, "messages")
	}
	if len(missing) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必填字段", "missing": missing})
		return
	}

	// 转换为 services.ChatMessage
	svcMessages := make([]services.ChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		svcMessages[i] = services.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	result, err := h.ai.Chat(c.Request.Context(), svcMessages)
	if err != nil {
		if errors.Is(err, services.ErrAIConfigNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AI 未配置，请先在 AI 聊天页面中配置 AI 参数"})
			return
		}
		log.Printf("[ai-chat] ❌ 调用失败: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 服务不可用", "message": err.Error()})
		return
	}

	log.Printf("[ai-chat] ✓ 成功回复 tokens=%d", result.TotalTokens)
	c.JSON(http.StatusOK, gin.H{"data": chatResponse{
		Message: chatMessageDTO{
			Role:    result.Role,
			Content: result.Content,
		},
		Usage: &usageDTO{
			PromptTokens:     result.PromptTokens,
			CompletionTokens: result.CompletionTokens,
			TotalTokens:      result.TotalTokens,
		},
	}})
}
