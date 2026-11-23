package tools

import (
	"strings"
	"testing"

	"go.xchunk.org/argon/argontest"

	"go.mauve.xchunk.org/ariel/internal/fakeapi"
	"go.mauve.xchunk.org/ariel/internal/telegram"
)

func TestSendRichMessage(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"chat": "team", "markdown": "# Report\n\n- [x] done", "silent": true}
	out, err := argontest.Call[sendRichMessageOut](t, srv, "sendRichMessage", in)
	if err != nil {
		t.Fatal(err)
	}
	if out.MessageID == 0 {
		t.Error("got zero message ID")
	}

	p := api.CallsTo("sendRichMessage")[0].Params
	rich, _ := p["rich_message"].(map[string]any)
	if p["chat_id"] != -2.0 || p["disable_notification"] != true ||
		rich["markdown"] != "# Report\n\n- [x] done" || rich["html"] != nil {
		t.Errorf("got %v", p)
	}
}

func TestSendRichMessageHTML(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"html": "<h1>Report</h1>", "skip_entity_detection": true}
	if _, err := argontest.Call[sendRichMessageOut](t, srv, "sendRichMessage", in); err != nil {
		t.Fatal(err)
	}

	rich, _ := api.CallsTo("sendRichMessage")[0].Params["rich_message"].(map[string]any)
	if rich["html"] != "<h1>Report</h1>" || rich["skip_entity_detection"] != true || rich["markdown"] != nil {
		t.Errorf("got %v", rich)
	}
}

func TestSendRichMessageInvalid(t *testing.T) {
	srv, api := newTestServer(t)

	tests := []struct {
		in   map[string]any
		want string
	}{
		{map[string]any{}, "set markdown or html"},
		{map[string]any{"markdown": "a", "html": "b"}, "not both"},
		{map[string]any{"markdown": " \n"}, "empty"},
		{map[string]any{"markdown": strings.Repeat("я", telegram.MaxRichTextLen+1)}, "split it"},
	}
	for _, tt := range tests {
		_, err := argontest.Call[sendRichMessageOut](t, srv, "sendRichMessage", tt.in)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("sendRichMessage(%v): got %v, want error containing %q", tt.in, err, tt.want)
		}
	}
	if n := len(api.CallsTo("sendRichMessage")); n != 0 {
		t.Errorf("sent %d invalid messages", n)
	}
}

func TestEditMessageRich(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"message_id": 7, "markdown": "## Done"}
	if _, err := argontest.Call[string](t, srv, "editMessage", in); err != nil {
		t.Fatal(err)
	}

	p := api.CallsTo("editMessageText")[0].Params
	rich, _ := p["rich_message"].(map[string]any)
	if p["message_id"] != 7.0 || p["text"] != nil || rich["markdown"] != "## Done" {
		t.Errorf("got %v", p)
	}

	for _, bad := range []map[string]any{
		{"message_id": 7},
		{"message_id": 7, "markdown": "a", "text": "b"},
		{"message_id": 7, "html": "a", "parse_mode": "HTML"},
	} {
		if _, err := argontest.Call[string](t, srv, "editMessage", bad); err == nil {
			t.Errorf("editMessage(%v): want error", bad)
		}
	}
}

func TestGuideResource(t *testing.T) {
	srv, _ := newTestServer(t)

	got, err := argontest.ReadResource(t, srv, guideURI)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Text, "# Writing Telegram rich messages") {
		t.Errorf("got %.80q, want the guide", got.Text)
	}
}

func TestSendRichMessageMarkupHint(t *testing.T) {
	srv, api := newTestServer(t)
	api.Reply = func(fakeapi.Call) string {
		return `{"ok":false,"error_code":400,"description":"Bad Request: can't parse rich message"}`
	}

	_, err := argontest.Call[sendRichMessageOut](t, srv, "sendRichMessage", map[string]any{"html": "<blink>x</blink>"})
	if err == nil || !strings.Contains(err.Error(), guideURI) {
		t.Errorf("got %v, want a pointer to the guide", err)
	}
}
