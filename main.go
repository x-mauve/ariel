// Command argontest runs an MCP server that exposes tools for sending
// notifications to a Telegram chat through a bot.
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
	"time"

	"go.xchunk.org/argon"
)

// startupTimeout bounds the token check performed on startup.
const startupTimeout = 30 * time.Second

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

	b := newBot(cfg.token, cfg.chatID)

	// Fail fast on a revoked or mistyped token instead of on the first tool call.
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	name, err := b.check(ctx)
	cancel()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("authorized as @%s", name)

	srv := argon.New("sender")
	registerTools(srv, b)

	if err := srv.Run(); err != nil {
		log.Fatalln(err)
	}
}
