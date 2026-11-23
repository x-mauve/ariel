// Package fakeapi provides an in-memory Telegram Bot API server for tests.
package fakeapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Call is a Bot API request received by a Server.
type Call struct {
	// Method is the Bot API method name.
	Method string
	// Params holds the decoded request parameters. JSON numbers decode as
	// float64; multipart form values stay strings.
	Params map[string]any
	// Files maps multipart file fields to their contents.
	Files map[string]string
}

// Server is an in-memory Bot API server. By default every call succeeds with
// a message carrying a fresh message_id, or with true for the methods in
// boolMethods. Reply overrides that per call.
type Server struct {
	// Reply returns the raw JSON envelope for a call, or "" for the default.
	// Set it before the first request; it may be called concurrently.
	Reply func(c Call) string

	mu     sync.Mutex
	calls  []Call
	nextID int64
}

// New starts a fake Bot API server that is closed when the test ends. It
// returns the server and its base URL, to be passed to lithium.WithBaseURL.
func New(t testing.TB) (*Server, string) {
	f := &Server{nextID: 100}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)

	return f, srv.URL
}

// boolMethods are the Bot API methods whose result is true rather than an object.
var boolMethods = map[string]bool{
	"deleteMessage": true,
}

func (f *Server) serve(w http.ResponseWriter, r *http.Request) {
	c := Call{
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
			file, err := fh[0].Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
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
	f.mu.Unlock()

	body := ""
	if f.Reply != nil {
		body = f.Reply(c)
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

// CallsTo returns the received calls of method, in order.
func (f *Server) CallsTo(method string) []Call {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []Call
	for _, c := range f.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}

	return out
}
