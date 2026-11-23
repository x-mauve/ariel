// Package telegram sends Telegram Bot API requests on behalf of a bot through
// go.xchunk.org/lithium. It defines only the subset of the Bot API the server
// uses, and adds per-request timeouts and retries of transient failures.
package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"go.xchunk.org/lithium"
)

// Chat is a named destination for messages.
type Chat struct {
	Name string `json:"name"`
	ID   int64  `json:"id"`
}

// Bot sends Bot API requests on behalf of a Telegram bot. It is safe for
// concurrent use.
type Bot struct {
	client *lithium.Client
	// chats lists the known destinations; the first one is the default.
	chats []Chat
}

// New creates a bot that talks to the Bot API with token and delivers messages
// to chats. chats must not be empty; its first entry is the default chat. opts
// are passed to the underlying lithium client, e.g. to point it at a test server.
func New(token string, chats []Chat, opts ...lithium.Option) *Bot {
	return &Bot{client: lithium.New(token, opts...), chats: chats}
}

// Chats returns the configured chats; the first one is the default. The caller
// must not modify the returned slice.
func (b *Bot) Chats() []Chat {
	return b.chats
}

// ChatID resolves a chat name to its ID. An empty name selects the default
// chat. An unknown name is an error that tells the caller how to find the
// valid ones.
func (b *Bot) ChatID(name string) (int64, error) {
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

// Send sends req as JSON and decodes the result into Resp. Each attempt is
// bounded by requestTimeout, and transient failures are retried. A Bot API
// error is returned as a *lithium.Error.
func (b *Bot) Send[Resp any](ctx context.Context, req lithium.Method) (Resp, error) {
	return retry(ctx, func(ctx context.Context) (Resp, error) {
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		return b.client.Send[Resp](ctx, req)
	})
}

// Upload sends req as multipart/form-data with data attached as the file field
// named field, under the file name name, and decodes the result into Resp.
// Like Send, it retries transient failures; each attempt is bounded by
// uploadTimeout.
func (b *Bot) Upload[Resp any](ctx context.Context, req lithium.Method, field, name string, data []byte) (Resp, error) {
	return retry(ctx, func(ctx context.Context) (Resp, error) {
		ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
		defer cancel()

		file := lithium.File{Field: field, Name: name, Reader: bytes.NewReader(data)}
		return b.client.Upload[Resp](ctx, req, file)
	})
}

// Check verifies the bot token by calling getMe and returns the bot username.
func (b *Bot) Check(ctx context.Context) (string, error) {
	me, err := b.Send[botInfo](ctx, GetMe{})
	if err != nil {
		return "", fmt.Errorf("check bot token: %w", err)
	}

	return me.Username, nil
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
