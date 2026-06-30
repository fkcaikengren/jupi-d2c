package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

// chatRequest 来自前端的 AI 聊天请求：使用用户提供的端点、密钥与模型名，
// messages 为 OpenAI 标准 Chat 格式（role/content）。
type chatRequest struct {
	URL      string            `json:"url"`
	Key      string            `json:"key"`
	Model    string            `json:"model"`
	Messages []chatMessageDTO  `json:"messages"`
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

// AIChat 代理用户提供的 OpenAI 兼容 API，以一次非流式交互返回结果。
// 密钥仅透传至上游，本服务不落盘。
func (h *Handlers) AIChat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON", "message": err.Error()})
		return
	}

	// 校验必填字段
	var missing []string
	if strings.TrimSpace(req.Key) == "" {
		missing = append(missing, "key")
	}
	if strings.TrimSpace(req.Model) == "" {
		missing = append(missing, "model")
	}
	if len(req.Messages) == 0 {
		missing = append(missing, "messages")
	}
	if len(missing) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必填字段", "missing": missing})
		return
	}

	// 组装 OpenAI 请求体
	apiMessages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		apiMessages[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	apiReq := openai.ChatCompletionRequest{
		Model:    strings.TrimSpace(req.Model),
		Messages: apiMessages,
		Stream:   false,
	}

	// 创建 client：允许用户自定义 baseURL，缺省用 OpenAI 官方端点
	baseURL := strings.TrimRight(strings.TrimSpace(req.URL), "/")
	if baseURL == "" {
		baseURL = openai.DefaultConfig("").BaseURL // "https://api.openai.com/v1"
	}
	clientConfig := openai.DefaultConfig(strings.TrimSpace(req.Key))
	clientConfig.BaseURL = baseURL

	client := openai.NewClientWithConfig(clientConfig)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	resp, err := client.CreateChatCompletion(ctx, apiReq)
	if err != nil {
		log.Printf("[ai-chat] ❌ API 调用失败: %v", err)

		// 尝试提取上游错误细节
		var apiErr *openai.APIError
		if errors.As(err, &apiErr) {
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "上游 API 返回错误",
				"message": apiErr.Message,
				"type":    apiErr.Type,
				"code":    apiErr.Code,
			})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI 服务不可用", "message": err.Error()})
		return
	}

	if len(resp.Choices) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI 未返回有效回复"})
		return
	}

	choice := resp.Choices[0]
	result := chatResponse{
		Message: chatMessageDTO{
			Role:    choice.Message.Role,
			Content: choice.Message.Content,
		},
		Usage: &usageDTO{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	log.Printf("[ai-chat] ✓ 成功回复 model=%q tokens=%d", req.Model, resp.Usage.TotalTokens)
	c.JSON(http.StatusOK, gin.H{"data": result})
}
