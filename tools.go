package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

// editMessageIn is the input schema of the editMessage tool.
type editMessageIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	MessageID int64  `json:"message_id" jsonschema:"ID of the message to edit, as returned by sendMessage"`
	Text      string `json:"text" jsonschema:"New message text, at most 4096 characters"`
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"Text formatting: MarkdownV2 or HTML; plain text if empty"`
	Level     string `json:"level,omitempty" jsonschema:"Severity: info, warn or error; adds an emoji prefix"`
}

// messageRefIn is the input schema of tools that act on a sent message.
type messageRefIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	MessageID int64  `json:"message_id" jsonschema:"ID of the message, as returned by sendMessage"`
}

// pinMessageIn is the input schema of the pinMessage tool.
type pinMessageIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	MessageID int64  `json:"message_id" jsonschema:"ID of the message, as returned by sendMessage"`
	Silent    bool   `json:"silent,omitempty" jsonschema:"Pin without notifying chat members"`
}

// sendDocumentIn is the input schema of the sendDocument tool.
type sendDocumentIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	Path      string `json:"path,omitempty" jsonschema:"Path to a local file to send; set either path or content"`
	Content   string `json:"content,omitempty" jsonschema:"Text content to send as a file, e.g. a log or a report; set either path or content"`
	Filename  string `json:"filename,omitempty" jsonschema:"File name shown in the chat; defaults to the base name of path, or report.txt for content"`
	Caption   string `json:"caption,omitempty" jsonschema:"Caption under the file, at most 1024 characters"`
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"Caption formatting: MarkdownV2 or HTML; plain text if empty"`
	Silent    bool   `json:"silent,omitempty" jsonschema:"Deliver without sound, for non-urgent notifications"`
	Level     string `json:"level,omitempty" jsonschema:"Severity: info, warn or error; adds an emoji prefix to the caption"`
}

// sendDocumentOut is the output of the sendDocument tool.
type sendDocumentOut struct {
	MessageID int64 `json:"message_id" jsonschema:"ID of the sent message"`
}

// documentFile returns the name and contents of the file described by in.
func documentFile(in sendDocumentIn) (string, []byte, error) {
	switch {
	case (in.Path == "") == (in.Content == ""):
		return "", nil, fmt.Errorf("set exactly one of path and content")
	case in.Content != "":
		name := cmp.Or(in.Filename, "report.txt")
		if len(in.Content) > maxUploadSize {
			return "", nil, fmt.Errorf("content exceeds the %d MiB upload limit", maxUploadSize>>20)
		}
		return name, []byte(in.Content), nil
	}

	info, err := os.Stat(in.Path)
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%s is not a regular file", in.Path)
	}
	if info.Size() > maxUploadSize {
		return "", nil, fmt.Errorf("%s exceeds the %d MiB upload limit", in.Path, maxUploadSize>>20)
	}

	data, err := os.ReadFile(in.Path)
	if err != nil {
		return "", nil, err
	}

	return cmp.Or(in.Filename, filepath.Base(in.Path)), data, nil
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

	argon.Tool(srv,
		"editMessage",
		"Replace the text of a sent message, e.g. to update a progress status",
		func(ctx context.Context, in editMessageIn) (string, error) {
			chatID, err := b.chatID(in.Chat)
			if err != nil {
				return "", err
			}
			if err := checkParseMode(in.ParseMode); err != nil {
				return "", err
			}
			text, err := withLevel(in.Level, in.Text)
			if err != nil {
				return "", err
			}
			if n := utf16Len(text); n > maxTextLen {
				return "", fmt.Errorf("text is %d characters long, the limit is %d", n, maxTextLen)
			}

			req := editMessageTextReq{
				ChatID:    chatID,
				MessageID: in.MessageID,
				Text:      text,
				ParseMode: in.ParseMode,
			}
			// The result is the edited message, which the tool does not need.
			if _, err := b.send[json.RawMessage](ctx, req); err != nil {
				return "", fmt.Errorf("edit message: %w", err)
			}

			return "Edited", nil
		},
		argon.Idempotent(),
		argon.OpenWorld(),
	)

	argon.Tool(srv,
		"deleteMessage",
		"Delete a sent message, e.g. an outdated notification",
		func(ctx context.Context, in messageRefIn) (string, error) {
			chatID, err := b.chatID(in.Chat)
			if err != nil {
				return "", err
			}

			req := deleteMessageReq{ChatID: chatID, MessageID: in.MessageID}
			if _, err := b.send[bool](ctx, req); err != nil {
				return "", fmt.Errorf("delete message: %w", err)
			}

			return "Deleted", nil
		},
		argon.Destructive(),
		argon.OpenWorld(),
	)

	argon.Tool(srv,
		"pinMessage",
		"Pin a sent message in its chat, for the most important notifications",
		func(ctx context.Context, in pinMessageIn) (string, error) {
			chatID, err := b.chatID(in.Chat)
			if err != nil {
				return "", err
			}

			req := pinChatMessageReq{
				ChatID:              chatID,
				MessageID:           in.MessageID,
				DisableNotification: in.Silent,
			}
			if _, err := b.send[bool](ctx, req); err != nil {
				return "", fmt.Errorf("pin message: %w", err)
			}

			return "Pinned", nil
		},
		argon.Idempotent(),
		argon.OpenWorld(),
	)

	argon.Tool(srv,
		"sendDocument",
		"Send a local file or text content as a document, e.g. a log or a report",
		func(ctx context.Context, in sendDocumentIn) (sendDocumentOut, error) {
			chatID, err := b.chatID(in.Chat)
			if err != nil {
				return sendDocumentOut{}, err
			}
			if err := checkParseMode(in.ParseMode); err != nil {
				return sendDocumentOut{}, err
			}
			caption, err := withLevel(in.Level, in.Caption)
			if err != nil {
				return sendDocumentOut{}, err
			}
			if n := utf16Len(caption); n > maxCaptionLen {
				return sendDocumentOut{}, fmt.Errorf("caption is %d characters long, the limit is %d", n, maxCaptionLen)
			}

			name, data, err := documentFile(in)
			if err != nil {
				return sendDocumentOut{}, err
			}

			req := sendDocumentReq{
				ChatID:              chatID,
				Caption:             caption,
				ParseMode:           in.ParseMode,
				DisableNotification: in.Silent,
			}
			msg, err := b.upload[message](ctx, req, "document", name, data)
			if err != nil {
				return sendDocumentOut{}, fmt.Errorf("send document: %w", err)
			}

			return sendDocumentOut{MessageID: msg.MessageID}, nil
		},
		argon.OpenWorld(),
		// Uploads of large files with retries take longer than the default limit.
		argon.ToolTimeout(5*time.Minute),
	)
}
