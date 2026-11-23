package tools

import (
	"strings"
	"testing"
)

func TestCheckRich(t *testing.T) {
	wide := "|" + strings.Repeat(" a |", 21) + "\n|" + strings.Repeat(" - |", 21) + "\n"
	narrow := "| a | b |\n|:--|--:|\n| 1 | 2 |\n"

	valid := []struct{ field, content string }{
		{"markdown", "# Title\n\n" + narrow},
		{"markdown", "![chart](https://example.com/a.png \"Chart\")"},
		{"markdown", "```html\n<tg-button type=\"callback_data\" data=\"x\">x</tg-button>\n![](./local.png)\n```\n"},
		{"markdown", "```\n" + wide + "```\n"},
		{"markdown", "Text\n\n---\n\nMore"},
		{"html", `<img src="https://example.com/a.png"><tg-button type="url" url="https://x.org">Open</tg-button>`},
	}
	for _, tt := range valid {
		if err := checkRich(tt.field, tt.content); err != nil {
			t.Errorf("checkRich(%s, %q): %v", tt.field, tt.content, err)
		}
	}

	invalid := []struct{ field, content, want string }{
		{"markdown", wide, "21 columns"},
		{"markdown", "![](/tmp/a.png)", "only http(s)"},
		{"markdown", `<tg-button type="callback_data" data="x">Go</tg-button>`, "callback_data"},
		{"html", `<video src="file:///a.mp4"></video>`, "only http(s)"},
		{"html", "<p><tg-button type=callback_data data=x>Go</tg-button></p>", "callback_data"},
	}
	for _, tt := range invalid {
		err := checkRich(tt.field, tt.content)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("checkRich(%s, %q): got %v, want error containing %q", tt.field, tt.content, err, tt.want)
		}
	}
}

func TestStripFences(t *testing.T) {
	in := "a\n```go\ncode\n```\nb\n~~~~\nx\n~~~\ny\n~~~~\nc\n```\nunterminated\n"
	want := "a\n```go\n```\nb\n~~~~\n~~~~\nc\n```\n"
	if got := stripFences(in); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
