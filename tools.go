package main

import (
	"context"
	"fmt"

	"go.xchunk.org/argon"
)

// sendMessageIn is the input schema of the sendMessage tool.
type sendMessageIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	Text      string `json:"text" jsonschema:"Message text"`
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"Text formatting: MarkdownV2 or HTML; plain text if empty"`
	Silent    bool   `json:"silent,omitempty" jsonschema:"Deliver without sound, for non-urgent notifications"`
}

// chatInfo describes a chat in the listChats output.
type chatInfo struct {
	chat
	Default bool `json:"default"`
}

// listChatsOut is the output of the listChats tool.
type listChatsOut struct {
	Chats []chatInfo `json:"chats"`
}

// registerTools registers all MCP tools of the server.
func registerTools(srv *argon.Server, b *bot) {
	argon.Tool(srv,
		"listChats",
		"List chats the bot can send messages to",
		func(ctx context.Context, _ struct{}) (listChatsOut, error) {
			out := listChatsOut{Chats: make([]chatInfo, len(b.chats))}
			for i, c := range b.chats {
				out.Chats[i] = chatInfo{chat: c, Default: i == 0}
			}

			return out, nil
		},
		argon.ReadOnly(),
	)

	argon.Tool(srv,
		"sendMessage",
		"Send message via Telegram bot",
		func(ctx context.Context, in sendMessageIn) (string, error) {
			chatID, err := b.chatID(in.Chat)
			if err != nil {
				return "", err
			}
			if err := checkParseMode(in.ParseMode); err != nil {
				return "", err
			}

			req := sendMessageReq{
				ChatID:              chatID,
				Text:                in.Text,
				ParseMode:           in.ParseMode,
				DisableNotification: in.Silent,
			}
			if _, err := b.send[struct{}](ctx, req); err != nil {
				return "", fmt.Errorf("send message: %w", err)
			}

			return "Sent!", nil
		},
		argon.OpenWorld(),
	)
}
