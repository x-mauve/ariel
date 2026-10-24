package main

import (
	"context"
	"fmt"

	"go.xchunk.org/argon"
)

// sendMessageIn is the input schema of the sendMessage tool.
type sendMessageIn struct {
	Text string `json:"text" jsonschema:"Message text"`
}

// registerTools registers all MCP tools of the server.
func registerTools(srv *argon.Server, b *bot) {
	argon.Tool(srv,
		"sendMessage",
		"Send message via Telegram bot",
		func(ctx context.Context, in sendMessageIn) (string, error) {
			req := sendMessageReq{ChatID: b.chatID, Text: in.Text}
			if _, err := b.send[struct{}](ctx, req); err != nil {
				return "", fmt.Errorf("send message: %w", err)
			}

			return "Sent!", nil
		},
		argon.OpenWorld(),
	)
}
