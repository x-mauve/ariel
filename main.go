// Command argontest runs an MCP server that exposes tools for sending
// notifications to a Telegram chat through a bot.
//
// Configuration is read from the environment:
//
//	TELEGRAM_BOT_TOKEN  bot token issued by @BotFather
//	TELEGRAM_CHAT_ID    ID of the default chat, registered as "default"
//	TELEGRAM_CHATS      optional named chats, e.g. "me:123,team:-456"
//
// At least one chat must be configured. The default chat is TELEGRAM_CHAT_ID
// if set, otherwise the first entry of TELEGRAM_CHATS.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"go.xchunk.org/argon"
)

// startupTimeout bounds the token check performed on startup.
const startupTimeout = 30 * time.Second

// config holds the settings loaded from the environment.
type config struct {
	token string
	chats []chat
}

// loadConfig reads and validates the configuration from the environment.
func loadConfig() (config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}

	var chats []chat
	if v := os.Getenv("TELEGRAM_CHAT_ID"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return config{}, fmt.Errorf("invalid TELEGRAM_CHAT_ID: %w", err)
		}
		chats = append(chats, chat{Name: "default", ID: id})
	}

	named, err := parseChats(os.Getenv("TELEGRAM_CHATS"))
	if err != nil {
		return config{}, fmt.Errorf("invalid TELEGRAM_CHATS: %w", err)
	}
	chats = append(chats, named...)

	if len(chats) == 0 {
		return config{}, fmt.Errorf("neither TELEGRAM_CHAT_ID nor TELEGRAM_CHATS is set")
	}

	seen := make(map[string]bool, len(chats))
	for _, c := range chats {
		if seen[c.Name] {
			return config{}, fmt.Errorf("duplicate chat name %q", c.Name)
		}
		seen[c.Name] = true
	}

	return config{token: token, chats: chats}, nil
}

// parseChats parses a comma-separated list of name:id pairs.
func parseChats(s string) ([]chat, error) {
	var chats []chat
	for entry := range strings.SplitSeq(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		name, rawID, ok := strings.Cut(entry, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("entry %q: want name:id", entry)
		}

		id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("entry %q: %w", entry, err)
		}
		chats = append(chats, chat{Name: name, ID: id})
	}

	return chats, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	b := newBot(cfg.token, cfg.chats)

	// Fail fast on a revoked or mistyped token instead of on the first tool call.
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	name, err := b.check(ctx)
	cancel()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("authorized as @%s", name)

	srv := argon.New("ariel")
	registerTools(srv, b)

	if err := srv.Run(); err != nil {
		log.Fatalln(err)
	}
}
