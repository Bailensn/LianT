// Package format 定义核心与各平台之间的中性文档 NDoc。
//
// 它刻意做小：三种块（p / code / quote）与六种行内（text / bold /
// italic / strike / code / link），span 不允许嵌套。任何更丰富的
// 结构（表格、嵌入、线程、表情包）都属于平台适配器，不属于核心。
package format

import (
	"errors"
	"fmt"
	"strings"
)

// Version 是 NDoc 的 schema 版本，与线上协议版本解耦。
const Version = 1

// 单条 NDoc 的硬上限，用来挡住畸形或恶意的输入。
const (
	MaxBlocks     = 200
	MaxSpans      = 20
	MaxURL        = 2048
	MaxBlockChars = 10000
	MaxTextBytes  = 32 * 1024
)

// 块类型。
const (
	BlockParagraph = "p"
	BlockCode      = "code"
	BlockQuote     = "quote"
)

// 行内类型。
const (
	SpanText   = "text"
	SpanBold   = "bold"
	SpanItalic = "italic"
	SpanStrike = "strike"
	SpanCode   = "code"
	SpanLink   = "link"
)

// Doc 是一条消息的中性表示。
type Doc struct {
	V      int     `json:"v"`
	Blocks []Block `json:"blocks"`
}

// Block 是段落、代码块或引用块。
//
//   - p / quote 使用 Spans；
//   - code 使用 Text（纯文本，可带 Lang）。
type Block struct {
	Type  string `json:"type"`
	Lang  string `json:"lang,omitempty"`
	Text  string `json:"text,omitempty"`
	Spans []Span `json:"spans,omitempty"`
}

// Span 是一段行内内容。link 必须有 URL，其余类型忽略 URL。
type Span struct {
	T    string `json:"t"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Validate 检查文档结构是否合法。调用方应在渲染或落库前调用。
func Validate(d *Doc) error {
	if d == nil {
		return errors.New("format: 空文档")
	}
	if d.V != 0 && d.V != Version {
		return fmt.Errorf("format: 不支持的文档版本 %d", d.V)
	}
	if len(d.Blocks) > MaxBlocks {
		return fmt.Errorf("format: 块数量 %d 超过上限 %d", len(d.Blocks), MaxBlocks)
	}

	total := 0
	for i := range d.Blocks {
		b := &d.Blocks[i]
		switch b.Type {
		case BlockParagraph, BlockQuote:
			if len(b.Spans) == 0 {
				return fmt.Errorf("format: 第 %d 块没有内容", i)
			}
			if len(b.Spans) > MaxSpans {
				return fmt.Errorf("format: 第 %d 块 span 数 %d 超过上限 %d", i, len(b.Spans), MaxSpans)
			}
			for j := range b.Spans {
				s := &b.Spans[j]
				if !validSpanType(s.T) {
					return fmt.Errorf("format: 第 %d 块第 %d 个 span 类型 %q 未知", i, j, s.T)
				}
				if s.T == SpanLink {
					if s.URL == "" {
						return fmt.Errorf("format: 第 %d 块第 %d 个 link 缺少 url", i, j)
					}
					if len(s.URL) > MaxURL {
						return fmt.Errorf("format: 第 %d 块第 %d 个 link 的 url 过长", i, j)
					}
				}
				total += len(s.Text) + len(s.URL)
			}
		case BlockCode:
			if len(b.Text) > MaxBlockChars {
				return fmt.Errorf("format: 第 %d 块代码过长", i)
			}
			total += len(b.Text)
		default:
			return fmt.Errorf("format: 第 %d 块类型 %q 未知", i, b.Type)
		}
	}
	if total > MaxTextBytes {
		return fmt.Errorf("format: 文档文本量 %d 字节超过上限 %d", total, MaxTextBytes)
	}
	return nil
}

func validSpanType(t string) bool {
	switch t {
	case SpanText, SpanBold, SpanItalic, SpanStrike, SpanCode, SpanLink:
		return true
	default:
		return false
	}
}

// PlainText 把文档压成纯文本，用于通知、搜索与降级（协议里的 body.plain）。
// 它不保证与渲染后的字符串逐字相等，但必须语义一致。
func PlainText(d *Doc) string {
	if d == nil {
		return ""
	}
	var b strings.Builder
	for i := range d.Blocks {
		if i > 0 {
			b.WriteByte('\n')
		}
		bk := &d.Blocks[i]
		if bk.Type == BlockCode {
			b.WriteString(bk.Text)
			continue
		}
		for j := range bk.Spans {
			b.WriteString(bk.Spans[j].Text)
		}
	}
	return b.String()
}
