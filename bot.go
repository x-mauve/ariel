package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

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

const (
	// requestTimeout bounds a single Bot API request.
	requestTimeout = 10 * time.Second
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
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

// Method returns the Bot API method name for the request.
func (sendMessageReq) Method() string {
	return "sendMessage"
}

// getMeReq is the payload of the Bot API getMe method.
type getMeReq struct{}

// Method returns the Bot API method name for the request.
func (getMeReq) Method() string {
	return "getMe"
}

// user is the subset of the Bot API User object used by the server.
type user struct {
	Username string `json:"username"`
}

// check verifies the bot token by calling getMe and returns the bot username.
func (b *bot) check(ctx context.Context) (string, error) {
	me, err := b.send[user](ctx, getMeReq{})
	if err != nil {
		return "", fmt.Errorf("check bot token: %w", err)
	}

	return me.Username, nil
}
