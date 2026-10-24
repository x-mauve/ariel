package main

import (
	"context"

	"go.xchunk.org/lithium"
)

// bot sends Bot API requests on behalf of the configured Telegram bot.
type bot struct {
	client *lithium.Client
	chatID int64
}

// newBot creates a bot that talks to the Bot API with token and delivers
// messages to chatID.
func newBot(token string, chatID int64) *bot {
	return &bot{client: lithium.New(token), chatID: chatID}
}

// send sends req and decodes the result into Resp.
func (b *bot) send[Resp any](ctx context.Context, req lithium.Method) (Resp, error) {
	return b.client.Send[Resp](ctx, req)
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
