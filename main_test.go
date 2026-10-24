package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/argontest"
	"go.xchunk.org/lithium"
)

// apiCall is a Bot API request received by fakeAPI.
type apiCall struct {
	Method string
	Params map[string]any
	// Files maps multipart file fields to their contents.
	Files map[string]string
}

// fakeAPI is an in-memory Bot API server. By default every call succeeds
// with a message carrying a fresh message_id; reply overrides that per method.
type fakeAPI struct {
	mu     sync.Mutex
	calls  []apiCall
	nextID int64
	// reply returns the raw JSON envelope for a call, or "" for the default.
	reply func(c apiCall) string
}

func newFakeAPI(t *testing.T) (*fakeAPI, string) {
	f := &fakeAPI{nextID: 100}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)

	return f, srv.URL
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	c := apiCall{
		Method: r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:],
		Params: map[string]any{},
		Files:  map[string]string{},
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for k, v := range r.MultipartForm.Value {
			c.Params[k] = v[0]
		}
		for k, fh := range r.MultipartForm.File {
			file, _ := fh[0].Open()
			data, _ := io.ReadAll(file)
			file.Close()
			c.Files[k] = string(data)
		}
	} else if err := json.NewDecoder(r.Body).Decode(&c.Params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.nextID++
	id := f.nextID
	reply := f.reply
	f.mu.Unlock()

	body := ""
	if reply != nil {
		body = reply(c)
	}
	if body == "" && boolMethods[c.Method] {
		body = `{"ok":true,"result":true}`
	}
	if body == "" {
		body = fmt.Sprintf(`{"ok":true,"result":{"message_id":%d,"username":"test_bot"}}`, id)
	}
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, body)
}

// boolMethods are the Bot API methods whose result is true rather than an object.
var boolMethods = map[string]bool{
	"answerCallbackQuery": true,
	"deleteMessage":       true,
	"pinChatMessage":      true,
}

// callsTo returns the received calls of method.
func (f *fakeAPI) callsTo(method string) []apiCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []apiCall
	for _, c := range f.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}

	return out
}

// testChats are the chats configured in tests; "default" is the default one.
var testChats = []chat{{Name: "default", ID: 1}, {Name: "team", ID: -2}}

// newTestServer returns an MCP server wired to a fake Bot API.
func newTestServer(t *testing.T) (*argon.Server, *fakeAPI) {
	api, url := newFakeAPI(t)
	srv := argon.New("test")
	registerTools(srv, newBot("token", testChats, lithium.WithBaseURL(url)))

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

	calls := api.callsTo("sendMessage")
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

	p := api.callsTo("sendMessage")[0].Params
	if p["parse_mode"] != "MarkdownV2" || p["disable_notification"] != true {
		t.Errorf("got %v", p)
	}

	in["parse_mode"] = "Markdown"
	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", in); err == nil {
		t.Error("want error for unsupported parse mode")
	}
}

func TestSendMessageLevel(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"text": "disk full", "level": "error"}
	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", in); err != nil {
		t.Fatal(err)
	}
	if got := api.callsTo("sendMessage")[0].Params["text"]; got != "🔴 disk full" {
		t.Errorf("got %q", got)
	}

	in["level"] = "fatal"
	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", in); err == nil {
		t.Error("want error for unsupported level")
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

	calls := api.callsTo("sendMessage")
	if len(calls) != 2 {
		t.Fatalf("got %d messages, want 2", len(calls))
	}
	if got := calls[0].Params["text"].(string) + calls[1].Params["text"].(string); got != text {
		t.Error("chunks do not add up to the original text")
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
		got := splitText(tt.text, tt.limit)
		if strings.Join(got, "|") != strings.Join(tt.want, "|") {
			t.Errorf("splitText(%q, %d) = %q, want %q", tt.text, tt.limit, got, tt.want)
		}
	}
}

func TestEditMessage(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"chat": "team", "message_id": 7, "text": "done", "level": "info"}
	if _, err := argontest.Call[string](t, srv, "editMessage", in); err != nil {
		t.Fatal(err)
	}

	p := api.callsTo("editMessageText")[0].Params
	if p["chat_id"] != -2.0 || p["message_id"] != 7.0 || p["text"] != "ℹ️ done" {
		t.Errorf("got %v", p)
	}

	in["text"] = strings.Repeat("a", maxTextLen+1)
	if _, err := argontest.Call[string](t, srv, "editMessage", in); err == nil {
		t.Error("want error for text over the limit")
	}
}

