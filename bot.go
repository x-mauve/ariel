package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"go.xchunk.org/lithium"
)

// chat is a named destination for messages.
type chat struct {
	Name string `json:"name"`
	ID   int64  `json:"id"`
}

// bot sends Bot API requests on behalf of the configured Telegram bot.
type bot struct {
	client *lithium.Client
	// chats lists the known destinations; the first one is the default.
	chats []chat
}

// newBot creates a bot that talks to the Bot API with token and delivers
// messages to chats. chats must not be empty; its first entry is the default.
func newBot(token string, chats []chat, opts ...lithium.Option) *bot {
	return &bot{client: lithium.New(token, opts...), chats: chats}
}

// chatID resolves a chat name to its ID. An empty name selects the default chat.
func (b *bot) chatID(name string) (int64, error) {
	if name == "" {
		return b.chats[0].ID, nil
	}

	for _, c := range b.chats {
		if c.Name == name {
			return c.ID, nil
		}
	}

	return 0, fmt.Errorf("unknown chat %q, call listChats for the available ones", name)
}

// send sends req and decodes the result into Resp. Each attempt is bounded by
// requestTimeout, and transient failures are retried.
func (b *bot) send[Resp any](ctx context.Context, req lithium.Method) (Resp, error) {
	return retry(ctx, func(ctx context.Context) (Resp, error) {
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		return b.client.Send[Resp](ctx, req)
	})
}

// upload sends req as multipart/form-data with data attached as file field and
// decodes the result into Resp. Like send, it retries transient failures;
// each attempt is bounded by uploadTimeout.
func (b *bot) upload[Resp any](ctx context.Context, req lithium.Method, field, name string, data []byte) (Resp, error) {
	return retry(ctx, func(ctx context.Context) (Resp, error) {
		ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
		defer cancel()

		file := lithium.File{Field: field, Name: name, Reader: bytes.NewReader(data)}
		return b.client.Upload[Resp](ctx, req, file)
	})
}

const (
	// requestTimeout bounds a single Bot API request.
	requestTimeout = 10 * time.Second
	// uploadTimeout bounds a single Bot API request that uploads a file.
	uploadTimeout = time.Minute
	// maxAttempts is how many times a request is tried before giving up.
	maxAttempts = 3
)

// retry calls fn until it succeeds, fails with a permanent error, runs out of
// attempts, or ctx is done. The last error is returned as is.
func retry[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	for attempt := 1; ; attempt++ {
		v, err := fn(ctx)
		if err == nil || attempt == maxAttempts {
			return v, err
		}

		wait, ok := retryDelay(err, attempt)
		if !ok {
			return v, err
		}

		select {
		case <-ctx.Done():
			return v, err
		case <-time.After(wait):
		}
	}
}

// retryDelay reports whether err is transient and how long to wait before the
// next attempt. Flood control (429) is honored through retry_after, server
// errors and network failures back off linearly. Other errors, e.g. a bad
// request or an undecodable result, are permanent: repeating them would fail
// again or deliver a duplicate message.
func retryDelay(err error, attempt int) (time.Duration, bool) {
	backoff := time.Duration(attempt) * time.Second

	var apiErr *lithium.Error
	if errors.As(err, &apiErr) {
		if apiErr.RetryAfter > 0 {
			return time.Duration(apiErr.RetryAfter) * time.Second, true
		}

		return backoff, apiErr.Code >= 500
	}

	var netErr net.Error
	return backoff, errors.As(err, &netErr)
}

// sendMessageReq is the payload of the Telegram Bot API sendMessage method.
type sendMessageReq struct {
	ChatID              int64  `json:"chat_id"`
	Text                string `json:"text"`
	ParseMode           string `json:"parse_mode,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// Method returns the Bot API method name for the request.
func (sendMessageReq) Method() string {
	return "sendMessage"
}

// editMessageTextReq is the payload of the Bot API editMessageText method.
type editMessageTextReq struct {
	ChatID    int64  `json:"chat_id"`
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// Method returns the Bot API method name for the request.
func (editMessageTextReq) Method() string {
	return "editMessageText"
}

// deleteMessageReq is the payload of the Bot API deleteMessage method.
type deleteMessageReq struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

// Method returns the Bot API method name for the request.
func (deleteMessageReq) Method() string {
	return "deleteMessage"
}

// pinChatMessageReq is the payload of the Bot API pinChatMessage method.
type pinChatMessageReq struct {
	ChatID              int64 `json:"chat_id"`
	MessageID           int64 `json:"message_id"`
	DisableNotification bool  `json:"disable_notification,omitempty"`
}

// Method returns the Bot API method name for the request.
func (pinChatMessageReq) Method() string {
	return "pinChatMessage"
}

// sendDocumentReq is the payload of the Bot API sendDocument method. The
// document itself is attached as a file.
type sendDocumentReq struct {
	ChatID              int64  `json:"chat_id"`
	Caption             string `json:"caption,omitempty"`
	ParseMode           string `json:"parse_mode,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// Method returns the Bot API method name for the request.
func (sendDocumentReq) Method() string {
	return "sendDocument"
}

// getMeReq is the payload of the Bot API getMe method.
type getMeReq struct{}

// Method returns the Bot API method name for the request.
func (getMeReq) Method() string {
	return "getMe"
}

// user is the subset of the Bot API getMe result used by the server.
type user struct {
	Username string `json:"username,omitempty"`
}

// check verifies the bot token by calling getMe and returns the bot username.
func (b *bot) check(ctx context.Context) (string, error) {
	me, err := b.send[user](ctx, getMeReq{})
	if err != nil {
		return "", fmt.Errorf("check bot token: %w", err)
	}

	return me.Username, nil
}

// message is the subset of the Bot API Message object used by the server.
type message struct {
	MessageID int64 `json:"message_id"`
}

// checkParseMode reports an error if mode is not a Bot API parse mode.
// An empty mode sends the text as is.
func checkParseMode(mode string) error {
	switch mode {
	case "", "MarkdownV2", "HTML":
		return nil
	default:
		return fmt.Errorf("unsupported parse mode %q, want MarkdownV2 or HTML", mode)
	}
}

// maxCaptionLen is the Bot API limit on caption length, in UTF-16 code units.
const maxCaptionLen = 1024

// maxUploadSize is the Bot API limit on the size of an uploaded file.
const maxUploadSize = 50 << 20

// maxTextLen is the Bot API limit on message text length, in UTF-16 code units.
const maxTextLen = 4096

// splitText splits text into chunks of at most limit UTF-16 code units each.
// It prefers to break after a newline, then after a space, and cuts mid-word
// only when a chunk has neither.
func splitText(text string, limit int) []string {
	var chunks []string
	for utf16Len(text) > limit {
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

// utf16Len returns the length of s in UTF-16 code units, as Telegram counts it.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}

	return n
}
