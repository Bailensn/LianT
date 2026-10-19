package bot

import (
	"html"
	"regexp"
	"strings"
)

var (
	cardBoldRe    = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	cardHeadRe    = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	cardListRe    = regexp.MustCompile(`(?m)^[-*]\s+`)
	cardCodeRe    = regexp.MustCompile("`([^`\\n]+)`")
	cardLinkRe    = regexp.MustCompile(`\[([^\]\n]+)\]\((https?://[^)\s]+)\)`)
	cardTitleTrim = regexp.MustCompile(`\s+`)
)

// CardDisplay 是后台“最近消息”里保存的卡片文本。
// 以 ## 开头，qbot 页面会按卡片气泡解析。
func CardDisplay(title, content string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "群消息"
	}
	return "## " + title + "\n\n" + strings.TrimSpace(content)
}

// BuildCardHTML 把 qbot 的 Markdown 卡片转成 Telegram HTML。
// Telegram 没有 QQ 那种卡片消息，HTML 富文本是最接近的等价形式。
func BuildCardHTML(title, content string) string {
	title = cardTitleTrim.ReplaceAllString(strings.TrimSpace(title), " ")
	if title == "" {
		title = "群消息"
	}
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80]) + "…"
	}
	body := html.EscapeString(strings.TrimSpace(content))
	body = cardBoldRe.ReplaceAllString(body, "<b>$1</b>")
	body = cardHeadRe.ReplaceAllString(body, "<b>$1</b>")
	body = cardListRe.ReplaceAllString(body, "• ")
	body = cardCodeRe.ReplaceAllString(body, "<code>$1</code>")
	body = cardLinkRe.ReplaceAllString(body, `<a href="$2">$1</a>`)
	return "<b>" + html.EscapeString(title) + "</b>\n\n" + body
}
