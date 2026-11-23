package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/argontest"
	"go.xchunk.org/lithium"

	"go.xchunk.org/ariel/internal/fakeapi"
	"go.xchunk.org/ariel/internal/telegram"
)

// testChats are the chats configured in tests; "default" is the default one.
var testChats = []telegram.Chat{{Name: "default", ID: 1}, {Name: "team", ID: -2}}

// newTestServer returns an MCP server wired to a fake Bot API.
func newTestServer(t *testing.T) (*argon.Server, *fakeapi.Server) {
	api, url := fakeapi.New(t)
	srv := argon.New("test")
	Register(srv, telegram.New("token", testChats, lithium.WithBaseURL(url)))

	return srv, api
}

func TestListChats(t *testing.T) {
	srv, _ := newTestServer(t)

	got, err := argontest.Call[listChatsOut](t, srv, "listChats", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"chats":[{"name":"default","id":1,"default":true},{"name":"team","id":-2,"default":false}]}`
	if b, _ := json.Marshal(got); string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}

func TestSendMessage(t *testing.T) {
	srv, api := newTestServer(t)

	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"chat": "team", "text": "yo"}); err != nil {
		t.Fatal(err)
	}

	calls := api.CallsTo("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	if calls[0].Params["chat_id"] != 1.0 || calls[1].Params["chat_id"] != -2.0 {
		t.Errorf("unexpected chat IDs: %v, %v", calls[0].Params, calls[1].Params)
	}
}

func TestSendMessageFormatting(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"text": "*hi*", "parse_mode": "MarkdownV2", "silent": true}
	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", in); err != nil {
		t.Fatal(err)
	}

	p := api.CallsTo("sendMessage")[0].Params
	if p["parse_mode"] != "MarkdownV2" || p["disable_notification"] != true {
		t.Errorf("got %v", p)
	}

	in["parse_mode"] = "Markdown"
	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", in); err == nil {
		t.Error("want error for unsupported parse mode")
	}
}

func TestSendMessageSplitsLongText(t *testing.T) {
	srv, api := newTestServer(t)

	text := strings.Repeat("a", 4000) + "\n" + strings.Repeat("b", 200)
	out, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"text": text})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.MessageIDs) != 2 || out.MessageIDs[0] == out.MessageIDs[1] {
		t.Errorf("got message IDs %v, want two distinct", out.MessageIDs)
	}

	calls := api.CallsTo("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("got %d messages, want 2", len(calls))
	}
	if got := calls[0].Params["text"].(string) + calls[1].Params["text"].(string); got != text {
		t.Error("chunks do not add up to the original text")
	}
}

func TestSendMessageUnknownChat(t *testing.T) {
	srv, _ := newTestServer(t)

	_, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"chat": "nope", "text": "hi"})
	if err == nil || !strings.Contains(err.Error(), "unknown chat") {
		t.Errorf("got %v, want unknown chat error", err)
	}
}

func TestEditMessage(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"chat": "team", "message_id": 7, "text": "done"}
	if _, err := argontest.Call[string](t, srv, "editMessage", in); err != nil {
		t.Fatal(err)
	}

	p := api.CallsTo("editMessageText")[0].Params
	if p["chat_id"] != -2.0 || p["message_id"] != 7.0 || p["text"] != "done" {
		t.Errorf("got %v", p)
	}

	in["text"] = strings.Repeat("a", telegram.MaxTextLen+1)
	if _, err := argontest.Call[string](t, srv, "editMessage", in); err == nil {
		t.Error("want error for text over the limit")
	}
}

func TestDeleteMessage(t *testing.T) {
	srv, api := newTestServer(t)

	if _, err := argontest.Call[string](t, srv, "deleteMessage", map[string]any{"message_id": 7}); err != nil {
		t.Fatal(err)
	}

	p := api.CallsTo("deleteMessage")[0].Params
	if p["chat_id"] != 1.0 || p["message_id"] != 7.0 {
		t.Errorf("got %v", p)
	}
}

func TestSendDocument(t *testing.T) {
	srv, api := newTestServer(t)

	path := filepath.Join(t.TempDir(), "build.log")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	in := map[string]any{"path": path, "caption": "build", "silent": true}
	out, err := argontest.Call[sendDocumentOut](t, srv, "sendDocument", in)
	if err != nil {
		t.Fatal(err)
	}
	if out.MessageID == 0 {
		t.Error("got zero message ID")
	}

	c := api.CallsTo("sendDocument")[0]
	if c.Files["document"] != "ok" || c.Params["caption"] != "build" ||
		c.Params["chat_id"] != "1" || c.Params["disable_notification"] != "true" {
		t.Errorf("got %+v", c)
	}
}

func TestSendDocumentContent(t *testing.T) {
	srv, api := newTestServer(t)

	if _, err := argontest.Call[sendDocumentOut](t, srv, "sendDocument", map[string]any{"content": "report"}); err != nil {
		t.Fatal(err)
	}
	if got := api.CallsTo("sendDocument")[0].Files["document"]; got != "report" {
		t.Errorf("got %q", got)
	}

	both := map[string]any{"content": "report", "path": "x"}
	if _, err := argontest.Call[sendDocumentOut](t, srv, "sendDocument", both); err == nil {
		t.Error("want error when both path and content are set")
	}
}
