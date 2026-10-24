// Command argontest runs an MCP server that exposes tools for sending
// notifications to a Telegram chat through a bot.
//
// Configuration is read from the environment:
//
//	TELEGRAM_BOT_TOKEN  bot token issued by @BotFather
//	TELEGRAM_CHAT_ID    ID of the chat that receives messages
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"go.xchunk.org/argon"
)

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

	srv := argon.New("sender")
	registerTools(srv, newBot(cfg.token, cfg.chatID))

	if err := srv.Run(); err != nil {
		log.Fatalln(err)
	}
}
