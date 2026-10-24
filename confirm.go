package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

// inlineKeyboardButton is a Bot API InlineKeyboardButton that sends a callback.
type inlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// inlineKeyboardMarkup is a Bot API InlineKeyboardMarkup.
type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard"`
}

// getUpdatesReq is the payload of the Bot API getUpdates method.
type getUpdatesReq struct {
	Offset         int64    `json:"offset,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	AllowedUpdates []string `json:"allowed_updates"`
}

// Method returns the Bot API method name for the request.
func (getUpdatesReq) Method() string {
	return "getUpdates"
}

// update is the subset of the Bot API Update object used by the server.
type update struct {
	UpdateID      int64          `json:"update_id"`
	CallbackQuery *callbackQuery `json:"callback_query"`
}

// callbackQuery is the subset of the Bot API CallbackQuery object used by the server.
type callbackQuery struct {
	ID      string `json:"id"`
	From    user   `json:"from"`
	Message *struct {
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
	Data string `json:"data"`
}

// answerCallbackQueryReq is the payload of the Bot API answerCallbackQuery method.
type answerCallbackQueryReq struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
}

// Method returns the Bot API method name for the request.
func (answerCallbackQueryReq) Method() string {
	return "answerCallbackQuery"
}

// editMessageReplyMarkupReq is the payload of the Bot API editMessageReplyMarkup
// method. A nil ReplyMarkup removes the keyboard.
type editMessageReplyMarkupReq struct {
	ChatID      int64                 `json:"chat_id"`
	MessageID   int64                 `json:"message_id"`
	ReplyMarkup *inlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// Method returns the Bot API method name for the request.
func (editMessageReplyMarkupReq) Method() string {
	return "editMessageReplyMarkup"
}

// pollTimeout is the long polling timeout of a single getUpdates request.
const pollTimeout = 25 * time.Second

// answer is a button press on a confirmation message.
type answer struct {
	// Index is the position of the pressed option.
	Index int
	// User is the person who pressed the button.
	User user
}

// keyboard returns an inline keyboard with one button per option. The callback
// data of a button is the index of its option.
func keyboard(options []string) *inlineKeyboardMarkup {
	rows := make([][]inlineKeyboardButton, len(options))
	for i, opt := range options {
		rows[i] = []inlineKeyboardButton{{Text: opt, CallbackData: strconv.Itoa(i)}}
	}

	return &inlineKeyboardMarkup{InlineKeyboard: rows}
}

// waitAnswer polls for a button press on message msgID in chat chatID until
// one arrives, the deadline passes or ctx is done. It reports false if no
// button was pressed in time.
//
// The Bot API serves updates to a single consumer at a time, so concurrent
// waits are serialized. Presses on other messages, e.g. confirmations that
// already timed out, are answered as expired and dropped.
func (b *bot) waitAnswer(ctx context.Context, chatID, msgID int64, options int, deadline time.Time) (answer, bool, error) {
	b.pollMu.Lock()
	defer b.pollMu.Unlock()

	for {
		left := time.Until(deadline)
		if left <= 0 {
			return answer{}, false, nil
		}

		req := getUpdatesReq{
			Offset:         b.offset,
			Timeout:        int(min(left, pollTimeout) / time.Second),
			AllowedUpdates: []string{"callback_query"},
		}
		updates, err := retry(ctx, func(ctx context.Context) ([]update, error) {
			ctx, cancel := context.WithTimeout(ctx, pollTimeout+requestTimeout)
			defer cancel()

			return b.client.Send[[]update](ctx, req)
		})
		if err != nil {
			return answer{}, false, fmt.Errorf("get updates: %w", err)
		}

		for _, u := range updates {
			b.offset = u.UpdateID + 1

			q := u.CallbackQuery
			if q == nil {
				continue
			}

			index, err := strconv.Atoi(q.Data)
			ours := q.Message != nil && q.Message.MessageID == msgID && q.Message.Chat.ID == chatID &&
				err == nil && index >= 0 && index < options
			if !ours {
				b.answerCallback(ctx, q.ID, "This question is no longer active")
				continue
			}

			b.answerCallback(ctx, q.ID, "")
			return answer{Index: index, User: q.From}, true, nil
		}
	}
}

// answerCallback stops the loading indicator on a pressed button, optionally
// showing text. Failures are only logged: the press itself is already handled.
func (b *bot) answerCallback(ctx context.Context, id, text string) {
	req := answerCallbackQueryReq{CallbackQueryID: id, Text: text}
	if _, err := b.send[bool](ctx, req); err != nil {
		log.Printf("answer callback query: %v", err)
	}
}

// removeKeyboard removes the buttons from a confirmation message so it cannot
// be answered again. Failures are only logged.
func (b *bot) removeKeyboard(chatID, msgID int64) {
	// The tool context may already be canceled, but the cleanup should still happen.
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req := editMessageReplyMarkupReq{ChatID: chatID, MessageID: msgID}
	if _, err := b.send[json.RawMessage](ctx, req); err != nil {
		log.Printf("remove keyboard: %v", err)
	}
}
