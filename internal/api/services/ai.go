package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/sashabaranov/go-openai"
)

// ------------------------------------------------------------
// AIService — AI 领域的共享服务
// ------------------------------------------------------------

// ReferDomResult 是 GenerateReferDom 的返回结果，handler 据此构造 HTTP 响应。
type ReferDomResult struct {
	ReferDom string      `json:"referDom"`
	Status   string      `json:"status"`
	Warnings []HTMLError `json:"warnings,omitempty"`
}

// AIService 封装与 LLM（OpenAI 兼容 API）交互的业务逻辑：
//   - 统一底层 callOpenAI 调用，避免 handlers 层重复实现
//   - refer-dom 的 AI 生成编排（prompt 构建、重试、提取、校验、持久化）
//   - 通用 AI 对话
type AIService struct {
	designs  *DesignService
	aiConfig *AIConfigService
}

// NewAIService 注入依赖并返回 AIService 实例。
func NewAIService(designs *DesignService, aiConfig *AIConfigService) *AIService {
	return &AIService{designs: designs, aiConfig: aiConfig}
}

// ------------------------------------------------------------
// 通用 OpenAI 调用
// ------------------------------------------------------------

// callOpenAI 封装的 OpenAI 非流式调用，返回 assistant 的纯文本回复。
func callOpenAI(ctx context.Context, url, key, model string, messages []openai.ChatCompletionMessage) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(url), "/")
	if baseURL == "" {
		baseURL = openai.DefaultConfig("").BaseURL
	}
	clientConfig := openai.DefaultConfig(strings.TrimSpace(key))
	clientConfig.BaseURL = baseURL

	client := openai.NewClientWithConfig(clientConfig)

	apiReq := openai.ChatCompletionRequest{
		Model:    strings.TrimSpace(model),
		Messages: messages,
		Stream:   false,
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := client.CreateChatCompletion(ctx, apiReq)
	if err != nil {
		return "", fmt.Errorf("OpenAI API 调用失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI 未返回有效回复")
	}

	return resp.Choices[0].Message.Content, nil
}

// ------------------------------------------------------------
// Refer-Dom 生成
// ------------------------------------------------------------

const maxReferDomRetries = 3

// referDomSystemPrompt 是 AI 生成参考 DOM 的系统消息。
const referDomSystemPrompt = `用户提供了一份设计稿的 AST/DSL，现不要求你还原成代码，先做好结构上的划分，对原本数据做好分组，生成 html 元素框架（纯 html元素，无样式），生成的结果用<body></body>包裹，不需要 html/head等标签。`

