// Package tools registers the MCP tools of the server on an argon.Server.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.xchunk.org/argon"

	"go.xchunk.org/ariel/internal/telegram"
)

// Register registers all MCP tools of the server on srv. The tools deliver
// messages through b.
func Register(srv *argon.Server, b *telegram.Bot) {
	registerListChats(srv, b)
	registerSendRichMessage(srv, b)
	registerSendMessage(srv, b)
	registerEditMessage(srv, b)
	registerDeleteMessage(srv, b)
	registerSendDocument(srv, b)
}

// chatInfo describes a chat in the listChats output.
type chatInfo struct {
	telegram.Chat
	Default bool `json:"default"`
}

// listChatsOut is the output of the listChats tool.
type listChatsOut struct {
	Chats []chatInfo `json:"chats"`
}

func registerListChats(srv *argon.Server, b *telegram.Bot) {
	argon.Tool(srv,
		"listChats",
		"List chats the bot can send messages to",
		func(ctx context.Context, _ struct{}) (listChatsOut, error) {
			chats := b.Chats()
			out := listChatsOut{Chats: make([]chatInfo, len(chats))}
			for i, c := range chats {
				out.Chats[i] = chatInfo{Chat: c, Default: i == 0}
			}

			return out, nil
		},
		argon.ReadOnly(),
	)
}

// sendMessageIn is the input schema of the sendMessage tool.
type sendMessageIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	Text      string `json:"text" jsonschema:"Message text; text over 4096 characters is split into several messages"`
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"Text formatting: MarkdownV2 or HTML; plain text if empty"`
	Silent    bool   `json:"silent,omitempty" jsonschema:"Deliver without sound, for non-urgent notifications"`
}

// sendMessageOut is the output of the sendMessage tool.
type sendMessageOut struct {
	MessageIDs []int64 `json:"message_ids" jsonschema:"IDs of the sent messages, one per part of a split text"`
}

func registerSendMessage(srv *argon.Server, b *telegram.Bot) {
	argon.Tool(srv,
		"sendMessage",
		"Send a plain text message via the Telegram bot",
		func(ctx context.Context, in sendMessageIn) (sendMessageOut, error) {
			chatID, err := b.ChatID(in.Chat)
			if err != nil {
				return sendMessageOut{}, err
			}
			if err := telegram.CheckParseMode(in.ParseMode); err != nil {
				return sendMessageOut{}, err
			}

			chunks := telegram.SplitText(in.Text, telegram.MaxTextLen)
			ids, err := sendText(ctx, b, chatID, chunks, in.ParseMode, in.Silent)
			return sendMessageOut{MessageIDs: ids}, err
		},
		argon.OpenWorld(),
	)
}

// sendText sends chunks as consecutive messages and returns their IDs. On
// failure it returns the IDs of the parts sent so far.
func sendText(ctx context.Context, b *telegram.Bot, chatID int64, chunks []string, parseMode string, silent bool) ([]int64, error) {
	ids := make([]int64, 0, len(chunks))
	for i, chunk := range chunks {
		req := telegram.SendMessage{
			ChatID:              chatID,
			Text:                chunk,
			ParseMode:           parseMode,
			DisableNotification: silent,
		}
		msg, err := b.Send[telegram.Message](ctx, req)
		if err != nil {
			return ids, fmt.Errorf("send message part %d of %d: %w", i+1, len(chunks), err)
		}
		ids = append(ids, msg.MessageID)
	}

	return ids, nil
}

// editMessageIn is the input schema of the editMessage tool.
type editMessageIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	MessageID int64  `json:"message_id" jsonschema:"ID of the message to edit, as returned by sendMessage"`
	Text      string `json:"text" jsonschema:"New message text, at most 4096 characters"`
	ParseMode string `json:"parse_mode,omitempty" jsonschema:"Text formatting: MarkdownV2 or HTML; plain text if empty"`
}

func registerEditMessage(srv *argon.Server, b *telegram.Bot) {
	argon.Tool(srv,
		"editMessage",
		"Replace the text of a sent message",
		func(ctx context.Context, in editMessageIn) (string, error) {
			chatID, err := b.ChatID(in.Chat)
			if err != nil {
				return "", err
			}
			if err := telegram.CheckParseMode(in.ParseMode); err != nil {
				return "", err
			}
			if n := telegram.UTF16Len(in.Text); n > telegram.MaxTextLen {
				return "", fmt.Errorf("text is %d characters long, the limit is %d", n, telegram.MaxTextLen)
			}

			req := telegram.EditMessageText{
				ChatID:    chatID,
				MessageID: in.MessageID,
				Text:      in.Text,
				ParseMode: in.ParseMode,
			}
			// The result is the edited message, which the tool does not need.
			if _, err := b.Send[json.RawMessage](ctx, req); err != nil {
				return "", fmt.Errorf("edit message: %w", err)
			}

			return "Edited", nil
		},
		argon.Idempotent(),
		argon.OpenWorld(),
	)
}

// messageRefIn is the input schema of tools that act on a sent message.
type messageRefIn struct {
	Chat      string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	MessageID int64  `json:"message_id" jsonschema:"ID of the message, as returned by the tool that sent it"`
}

func registerDeleteMessage(srv *argon.Server, b *telegram.Bot) {
	argon.Tool(srv,
		"deleteMessage",
		"Delete a message sent by the bot",
		func(ctx context.Context, in messageRefIn) (string, error) {
			chatID, err := b.ChatID(in.Chat)
			if err != nil {
				return "", err
			}

			req := telegram.DeleteMessage{ChatID: chatID, MessageID: in.MessageID}
			if _, err := b.Send[bool](ctx, req); err != nil {
				return "", fmt.Errorf("delete message: %w", err)
			}

			return "Deleted", nil
		},
		argon.Destructive(),
		argon.OpenWorld(),
	)
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
}

// sendDocumentOut is the output of the sendDocument tool.
type sendDocumentOut struct {
	MessageID int64 `json:"message_id" jsonschema:"ID of the sent message"`
}

func registerSendDocument(srv *argon.Server, b *telegram.Bot) {
	argon.Tool(srv,
		"sendDocument",
		"Send a local file or text content as a document, e.g. a log or a report",
		func(ctx context.Context, in sendDocumentIn) (sendDocumentOut, error) {
			chatID, err := b.ChatID(in.Chat)
			if err != nil {
				return sendDocumentOut{}, err
			}
			if err := telegram.CheckParseMode(in.ParseMode); err != nil {
				return sendDocumentOut{}, err
			}
			if n := telegram.UTF16Len(in.Caption); n > telegram.MaxCaptionLen {
				return sendDocumentOut{}, fmt.Errorf("caption is %d characters long, the limit is %d", n, telegram.MaxCaptionLen)
			}

			name, data, err := documentFile(in)
			if err != nil {
				return sendDocumentOut{}, err
			}

			req := telegram.SendDocument{
				ChatID:              chatID,
				Caption:             in.Caption,
				ParseMode:           in.ParseMode,
				DisableNotification: in.Silent,
			}
			msg, err := b.Upload[telegram.Message](ctx, req, "document", name, data)
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
