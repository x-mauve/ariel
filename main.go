package main

import (
	"context"
	"log"
	"os"

	"go.xchunk.org/argon"
	"go.xchunk.org/lithium"
)

type SendMessageReq struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func (SendMessageReq) Method() string {
	return "sendMessage"
}

func main() {
	c := lithium.New(os.Getenv("TELEGRAM_BOT_TOKEN"))

	srv := argon.New("sender")

	argon.Tool(srv,
		"sendMessage",
		"Send message via Telegram bot",
		func(ctx context.Context, in struct {
			Text string `json:"text" jsonschema:"Message text"`
		}) (string, error) {
			if _, err := c.Send[struct{}](ctx, SendMessageReq{
				ChatID: 1256738876,
				Text:   in.Text,
			}); err != nil {
				return err.Error(), err
			}

			return "Sent!", nil
		},
		argon.OpenWorld(),
	)

	if err := srv.Run(); err != nil {
		log.Fatalln(err)
	}
}