// GenerateReferDom 分析指定 design 的 AST 并生成参考 HTML DOM 结构。
// 内部会验证 AI 输出的 HTML 合法性，最多重试 maxReferDomRetries 次。
// 返回 ReferDomResult 供 handler 直接使用。
func (s *AIService) GenerateReferDom(ctx context.Context, id string) (*ReferDomResult, error) {
	// 检查 AI 是否已配置
	aiCfg, err := s.aiConfig.GetConfigured()
	if err != nil {
		if errors.Is(err, ErrAIConfigNotFound) {
			return nil, fmt.Errorf("%w: AI 未配置，请先在 AI 聊天页面中配置 AI 参数", ErrAIConfigNotFound)
		}
		return nil, fmt.Errorf("读取 AI 配置失败: %w", err)
	}

	// 获取 AST
	ast, err := s.designs.GetAST(id)
	if err != nil {
		return nil, err // 外部可判断 ErrDesignNotFound
	}

	lastOutput := ""
	var lastErrors []HTMLError

	for attempt := 0; attempt < maxReferDomRetries; attempt++ {
		// 构造 messages
		messages := []openai.ChatCompletionMessage{
			{Role: "system", Content: referDomSystemPrompt},
			{Role: "user", Content: ast},
		}
		if attempt > 0 {
			// 把上一次的 HTML 错误反馈给 AI，让它修正
			var sb strings.Builder
			sb.WriteString("你上一步生成的 HTML 存在以下问题，请逐一修正后重新输出纯 HTML 标签（不要 markdown 包裹）：\n")
			for _, e := range lastErrors {
				sb.WriteString(fmt.Sprintf("- line %d col %d: %s\n", e.Line, e.Col, e.Message))
			}
			messages = append(messages, openai.ChatCompletionMessage{Role: "user", Content: sb.String()})
		}

		// 调用 AI
		rawOutput, err := callOpenAI(ctx, aiCfg.URL, aiCfg.Key, aiCfg.Model, messages)
		if err != nil {
			return nil, fmt.Errorf("AI 服务调用失败: %w", err)
		}
		lastOutput = rawOutput

		log.Printf("[refer-dom] ↳ 第 %d 次 AI 返回（id=%s model=%s），长度 %d", attempt+1, id, aiCfg.Model, len(rawOutput))

		// 从 AI 回答中提取 <body>...</body> 部分
		bodyContent, err := extractBodyContent(rawOutput)
		if err != nil {
			log.Printf("[refer-dom] ⚠ 第 %d 次未能找到 <body>: %v", attempt+1, err)
			lastErrors = []HTMLError{{
				Line:    0,
				Col:     0,
				Message: err.Error(),
			}}
			continue
		}

		// 对 body 内容做 HTML 规则检查
		lastErrors = CheckHTML(bodyContent)
		if len(lastErrors) == 0 {
			// 合法 — 保存并返回
			if err := s.designs.UpdateReferDomWithStatus(id, bodyContent, "ok", ""); err != nil {
				return nil, fmt.Errorf("internal error: %w", err)
			}
			log.Printf("[refer-dom] ✓ 生成成功 id=%s attempt=%d", id, attempt+1)
			return &ReferDomResult{
				ReferDom: bodyContent,
				Status:   "ok",
			}, nil
		}

		log.Printf("[refer-dom] ⚠ 第 %d 次生成有 %d 个问题，准备重试", attempt+1, len(lastErrors))
	}

	// 所有重试均失败 — 保存最后一次输出并返回 warning
	log.Printf("[refer-dom] ✗ 所有重试均失败 id=%s，保存为 warning", id)

	errorsJSON, _ := json.Marshal(lastErrors)

	if err := s.designs.UpdateReferDomWithStatus(id, lastOutput, "warning", string(errorsJSON)); err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}

	return &ReferDomResult{
		ReferDom: lastOutput,
		Status:   "warning",
		Warnings: lastErrors,
	}, nil
}

// ------------------------------------------------------------
// 通用 AI 对话
// ------------------------------------------------------------