func TestDeleteMessage(t *testing.T) {
	srv, api := newTestServer(t)

	if _, err := argontest.Call[string](t, srv, "deleteMessage", map[string]any{"message_id": 7}); err != nil {
		t.Fatal(err)
	}

	p := api.callsTo("deleteMessage")[0].Params
	if p["chat_id"] != 1.0 || p["message_id"] != 7.0 {
		t.Errorf("got %v", p)
	}
}

func TestPinMessage(t *testing.T) {
	srv, api := newTestServer(t)

	in := map[string]any{"chat": "team", "message_id": 7, "silent": true}
	if _, err := argontest.Call[string](t, srv, "pinMessage", in); err != nil {
		t.Fatal(err)
	}

	p := api.callsTo("pinChatMessage")[0].Params
	if p["chat_id"] != -2.0 || p["message_id"] != 7.0 || p["disable_notification"] != true {
		t.Errorf("got %v", p)
	}
}

func TestSendDocument(t *testing.T) {
	srv, api := newTestServer(t)

	path := filepath.Join(t.TempDir(), "build.log")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	in := map[string]any{"path": path, "caption": "build", "level": "warn", "silent": true}
	out, err := argontest.Call[sendDocumentOut](t, srv, "sendDocument", in)
	if err != nil {
		t.Fatal(err)
	}
	if out.MessageID == 0 {
		t.Error("got zero message ID")
	}

	c := api.callsTo("sendDocument")[0]
	if c.Files["document"] != "ok" || c.Params["caption"] != "⚠️ build" ||
		c.Params["chat_id"] != "1" || c.Params["disable_notification"] != "true" {
		t.Errorf("got %+v", c)
	}
}

func TestSendDocumentContent(t *testing.T) {
	srv, api := newTestServer(t)

	if _, err := argontest.Call[sendDocumentOut](t, srv, "sendDocument", map[string]any{"content": "report"}); err != nil {
		t.Fatal(err)
	}
	if got := api.callsTo("sendDocument")[0].Files["document"]; got != "report" {
		t.Errorf("got %q", got)
	}

	both := map[string]any{"content": "report", "path": "x"}
	if _, err := argontest.Call[sendDocumentOut](t, srv, "sendDocument", both); err == nil {
		t.Error("want error when both path and content are set")
	}
}

