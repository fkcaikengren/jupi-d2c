package services

import (
	"strings"
	"testing"
)

// TestCheckHTML_BodyFragment 验证用户提供的 body 片段(包含注释 + void 元素 + 嵌套 div)
// 应该通过 CheckHTML 的简化规则。
func TestCheckHTML_BodyFragment(t *testing.T) {
	body := `<body>
  <div name="服务">
    <!-- 头部 -->
    <div name="头部">
      <!-- 状态栏 -->
      <div name="组件/状态栏">
        <div name="BG"></div>
        <img src="http://localhost:5678/uploads/2026-07-01_16-00-39_new/1782892840037-687a8ee3f1ff.svg" alt="Right Side" />
        <img src="http://localhost:5678/uploads/2026-07-01_16-00-39_new/1782892840042-64cf3a1b0e80.svg" alt="Left Side" />
      </div>
      <!-- 标题栏 -->
      <div name="组件/标题栏 1">
        <div name="组件/标题区（左、中、右）">
          <div name="Frame 2902">
            <span>企业服务</span>
          </div>
        </div>
        <img src="http://localhost:5678/uploads/2026-07-01_16-00-39_new/1782892840072-0722156d7c38.svg" alt="组件/标题区（左、中、右） 1" />
      </div>
    </div>
    <!-- 容器 959 -->
    <div name="容器 959">
      <div name="容器 945">
        <div name="Frame 2955">
          <div name="Frame 2948">
            <span>培训课程推荐</span>
            <span>每天5分钟，轻松获得学习证明</span>
          </div>
          <div name="小按钮">
            <span>查看更多</span>
            <img src="http://localhost:5678/uploads/2026-07-01_16-00-39_new/1782892840102-2291c834c0f1.svg" alt="arrow-left" />
          </div>
        </div>
      </div>
    </div>
  </div>
</body>`

	errs := CheckHTML(body)
	if len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("unexpected error at %d:%d: %s", e.Line, e.Col, e.Message)
		}
	}
}

// TestCheckHTML_NoRoot 验证多顶层 div(无单一根节点)的 HTML 片段也能通过。
func TestCheckHTML_NoRoot(t *testing.T) {
	html := `<div>a</div><div>b</div><span>c</span>`
	errs := CheckHTML(html)
	if len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("unexpected error at %d:%d: %s", e.Line, e.Col, e.Message)
		}
	}
}

// TestCheckHTML_VoidVariants 验证 void 元素的三种写法都合法。
func TestCheckHTML_VoidVariants(t *testing.T) {
	cases := []string{
		`<div><img src="a"></div>`,            // void 不闭合
		`<div><img src="a"/></div>`,            // void 自闭合
		`<div><img src="a"></img></div>`,       // void 显式闭合
		`<div><br><hr><input></div>`,           // 多个 void 不闭合
	}
	for _, c := range cases {
		errs := CheckHTML(c)
		if len(errs) != 0 {
			t.Errorf("case %q: unexpected errors: %v", c, errs)
		}
	}
}

// TestCheckHTML_TagMismatch 验证开始/结束标签不匹配仍能报错。
func TestCheckHTML_TagMismatch(t *testing.T) {
	cases := []string{
		`<div><span></div>`,     // span 关闭时栈顶是 div,不匹配
		`<div></span></div>`,    // 多余结束标签
		`<div>`,                 // 未闭合
		`<a><b><c></a></b>`,     // 嵌套错位
	}
	for _, c := range cases {
		errs := CheckHTML(c)
		if len(errs) == 0 {
			t.Errorf("case %q: expected errors, got none", c)
		}
	}
}

// TestCheckHTML_CommentsAndDoctype 验证注释和 DOCTYPE 静默通过。
func TestCheckHTML_CommentsAndDoctype(t *testing.T) {
	cases := []string{
		`<!DOCTYPE html><div>hello</div>`,
		`<div><!-- 注释 --></div>`,
		`<div><?xml version="1.0"?><span>x</span></div>`,
	}
	for _, c := range cases {
		errs := CheckHTML(c)
		if len(errs) != 0 {
			t.Errorf("case %q: unexpected errors: %v", c, errs)
		}
	}
}

// TestCheckHTML_ForbiddenTags 验证黑名单标签（script / link / style）任何写法都报错。
// 即便是 <link>（void 元素）也会被黑名单拦截。
func TestCheckHTML_ForbiddenTags(t *testing.T) {
	cases := []string{
		`<div><script>alert(1)</script></div>`,
		`<div><script src="a.js"></script></div>`,
		`<div><link rel="stylesheet" href="a.css"></div>`,
		`<div><link rel="stylesheet" href="a.css"/></div>`,
		`<div><style>body{color:red}</style></div>`,
		`<div><style>x</style></div></div>`, // 故意带未闭合的 </div> 验证仍是 forbidden 优先
		`<div></script></div>`,               // 只有结束标签,没有开始标签,依然报 forbidden
		`<script>`,                           // 未闭合 + 禁止
	}
	for _, c := range cases {
		errs := CheckHTML(c)
		if len(errs) == 0 {
			t.Errorf("case %q: expected at least one forbidden-tag error, got none", c)
			continue
		}
		// 至少要有一条错误消息提到"不允许的标签"
		hit := false
		for _, e := range errs {
			if strings.HasPrefix(e.Message, "不允许的标签") {
				hit = true
				break
			}
		}
		if !hit {
			t.Errorf("case %q: errors do not include a forbidden-tag error: %v", c, errs)
		}
	}
}