// ChatMessage 是 Chat 请求的输入消息单元。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResult 是 Chat 的返回结果。
type ChatResult struct {
	Role             string
	Content          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Chat 使用服务端保存的 AI 配置，代理 OpenAI 兼容 API，以一次非流式交互返回结果。
func (s *AIService) Chat(ctx context.Context, messages []ChatMessage) (*ChatResult, error) {
	aiCfg, err := s.aiConfig.GetConfigured()
	if err != nil {
		if errors.Is(err, ErrAIConfigNotFound) {
			return nil, fmt.Errorf("%w: AI 未配置，请先在 AI 聊天页面中配置 AI 参数", ErrAIConfigNotFound)
		}
		return nil, fmt.Errorf("读取 AI 配置失败: %w", err)
	}

	// 组装 OpenAI 请求体
	apiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		apiMessages[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(aiCfg.URL), "/")
	if baseURL == "" {
		baseURL = openai.DefaultConfig("").BaseURL
	}
	clientConfig := openai.DefaultConfig(strings.TrimSpace(aiCfg.Key))
	clientConfig.BaseURL = baseURL

	client := openai.NewClientWithConfig(clientConfig)

	apiReq := openai.ChatCompletionRequest{
		Model:    strings.TrimSpace(aiCfg.Model),
		Messages: apiMessages,
		Stream:   false,
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := client.CreateChatCompletion(ctx, apiReq)
	if err != nil {
		return nil, fmt.Errorf("AI 服务不可用: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI 未返回有效回复")
	}

	choice := resp.Choices[0]
	return &ChatResult{
		Role:             choice.Message.Role,
		Content:          choice.Message.Content,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}, nil
}

// ------------------------------------------------------------
// HTML Body 提取工具（纯函数）
// ------------------------------------------------------------

// extractBodyContent 从 AI 原始输出中提取 <body>...</body> 及其内部内容。
// 返回的字符串包含 <body> 和 </body> 标签本身。
//
// 使用 html.Tokenizer 按"标签配对"语义扫描，天然避开：
//   - 注释、属性值、文本里出现的字面量 <body>（被 tokenizer 静默归入注释/文本 token）
//   - 未配对的 "<body>" 提及（不会出现在 completed 列表里）
//
// 匹配规则：
//   - 用栈记录所有 <body> 起始位置，每次 <body> 入栈、</body> 出栈
//   - 每次出栈记录一对 (start, end)，记入 completed 列表
//   - 若 AI 输出里第一个 <body> 是"未配对的提及"(没对应的 </body>)，它不会进入 completed
//   - 若有多个配对完成(嵌套或并列)，取**起始位置最早**的那对作为答案
//   - 全部未配对则返回错误，供重试时反馈 AI
func extractBodyContent(raw string) (string, error) {
	z := html.NewTokenizer(strings.NewReader(raw))

	type bodyRange struct{ start, end int }
	var (
		stack      []int       // <body> 起始位置栈
		completed  []bodyRange // 所有"开闭配对"完成的 body 区间
		byteOffset int         // 当前 token 在 raw 中的字节偏移
	)

	for {
		tt := z.Next()

		// z.Raw() 返回上一次 Next() 消费的原始字节,
		// 用它累加字节偏移以定位 <body> 和 </body> 在原文中的位置。
		tokenBytes := z.Raw()
		tokenStart := byteOffset
		byteOffset += len(tokenBytes)
		tokenEnd := byteOffset

		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				if len(completed) == 0 {
					if len(stack) > 0 {
						return "", fmt.Errorf("<body> 标签未闭合")
					}
					return "", fmt.Errorf("AI 输出中未找到 <body>...</body>")
				}
				// 取起始位置最早的那对(最外层/最优先)
				outer := completed[0]
				for _, b := range completed[1:] {
					if b.start < outer.start {
						outer = b
					}
				}
				result := raw[outer.start:outer.end]
				if isEmptyBodyContent(result) {
					return "", fmt.Errorf("<body> 内容为空")
				}
				return result, nil
			}
			return "", fmt.Errorf("HTML 解析错误: %v", err)

		case html.StartTagToken:
			name, _ := z.TagName()
			tag := string(name)
			// 消耗属性 token(否则下一次 Next() 会拿到属性)
			for {
				_, _, more := z.TagAttr()
				if !more {
					break
				}
			}
			if tag == "body" {
				stack = append(stack, tokenStart)
			}

		case html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "body" {
				if len(stack) > 0 {
					start := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					completed = append(completed, bodyRange{start: start, end: tokenEnd})
				}
			}
		}
	}
}

// isEmptyBodyContent 判断 <body>...</body> 内部内容是否为空(纯空白也算空)。
// 通过定位开标签后的第一个 > 来取内部子串,避免被属性里的 > 干扰。
func isEmptyBodyContent(s string) bool {
	gt := strings.Index(s, ">")
	if gt < 0 {
		return true
	}
	inner := s[gt+1 : len(s)-len("</body>")]
	return strings.TrimSpace(inner) == ""
}