func TestSendMessageDelayed(t *testing.T) {
	srv, api := newTestServer(t)

	out, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"text": "later", "delay_seconds": 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.ScheduledAt == "" || len(out.MessageIDs) != 0 {
		t.Errorf("got %+v, want a scheduled message", out)
	}
	if n := len(api.callsTo("sendMessage")); n != 0 {
		t.Fatalf("sent %d messages before the delay", n)
	}

	deadline := time.Now().Add(5 * time.Second)
	for len(api.callsTo("sendMessage")) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("scheduled message was not sent")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestSendTime(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

	at, err := sendTime(sendMessageIn{At: "2026-10-24T18:00:00+05:00"}, now)
	if err != nil || !at.Equal(now.Add(time.Hour)) {
		t.Errorf("got %v, %v", at, err)
	}
	if at, err := sendTime(sendMessageIn{}, now); err != nil || !at.IsZero() {
		t.Errorf("got %v, %v, want immediate send", at, err)
	}

	for _, in := range []sendMessageIn{
		{Delay: 1, At: "2026-10-24T13:00:00Z"},
		{Delay: -1},
		{At: "tomorrow"},
		{At: "2026-10-24T11:00:00Z"},
		{Delay: int(maxDelay/time.Second) + 1},
	} {
		if _, err := sendTime(in, now); err == nil {
			t.Errorf("sendTime(%+v): want error", in)
		}
	}
}

func TestAskConfirmation(t *testing.T) {
	srv, api := newTestServer(t)
	api.reply = func(c apiCall) string {
		if c.Method != "getUpdates" {
			return ""
		}
		// The question is the first call, so it gets message ID 101. The first
		// update presses a button of some older message and must be ignored.
		return `{"ok":true,"result":[
			{"update_id":5,"callback_query":{"id":"old","from":{"first_name":"Ann"},
				"message":{"message_id":50,"chat":{"id":1}},"data":"0"}},
			{"update_id":6,"callback_query":{"id":"new","from":{"first_name":"Ann","username":"ann"},
				"message":{"message_id":101,"chat":{"id":1}},"data":"1"}}
		]}`
	}

	in := map[string]any{"question": "Deploy?", "options": []string{"Go", "Wait"}}
	out, err := argontest.Call[askConfirmationOut](t, srv, "askConfirmation", in)
	if err != nil {
		t.Fatal(err)
	}

	want := askConfirmationOut{Answered: true, Choice: "Wait", Index: 1, User: "@ann"}
	if out != want {
		t.Errorf("got %+v, want %+v", out, want)
	}

	q := api.callsTo("sendMessage")[0].Params
	if _, ok := q["reply_markup"]; !ok {
		t.Error("question was sent without buttons")
	}
	if answers := api.callsTo("answerCallbackQuery"); len(answers) != 2 || answers[0].Params["text"] == nil {
		t.Errorf("got callback answers %+v, want the old press answered as expired", answers)
	}
	if n := len(api.callsTo("editMessageReplyMarkup")); n != 1 {
		t.Errorf("got %d keyboard removals, want 1", n)
	}
}

func TestAskConfirmationTimeout(t *testing.T) {
	srv, api := newTestServer(t)
	api.reply = func(c apiCall) string {
		if c.Method != "getUpdates" {
			return ""
		}
		time.Sleep(100 * time.Millisecond)
		return `{"ok":true,"result":[]}`
	}

	in := map[string]any{"question": "Deploy?", "timeout_seconds": 1}
	out, err := argontest.Call[askConfirmationOut](t, srv, "askConfirmation", in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Answered || out.Index != -1 {
		t.Errorf("got %+v, want no answer", out)
	}
	if n := len(api.callsTo("editMessageReplyMarkup")); n != 1 {
		t.Errorf("got %d keyboard removals, want 1", n)
	}
}

func TestConfirmOptions(t *testing.T) {
	if got, err := confirmOptions(nil); err != nil || len(got) != 2 {
		t.Errorf("got %v, %v, want Yes and No", got, err)
	}
	for _, bad := range [][]string{{"one"}, {"a", " "}, make([]string, 9)} {
		if _, err := confirmOptions(bad); err == nil {
			t.Errorf("confirmOptions(%q): want error", bad)
		}
	}
}

func TestSendMessageUnknownChat(t *testing.T) {
	srv, _ := newTestServer(t)

	_, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"chat": "nope", "text": "hi"})
	if err == nil || !strings.Contains(err.Error(), "unknown chat") {
		t.Errorf("got %v, want unknown chat error", err)
	}
}

func TestRetryFloodControl(t *testing.T) {
	srv, api := newTestServer(t)
	attempts := 0
	api.reply = func(apiCall) string {
		if attempts++; attempts == 1 {
			return `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`
		}
		return ""
	}

	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if n := len(api.callsTo("sendMessage")); n != 2 {
		t.Errorf("got %d attempts, want 2", n)
	}
}

func TestNoRetryOnBadRequest(t *testing.T) {
	srv, api := newTestServer(t)
	api.reply = func(apiCall) string {
		return `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`
	}

	if _, err := argontest.Call[sendMessageOut](t, srv, "sendMessage", map[string]any{"text": "hi"}); err == nil {
		t.Fatal("want error")
	}
	if n := len(api.callsTo("sendMessage")); n != 1 {
		t.Errorf("got %d attempts, want 1", n)
	}
}

func TestParseChats(t *testing.T) {
	got, err := parseChats(" me:1 , team:-2,")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (chat{"me", 1}) || got[1] != (chat{"team", -2}) {
		t.Errorf("got %v", got)
	}

	for _, bad := range []string{"me", ":1", "me:x"} {
		if _, err := parseChats(bad); err == nil {
			t.Errorf("parseChats(%q): want error", bad)
		}
	}
}

func TestCheck(t *testing.T) {
	_, url := newFakeAPI(t)
	name, err := newBot("token", testChats, lithium.WithBaseURL(url)).check(context.Background())
	if err != nil || name != "test_bot" {
		t.Errorf("got %q, %v", name, err)
	}
}
