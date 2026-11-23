package telegram

import (
	"context"
	"strings"
	"testing"

	"go.xchunk.org/lithium"

	"go.xchunk.org/ariel/internal/fakeapi"
)

// testChats are the chats configured in tests; "default" is the default one.
var testChats = []Chat{{Name: "default", ID: 1}, {Name: "team", ID: -2}}

func newTestBot(t *testing.T) (*Bot, *fakeapi.Server) {
	api, url := fakeapi.New(t)
	return New("token", testChats, lithium.WithBaseURL(url)), api
}

func TestChatID(t *testing.T) {
	b, _ := newTestBot(t)

	for name, want := range map[string]int64{"": 1, "default": 1, "team": -2} {
		if got, err := b.ChatID(name); err != nil || got != want {
			t.Errorf("ChatID(%q) = %d, %v, want %d", name, got, err, want)
		}
	}
	if _, err := b.ChatID("nope"); err == nil || !strings.Contains(err.Error(), "unknown chat") {
		t.Errorf("got %v, want unknown chat error", err)
	}
}

func TestCheck(t *testing.T) {
	b, _ := newTestBot(t)

	name, err := b.Check(context.Background())
	if err != nil || name != "test_bot" {
		t.Errorf("got %q, %v", name, err)
	}
}

func TestRetryFloodControl(t *testing.T) {
	b, api := newTestBot(t)
	attempts := 0
	api.Reply = func(fakeapi.Call) string {
		if attempts++; attempts == 1 {
			return `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`
		}
		return ""
	}

	if _, err := b.Send[Message](context.Background(), SendMessage{ChatID: 1, Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	if n := len(api.CallsTo("sendMessage")); n != 2 {
		t.Errorf("got %d attempts, want 2", n)
	}
}

func TestNoRetryOnBadRequest(t *testing.T) {
	b, api := newTestBot(t)
	api.Reply = func(fakeapi.Call) string {
		return `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`
	}

	if _, err := b.Send[Message](context.Background(), SendMessage{ChatID: 1, Text: "hi"}); err == nil {
		t.Fatal("want error")
	}
	if n := len(api.CallsTo("sendMessage")); n != 1 {
		t.Errorf("got %d attempts, want 1", n)
	}
}

func TestSplitText(t *testing.T) {
	tests := []struct {
		text  string
		limit int
		want  []string
	}{
		{"short", 10, []string{"short"}},
		{"one two three", 8, []string{"one two ", "three"}},
		{"line1\nline two", 12, []string{"line1\n", "line two"}},
		{"abcdefgh", 3, []string{"abc", "def", "gh"}},
		// 😀 takes two UTF-16 code units, so only two fit into four.
		{"😀😀😀", 4, []string{"😀😀", "😀"}},
	}
	for _, tt := range tests {
		got := SplitText(tt.text, tt.limit)
		if strings.Join(got, "|") != strings.Join(tt.want, "|") {
			t.Errorf("SplitText(%q, %d) = %q, want %q", tt.text, tt.limit, got, tt.want)
		}
	}
}
