package tools

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"go.xchunk.org/ariel/internal/telegram"
)

// checkRich reports content that the Bot API would reject in the rich message
// field named field. Errors say how to fix the content, so that the model can
// retry on its own.
func checkRich(field, content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("%s is empty", field)
	}
	if n := utf8.RuneCountInString(content); n > telegram.MaxRichTextLen {
		return fmt.Errorf("%s is %d characters long, the limit is %d: "+
			"split it into several messages or send long details with sendDocument",
			field, n, telegram.MaxRichTextLen)
	}

	return nil
}
