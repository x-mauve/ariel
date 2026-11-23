package telegram

import (
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	// MaxTextLen is the Bot API limit on message text length, in UTF-16 code units.
	MaxTextLen = 4096
	// MaxCaptionLen is the Bot API limit on caption length, in UTF-16 code units.
	MaxCaptionLen = 1024
	// MaxUploadSize is the Bot API limit on the size of an uploaded file, in bytes.
	MaxUploadSize = 50 << 20
)

// CheckParseMode reports an error if mode is not a Bot API parse mode for
// plain messages. An empty mode sends the text as is.
func CheckParseMode(mode string) error {
	switch mode {
	case "", "MarkdownV2", "HTML":
		return nil
	default:
		return fmt.Errorf("unsupported parse mode %q, want MarkdownV2 or HTML", mode)
	}
}

// SplitText splits text into chunks of at most limit UTF-16 code units each.
// It prefers to break after a newline, then after a space, and cuts mid-word
// only when a chunk has neither. The chunks concatenate back to text.
func SplitText(text string, limit int) []string {
	var chunks []string
	for UTF16Len(text) > limit {
		cut, lastNewline, lastSpace, width := 0, 0, 0, 0
		for i, r := range text {
			if width += utf16.RuneLen(r); width > limit {
				break
			}
			cut = i + utf8.RuneLen(r)
			switch r {
			case '\n':
				lastNewline = cut
			case ' ':
				lastSpace = cut
			}
		}

		if lastNewline > 0 {
			cut = lastNewline
		} else if lastSpace > 0 {
			cut = lastSpace
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}

	return append(chunks, text)
}

// UTF16Len returns the length of s in UTF-16 code units, as Telegram counts
// the length of plain messages and captions.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}

	return n
}
