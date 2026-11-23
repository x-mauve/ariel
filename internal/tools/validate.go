package tools

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.mauve.xchunk.org/ariel/internal/telegram"
)

// maxTableColumns is the Bot API limit on the number of columns in a rich
// message table.
const maxTableColumns = 20

var (
	// tableDelimiter matches the delimiter row of a GitHub Flavored Markdown
	// table, e.g. "| --- | :-: |". Its cells are counted with tableCell.
	tableDelimiter = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)+\|?\s*$|^\s*\|\s*:?-+:?\s*\|\s*$`)
	// tableCell matches one cell of a table delimiter row.
	tableCell = regexp.MustCompile(`:?-+:?`)
	// markdownImage matches a Markdown image and captures its URL.
	markdownImage = regexp.MustCompile(`!\[[^\]]*\]\(\s*<?([^\s)>]+)`)
	// htmlMedia matches the src attribute of an HTML media tag and captures its URL.
	htmlMedia = regexp.MustCompile(`(?i)<(?:img|video|audio)\b[^>]*?\bsrc\s*=\s*["']([^"']*)`)
	// callbackButton matches a rich message button that sends a callback query.
	callbackButton = regexp.MustCompile(`(?i)<tg-button\b[^>]*\btype\s*=\s*["']?callback_data`)
)

// checkRich reports content that the Bot API would reject, or that would not
// work through this server, in the rich message field named field ("markdown"
// or "html"). Errors say how to fix the content, so that the model can retry
// on its own.
func checkRich(field, content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("%s is empty", field)
	}
	if n := utf8.RuneCountInString(content); n > telegram.MaxRichTextLen {
		return fmt.Errorf("%s is %d characters long, the limit is %d: "+
			"split it into several messages or send long details with sendDocument",
			field, n, telegram.MaxRichTextLen)
	}

	// Code samples may legitimately contain anything, so only markup outside
	// fenced code blocks is checked.
	markup := content
	if field == "markdown" {
		markup = stripFences(content)
	}

	if callbackButton.MatchString(markup) {
		return fmt.Errorf("callback_data buttons need a bot that answers callback queries, " +
			"and this server does not: use url or copy_text buttons instead")
	}

	media := htmlMedia.FindAllStringSubmatch(markup, -1)
	if field == "markdown" {
		media = append(media, markdownImage.FindAllStringSubmatch(markup, -1)...)
		if err := checkTables(markup); err != nil {
			return err
		}
	}
	for _, m := range media {
		if !remoteMedia(m[1]) {
			return fmt.Errorf("media %q: rich messages accept only http(s) URLs and tg:// links; "+
				"local files are not supported, send them with sendDocument", m[1])
		}
	}

	return nil
}

// checkTables reports a Markdown table with more columns than the Bot API allows.
func checkTables(markdown string) error {
	for line := range strings.Lines(markdown) {
		if !strings.Contains(line, "|") || !tableDelimiter.MatchString(line) {
			continue
		}
		if n := len(tableCell.FindAllString(line, -1)); n > maxTableColumns {
			return fmt.Errorf("a table has %d columns, the limit is %d: "+
				"transpose it, split it into several tables or drop columns", n, maxTableColumns)
		}
	}

	return nil
}

// remoteMedia reports whether url is a media source the Bot API can fetch on
// its own: an HTTP(S) URL or a tg:// link to a file already on Telegram servers.
func remoteMedia(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "tg://")
}

// stripFences returns markdown with the contents of fenced code blocks
// removed. The fence lines themselves are kept, and an unterminated block runs
// to the end of the text, as in CommonMark.
func stripFences(markdown string) string {
	var b strings.Builder
	fence := ""
	for line := range strings.Lines(markdown) {
		trimmed := strings.TrimLeft(line, " ")
		if fence == "" {
			if marker := fenceMarker(trimmed); marker != "" {
				fence = marker
			}
			b.WriteString(line)
			continue
		}
		if strings.HasPrefix(trimmed, fence) && strings.TrimSpace(strings.TrimLeft(trimmed, fence[:1])) == "" {
			fence = ""
			b.WriteString(line)
		}
	}

	return b.String()
}

// fenceMarker returns the opening code fence that line starts with, e.g. "```"
// or "~~~~", or "" if line does not open a fenced code block.
func fenceMarker(line string) string {
	for _, c := range []byte{'`', '~'} {
		n := 0
		for n < len(line) && line[n] == c {
			n++
		}
		if n >= 3 {
			return line[:n]
		}
	}

	return ""
}
