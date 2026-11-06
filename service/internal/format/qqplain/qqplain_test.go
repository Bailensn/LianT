package qqplain_test

import (
	"testing"

	"github.com/Bailensn/LianT/service/internal/format"
	"github.com/Bailensn/LianT/service/internal/format/qqplain"
)

func mustFormat(t *testing.T) format.Format {
	t.Helper()
	f, ok := format.Get(qqplain.Name)
	if !ok {
		t.Fatalf("格式 %q 没有注册", qqplain.Name)
	}
	return f
}

func TestRender(t *testing.T) {
	f := mustFormat(t)
	doc := &format.Doc{
		V: format.Version,
		Blocks: []format.Block{
			{
				Type: format.BlockParagraph,
				Spans: []format.Span{
					{T: format.SpanText, Text: "看 "},
					{T: format.SpanBold, Text: "这里"},
					{T: format.SpanText, Text: " "},
					{T: format.SpanLink, Text: "文档", URL: "https://example.com"},
				},
			},
			{Type: format.BlockCode, Text: "fmt.Println()"},
		},
	}

	got, err := f.Render(doc)
	if err != nil {
		t.Fatalf("Render 返回错误: %v", err)
	}
	want := "看 这里 文档 https://example.com\nfmt.Println()"
	if got != want {
		t.Fatalf("Render = %q，期望 %q", got, want)
	}
}

func TestRenderLinkTextEqualsURL(t *testing.T) {
	f := mustFormat(t)
	doc := &format.Doc{
		V: format.Version,
		Blocks: []format.Block{{
			Type:  format.BlockParagraph,
			Spans: []format.Span{{T: format.SpanLink, Text: "https://example.com", URL: "https://example.com"}},
		}},
	}
	got, err := f.Render(doc)
	if err != nil {
		t.Fatalf("Render 返回错误: %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("Render = %q，URL 不应重复", got)
	}
}

func TestParsePlainText(t *testing.T) {
	f := mustFormat(t)
	doc, err := f.Parse("你好", nil)
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}
	if len(doc.Blocks) != 1 || len(doc.Blocks[0].Spans) != 1 {
		t.Fatalf("Parse 结构不对: %+v", doc)
	}
	if got := doc.Blocks[0].Spans[0].Text; got != "你好" {
		t.Fatalf("Parse 文本 = %q", got)
	}
}

func TestParseEntitiesUseRuneOffsets(t *testing.T) {
	f := mustFormat(t)
	// 「看这里」是三个码点；粗体覆盖第一个字。
	doc, err := f.Parse("看这里", []format.Entity{
		{Type: format.EntityBold, Offset: 0, Length: 1},
	})
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}

	spans := doc.Blocks[0].Spans
	if len(spans) != 2 {
		t.Fatalf("期望 2 个 span，实际 %d: %+v", len(spans), spans)
	}
	if spans[0].T != format.SpanBold || spans[0].Text != "看" {
		t.Fatalf("第一个 span = %+v", spans[0])
	}
	if spans[1].T != format.SpanText || spans[1].Text != "这里" {
		t.Fatalf("第二个 span = %+v", spans[1])
	}
}

func TestParseInvalidEntitiesFallBack(t *testing.T) {
	f := mustFormat(t)

	cases := [][]format.Entity{
		{{Type: format.EntityBold, Offset: 2, Length: 5}},           // 越界
		{{Type: format.EntityBold, Offset: 0, Length: 0}},           // 长度非法
		{{Type: format.EntityBold, Offset: 1, Length: 2},            // 重叠
			{Type: format.EntityItalic, Offset: 2, Length: 2}},
	}

	for i, ents := range cases {
		doc, err := f.Parse("看这里", ents)
		if err != nil {
			t.Fatalf("case %d: Parse 返回错误: %v", i, err)
		}
		spans := doc.Blocks[0].Spans
		if len(spans) != 1 || spans[0].T != format.SpanText || spans[0].Text != "看这里" {
			t.Fatalf("case %d: 非法实体应整体降级，实际 %+v", i, spans)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	f := mustFormat(t)

	doc, err := f.Parse("纯文本消息", nil)
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}
	rendered, err := f.Render(doc)
	if err != nil {
		t.Fatalf("Render 返回错误: %v", err)
	}
	if rendered != "纯文本消息" {
		t.Fatalf("round trip = %q", rendered)
	}
}
