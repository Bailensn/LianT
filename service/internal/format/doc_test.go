package format_test

import (
	"strings"
	"testing"

	"github.com/Bailensn/LianT/service/internal/format"
)

func validDoc() *format.Doc {
	return &format.Doc{
		V: format.Version,
		Blocks: []format.Block{
			{
				Type: format.BlockParagraph,
				Spans: []format.Span{
					{T: format.SpanText, Text: "看 "},
					{T: format.SpanBold, Text: "这里"},
					{T: format.SpanLink, Text: "文档", URL: "https://example.com"},
				},
			},
			{Type: format.BlockCode, Lang: "go", Text: "fmt.Println()"},
		},
	}
}

func TestValidateAcceptsValidDoc(t *testing.T) {
	if err := format.Validate(validDoc()); err != nil {
		t.Fatalf("合法文档被拒绝: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		doc  *format.Doc
	}{
		{"nil", nil},
		{"版本不支持", &format.Doc{V: 2}},
		{"块类型未知", &format.Doc{V: 1, Blocks: []format.Block{{Type: "table"}}}},
		{"段落没有 span", &format.Doc{V: 1, Blocks: []format.Block{{Type: format.BlockParagraph}}}},
		{"span 类型未知", &format.Doc{V: 1, Blocks: []format.Block{{
			Type:  format.BlockParagraph,
			Spans: []format.Span{{T: "rainbow", Text: "x"}},
		}}}},
		{"link 缺 url", &format.Doc{V: 1, Blocks: []format.Block{{
			Type:  format.BlockParagraph,
			Spans: []format.Span{{T: format.SpanLink, Text: "x"}},
		}}}},
		{"link url 过长", &format.Doc{V: 1, Blocks: []format.Block{{
			Type:  format.BlockParagraph,
			Spans: []format.Span{{T: format.SpanLink, Text: "x", URL: "https://example.com/" + strings.Repeat("a", format.MaxURL)}},
		}}}},
		{"代码块过长", &format.Doc{V: 1, Blocks: []format.Block{{
			Type: format.BlockCode,
			Text: strings.Repeat("a", format.MaxBlockChars+1),
		}}}},
		{"总文本超限", &format.Doc{V: 1, Blocks: []format.Block{{
			Type:  format.BlockParagraph,
			Spans: []format.Span{{T: format.SpanText, Text: strings.Repeat("a", format.MaxTextBytes+1)}},
		}}}},
	}

	for _, c := range cases {
		if err := format.Validate(c.doc); err == nil {
			t.Errorf("%s: 期望报错，实际通过", c.name)
		}
	}
}

func TestValidateRejectsTooManyBlocks(t *testing.T) {
	doc := &format.Doc{V: 1}
	for i := 0; i <= format.MaxBlocks; i++ {
		doc.Blocks = append(doc.Blocks, format.Block{
			Type:  format.BlockParagraph,
			Spans: []format.Span{{T: format.SpanText, Text: "x"}},
		})
	}
	if err := format.Validate(doc); err == nil {
		t.Fatal("超过块数上限没有报错")
	}
}

func TestPlainText(t *testing.T) {
	got := format.PlainText(validDoc())
	want := "看 这里文档\nfmt.Println()"
	if got != want {
		t.Fatalf("PlainText = %q，期望 %q", got, want)
	}
	if format.PlainText(nil) != "" {
		t.Fatal("nil 文档应得到空串")
	}
}
