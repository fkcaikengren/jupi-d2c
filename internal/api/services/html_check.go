// Package services 放置与传输层（gin/HTTP）无关的领域逻辑。
//
// html_check.go 基于 golang.org/x/net/html 的 Tokenizer 实现自定义的 HTML 规则检查器，
// 用于校验 AI 生成的参考 DOM 结构是否合法，并给出详细定位错误，供重试 prompt 使用。
package services

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// HTMLError 记录一条 HTML 校验错误的具体位置与描述。
type HTMLError struct {
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Message string `json:"message"`
}

// forbiddenTags 是 AI 输出 HTML 的黑名单。落在名单内的 tag 将触发错误。
// 目前禁止：script（内联脚本）、link（外部资源链接）、style（内联样式），
// 这些都是"页面级"的标签，AI 在做参考 DOM 时不应使用。
var forbiddenTags = map[string]bool{
	"script": true,
	"link":   true,
	"style":  true,
}

// voidElements 是 HTML5 标准中的 void 元素（自闭合标签，<img>、<br/> 等）。
// 这些标签以"出入栈都跳过"的方式处理：开始标签不入栈，结束标签不参与匹配。
// 这样无论是 <img>、<img/> 还是 <img></img> 都被视为合法写法。
// 注意：黑名单检查发生在 void 判断之前，所以 <link> 仍然会被禁止标签规则拦截。
var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}

// CheckHTML 用 html.Tokenizer 遍历整个输入，检查"开始/结束标签是否配对"。
//
// 核心规则：
//   - 黑名单标签（script / link / style）出现即报错
//   - 非 void 元素的开始标签压栈
//   - 结束标签与栈顶匹配：匹配成功弹出，匹配失败报错
//   - void 元素（<img>、<br/> 等）出入栈都跳过，不参与配对检查
//   - HTML 注释、DOCTYPE 静默跳过（不报错）
//   - 遍历结束后，栈中残留即视为"未闭合标签"
//
// 返回错误列表，每条错误包含行号和列号。全部通过则返回 nil。
func CheckHTML(s string) []HTMLError {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	var errors []HTMLError
	z := html.NewTokenizer(strings.NewReader(s))
	tagStack := make([]string, 0) // 开始标签名栈
	lineStack := make([]int, 0)   // 对应开始标签的行号
	colStack := make([]int, 0)    // 对应开始标签的列号
	line, col := 1, 1             // tokenizer 当前行/列
	prevLine, prevCol := 1, 1     // 上一个 token 的起始位置

	for {
		tt := z.Next()

		// 更新行列位置：Tokenizer 每次 Next() 消费的字节用 Raw() 获取
		raw := z.Raw()
		if len(raw) > 0 {
			prevLine, prevCol = line, col
			for _, b := range raw {
				if b == '\n' {
					line++
					col = 1
				} else {
					col++
				}
			}
		}

		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				// 正常结束 — 检查栈中剩余未闭合标签
				for i := len(tagStack) - 1; i >= 0; i-- {
					errors = append(errors, HTMLError{
						Line:    lineStack[i],
						Col:     colStack[i],
						Message: fmt.Sprintf("标签 <%s> 未闭合", tagStack[i]),
					})
				}
				if len(errors) == 0 {
					return nil
				}
				return errors
			}
			// tokenizer 遇到畸形结构（无法解析的标签）
			errors = append(errors, HTMLError{
				Line:    prevLine,
				Col:     prevCol,
				Message: fmt.Sprintf("HTML 解析错误: %v", err),
			})
			return errors

		case html.StartTagToken:
			rawName, _ := z.TagName()
			tagName := string(rawName)
			if tagName == "" {
				break
			}
			// 消耗属性 token
			for {
				_, _, more := z.TagAttr()
				if !more {
					break
				}
			}
			// 黑名单检查（在 void 判断之前，确保 <link> 也会被拦截）
			if forbiddenTags[tagName] {
				errors = append(errors, HTMLError{
					Line:    prevLine,
					Col:     prevCol,
					Message: fmt.Sprintf("不允许的标签 <%s>", tagName),
				})
				break
			}
			// void 元素出入栈都跳过
			if voidElements[tagName] {
				break
			}
			// 非 void 元素入栈
			tagStack = append(tagStack, tagName)
			lineStack = append(lineStack, prevLine)
			colStack = append(colStack, prevCol)

		case html.EndTagToken:
			rawName, _ := z.TagName()
			tagName := string(rawName)
			// 黑名单检查
			if forbiddenTags[tagName] {
				errors = append(errors, HTMLError{
					Line:    prevLine,
					Col:     prevCol,
					Message: fmt.Sprintf("不允许的标签 </%s>", tagName),
				})
				break
			}
			// void 元素结束标签静默跳过
			if voidElements[tagName] {
				break
			}
			// 从栈中匹配
			if len(tagStack) == 0 {
				errors = append(errors, HTMLError{
					Line:    prevLine,
					Col:     prevCol,
					Message: fmt.Sprintf("多余的结束标签 </%s>，没有对应的开始标签", tagName),
				})
				break
			}
			top := tagStack[len(tagStack)-1]
			if top != tagName {
				// 标签不匹配：记录错误，弹出栈顶（容错继续）
				errors = append(errors, HTMLError{
					Line: prevLine, Col: prevCol,
					Message: fmt.Sprintf("标签不匹配: 期望 </%s> 但得到 </%s>（开始标签位于 %d:%d）",
						top, tagName, lineStack[len(lineStack)-1], colStack[len(colStack)-1]),
				})
			}
			tagStack = tagStack[:len(tagStack)-1]
			lineStack = lineStack[:len(lineStack)-1]
			colStack = colStack[:len(colStack)-1]

		case html.SelfClosingTagToken:
			rawName, _ := z.TagName()
			tagName := string(rawName)
			// 消耗属性 token
			for {
				_, _, more := z.TagAttr()
				if !more {
					break
				}
			}
			// 黑名单检查
			if forbiddenTags[tagName] {
				errors = append(errors, HTMLError{
					Line:    prevLine,
					Col:     prevCol,
					Message: fmt.Sprintf("不允许的标签 <%s/>", tagName),
				})
				break
			}
			// void 元素自闭合：等同出入栈都跳过
			if voidElements[tagName] {
				break
			}
			// 非 void 元素自闭合：当作开始标签入栈
			tagStack = append(tagStack, tagName)
			lineStack = append(lineStack, prevLine)
			colStack = append(colStack, prevCol)

		case html.CommentToken, html.DoctypeToken:
			// 注释和 DOCTYPE 静默跳过，不影响标签配对
		}
	}
}
