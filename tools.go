package main

import (
	"context"
	"fmt"

	"go.xchunk.org/argon"
)

// sendMessageIn is the input schema of the sendMessage tool.
type sendMessageIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	Text      string `json:"text" jsonschema:"Message text; text over 4096 characters is split into several messages"`
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"Text formatting: MarkdownV2 or HTML; plain text if empty"`
	Silent    bool   `json:"silent,omitempty" jsonschema:"Deliver without sound, for non-urgent notifications"`
	Level     string `json:"level,omitempty" jsonschema:"Severity: info, warn or error; adds an emoji prefix"`
}

// sendMessageOut is the output of the sendMessage tool.
type sendMessageOut struct {
	MessageIDs []int64 `json:"message_ids" jsonschema:"IDs of the sent messages, one per part of a split text"`
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

// levelPrefixes maps severity levels to the emoji prepended to the text.
var levelPrefixes = map[string]string{
	"info":  "ℹ️ ",
	"warn":  "⚠️ ",
	"error": "🔴 ",
}

// withLevel prepends the emoji of level to text. An empty level leaves text as is.
func withLevel(level, text string) (string, error) {
	if level == "" {
		return text, nil
	}

	prefix, ok := levelPrefixes[level]
	if !ok {
		return "", fmt.Errorf("unsupported level %q, want info, warn or error", level)
	}

	return prefix + text, nil
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
		func(ctx context.Context, in sendMessageIn) (sendMessageOut, error) {
			chatID, err := b.chatID(in.Chat)
			if err != nil {
				return sendMessageOut{}, err
			}
			if err := checkParseMode(in.ParseMode); err != nil {
				return sendMessageOut{}, err
			}
			text, err := withLevel(in.Level, in.Text)
			if err != nil {
				return sendMessageOut{}, err
			}

			chunks := splitText(text, maxTextLen)
			var out sendMessageOut
			for i, chunk := range chunks {
				req := sendMessageReq{
					ChatID:              chatID,
					Text:                chunk,
					ParseMode:           in.ParseMode,
					DisableNotification: in.Silent,
				}
				msg, err := b.send[message](ctx, req)
				if err != nil {
					return out, fmt.Errorf("send message part %d of %d: %w", i+1, len(chunks), err)
				}
				out.MessageIDs = append(out.MessageIDs, msg.MessageID)
			}

			return out, nil
		},
		argon.OpenWorld(),
	)
}
