// Package qqplain 是 QQ 的纯文本格式适配器。
//
// QQ 的 markdown 是「模板 + 参数」结构，模板 ID 与 keyboard 都属于
// 适配器内部，不进 NDoc。v1 出站一律降级为纯文本，入站只在平台
// 给出 entities 时才还原行内标记。
package qqplain

import (
	"sort"
	"strings"

	"github.com/Bailensn/LianT/service/internal/format"
)

// Name 是注册名，与 platform.Adapter.Format() 的返回值一致。
const Name = "qq-plain"

func init() {
	format.Register(impl{})
}

type impl struct{}

func (impl) Name() string { return Name }

// Render 把 NDoc 压成纯文本：粗体/斜体/删除线/行内代码只保留文字，
// 链接保留「文字 + 空格 + URL」；块之间用换行分隔。
func (impl) Render(doc *format.Doc) (string, error) {
	if err := format.Validate(doc); err != nil {
		return "", err
	}
	var b strings.Builder
	for i := range doc.Blocks {
		if i > 0 {
			b.WriteByte('\n')
		}
		blk := &doc.Blocks[i]
		if blk.Type == format.BlockCode {
			b.WriteString(blk.Text)
			continue
		}
		for j := range blk.Spans {
			sp := &blk.Spans[j]
			switch sp.T {
			case format.SpanLink:
				b.WriteString(sp.Text)
				if sp.URL != "" && sp.URL != sp.Text {
					b.WriteString(" ")
					b.WriteString(sp.URL)
				}
			default:
				b.WriteString(sp.Text)
			}
		}
	}
	return b.String(), nil
}

// Parse 把纯文本还原成单个段落。entities 合法时按偏移切分，
// 不合法就整体降级为纯文本——解析失败不应该丢消息。
func (impl) Parse(text string, ents []format.Entity) (*format.Doc, error) {
	spans := []format.Span{{T: format.SpanText, Text: text}}
	if len(ents) > 0 {
		if split, ok := splitByEntity(text, ents); ok {
			spans = split
		}
	}
	return &format.Doc{
		V: format.Version,
		Blocks: []format.Block{{
			Type:  format.BlockParagraph,
			Spans: spans,
		}},
	}, nil
}

// splitByEntity 按码点把 text 切分为 span。任何越界或重叠都返回 false，
// 由调用方整体降级。
func splitByEntity(text string, ents []format.Entity) ([]format.Span, bool) {
	runes := []rune(text)
	sorted := make([]format.Entity, len(ents))
	copy(sorted, ents)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Offset < sorted[j].Offset
	})

	spans := make([]format.Span, 0, len(sorted)+1)
	pos := 0
	for _, e := range sorted {
		if e.Offset < pos || e.Length <= 0 || e.Offset+e.Length > len(runes) {
			return nil, false
		}
		if e.Offset > pos {
			spans = append(spans, format.Span{
				T:    format.SpanText,
				Text: string(runes[pos:e.Offset]),
			})
		}
		spanType := spanTypeOf(e.Type)
		if spanType == "" {
			spanType = format.SpanText
		}
		sp := format.Span{
			T:    spanType,
			Text: string(runes[e.Offset : e.Offset+e.Length]),
		}
		if spanType == format.SpanLink && e.URL != "" {
			sp.URL = e.URL
		}
		spans = append(spans, sp)
		pos = e.Offset + e.Length
	}
	if pos < len(runes) {
		spans = append(spans, format.Span{
			T:    format.SpanText,
			Text: string(runes[pos:]),
		})
	}
	if len(spans) == 0 {
		return nil, false
	}
	return spans, true
}

func spanTypeOf(entityType string) string {
	switch entityType {
	case format.EntityBold:
		return format.SpanBold
	case format.EntityItalic:
		return format.SpanItalic
	case format.EntityStrike:
		return format.SpanStrike
	case format.EntityCode, format.EntityPre:
		return format.SpanCode
	case format.EntityLink:
		return format.SpanLink
	default:
		return ""
	}
}
