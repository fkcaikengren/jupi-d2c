package services

import (
	"strings"
	"testing"
)

// TestExtractBodyContent_HappyPath 验证最常见的"一段 body"输入能正确提取。
func TestExtractBodyContent_HappyPath(t *testing.T) {
	in := "<body><div>hello</div></body>"
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

// TestExtractBodyContent_WithAttributes 验证 body 开标签带属性时也能正确处理。
func TestExtractBodyContent_WithAttributes(t *testing.T) {
	in := `<body class="x" data-id="1"><div>a</div></body>`
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

// TestExtractBodyContent_CaseInsensitive 验证大小写不敏感。
func TestExtractBodyContent_CaseInsensitive(t *testing.T) {
	in := `<BODY><div>a</div></BODY>`
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

// TestExtractBodyContent_EmptyAndWhitespace 验证空输入 / 纯空白输入报错。
func TestExtractBodyContent_EmptyAndWhitespace(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"\n\n\t",
	}
	for _, c := range cases {
		_, err := extractBodyContent(c)
		if err == nil {
			t.Errorf("case %q: expected error, got nil", c)
		}
	}
}

// TestExtractBodyContent_NoBody 验证完全没有 body 标签时报错。
func TestExtractBodyContent_NoBody(t *testing.T) {
	_, err := extractBodyContent("<div>only div here</div>")
	if err == nil {
		t.Fatal("expected error for input without body, got nil")
	}
	if !strings.Contains(err.Error(), "<body>") {
		t.Errorf("error message should mention <body>, got: %v", err)
	}
}

// TestExtractBodyContent_Unclosed 验证只有 <body> 没有 </body> 报错。
func TestExtractBodyContent_Unclosed(t *testing.T) {
	cases := []string{
		`<body><div>no close`,
		`<body>`,
	}
	for _, c := range cases {
		_, err := extractBodyContent(c)
		if err == nil {
			t.Errorf("case %q: expected error, got nil", c)
		}
	}
}

// TestExtractBodyContent_EmptyBody 验证空 body 标签 <body></body> 报错。
func TestExtractBodyContent_EmptyBody(t *testing.T) {
	cases := []string{
		`<body></body>`,
		`<body>   </body>`,
		`<body>
		</body>`,
	}
	for _, c := range cases {
		_, err := extractBodyContent(c)
		if err == nil {
			t.Errorf("case %q: expected error for empty body, got nil", c)
		}
	}
}

// TestExtractBodyContent_MentionedInText 是核心 bug 场景:
// AI 在说明文字里提到 <body> 这个字符串,后面才是真正的答案。
// 旧版字符串匹配会误中说明里的 <body>,新版 tokenizer 应该正确取出真正的配对。
func TestExtractBodyContent_MentionedInText(t *testing.T) {
	in := `要使用 body 标签就写 <body>，下面是真正的答案:
<body><div>real answer</div></body>`
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `<body><div>real answer</div></body>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExtractBodyContent_CommentWithBody 验证注释里的 <body>...</body> 不会误中。
func TestExtractBodyContent_CommentWithBody(t *testing.T) {
	in := `<!-- example: <body>old</body> --><body>new</body>`
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "<body>new</body>" {
		t.Errorf("got %q, want %q", got, "<body>new</body>")
	}
}

// TestExtractBodyContent_NestedBodies 验证嵌套 body 时,取最外层配对。
func TestExtractBodyContent_NestedBodies(t *testing.T) {
	in := `<body><body><div>inner</div></body></body>`
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

// TestExtractBodyContent_MultipleBodies 验证多个 body 出现时,取第一个配对。
func TestExtractBodyContent_MultipleBodies(t *testing.T) {
	in := `<body><div>first</div></body>
some text in between
<body><div>second</div></body>`
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `<body><div>first</div></body>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExtractBodyContent_InMarkdownFence 验证 markdown 代码块包裹的场景能正常提取。
// AI 经常用 ```html ... ``` 包裹答案。
func TestExtractBodyContent_InMarkdownFence(t *testing.T) {
	in := "```html\n<body><div>a</div></body>\n```"
	got, err := extractBodyContent(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "<body><div>a</div></body>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
