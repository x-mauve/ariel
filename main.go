// Command argontest runs an MCP server that exposes a single tool for
// sending messages to a fixed chat through a Telegram bot.
//
// Configuration is read from the environment:
//
//	TELEGRAM_BOT_TOKEN  bot token issued by @BotFather
//	TELEGRAM_CHAT_ID    ID of the chat that receives messages
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"go.xchunk.org/argon"
	"go.xchunk.org/lithium"
)

// sendMessageReq is the payload of the Telegram Bot API sendMessage method.
type sendMessageReq struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

// Method returns the Bot API method name for the request.
func (sendMessageReq) Method() string {
	return "sendMessage"
}

// sendMessageIn is the input schema of the sendMessage tool.
type sendMessageIn struct {
	Text string `json:"text" jsonschema:"Message text"`
}

// config holds the settings loaded from the environment.
type config struct {
	token  string
	chatID int64
}

// loadConfig reads and validates the configuration from the environment.
func loadConfig() (config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}

	chatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
	if err != nil {
		return config{}, fmt.Errorf("invalid TELEGRAM_CHAT_ID: %w", err)
	}

	return config{token: token, chatID: chatID}, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	bot := lithium.New(cfg.token)
	srv := argon.New("sender")

	argon.Tool(srv,
		"sendMessage",
		"Send message via Telegram bot",
		func(ctx context.Context, in sendMessageIn) (string, error) {
			req := sendMessageReq{ChatID: cfg.chatID, Text: in.Text}
			if _, err := bot.Send[struct{}](ctx, req); err != nil {
				return "", fmt.Errorf("send message: %w", err)
			}

			return "Sent!", nil
		},
		argon.OpenWorld(),
	)

	if err := srv.Run(); err != nil {
		log.Fatalln(err)
	}
}
