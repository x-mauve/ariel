package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

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
	if body == "" {
		body = fmt.Sprintf(`{"ok":true,"result":{"message_id":%d,"username":"test_bot"}}`, id)
	}
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, body)
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
