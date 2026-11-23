package tools

import (
	"context"
	"fmt"

	"go.xchunk.org/argon"

	"go.mauve.xchunk.org/ariel/internal/telegram"
)

// richContent is the part of a tool input that describes rich message content.
// It is embedded into the input schemas of the tools that send rich messages.
type richContent struct {
	Markdown            string `json:"markdown,omitempty" jsonschema:"Message content in Telegram rich Markdown: GitHub Flavored Markdown with headings, tables, task lists, ==marked==, ||spoiler||, $LaTeX$ and <details>; set either markdown or html"`
	HTML                string `json:"html,omitempty" jsonschema:"Message content in Telegram rich HTML, for features Markdown lacks: buttons, compact or striped tables, cell alignment; set either markdown or html"`
	SkipEntityDetection bool   `json:"skip_entity_detection,omitempty" jsonschema:"Do not turn bare URLs, emails, @mentions, #hashtags and /commands into links"`
}

// set reports whether any content field is set.
func (c richContent) set() bool {
	return c.Markdown != "" || c.HTML != ""
}

// message validates c and converts it to a Bot API InputRichMessage.
func (c richContent) message() (telegram.InputRichMessage, error) {
	msg := telegram.InputRichMessage{
		Markdown:            c.Markdown,
		HTML:                c.HTML,
		SkipEntityDetection: c.SkipEntityDetection,
	}

	switch {
	case c.Markdown != "" && c.HTML != "":
		return msg, fmt.Errorf("set either markdown or html, not both")
	case c.Markdown != "":
		return msg, checkRich("markdown", c.Markdown)
	case c.HTML != "":
		return msg, checkRich("html", c.HTML)
	default:
		return msg, fmt.Errorf("set markdown or html with the message content")
	}
}

// sendRichMessageIn is the input schema of the sendRichMessage tool.
type sendRichMessageIn struct {
	Chat string `json:"chat,omitempty" jsonschema:"Chat name from listChats; the default chat if empty"`
	richContent
	Silent         bool `json:"silent,omitempty" jsonschema:"Deliver without sound, for non-urgent messages"`
	ProtectContent bool `json:"protect_content,omitempty" jsonschema:"Prevent forwarding and saving of the message"`
}

// sendRichMessageOut is the output of the sendRichMessage tool.
type sendRichMessageOut struct {
	MessageID int64 `json:"message_id" jsonschema:"ID of the sent message, for editMessage and deleteMessage"`
}

func registerSendRichMessage(srv *argon.Server, b *telegram.Bot) {
	argon.Tool(srv,
		"sendRichMessage",
		"Send a rich message via the Telegram bot: a formatted document with headings, "+
			"tables, lists, quotes, code blocks, formulas and collapsible sections, "+
			"up to 32768 characters in a single message. Read the "+guideURI+" resource "+
			"before the first call: it covers structure, syntax, limits and examples",
		func(ctx context.Context, in sendRichMessageIn) (sendRichMessageOut, error) {
			chatID, err := b.ChatID(in.Chat)
			if err != nil {
				return sendRichMessageOut{}, err
			}
			rich, err := in.message()
			if err != nil {
				return sendRichMessageOut{}, err
			}

			req := telegram.SendRichMessage{
				ChatID:              chatID,
				RichMessage:         rich,
				DisableNotification: in.Silent,
				ProtectContent:      in.ProtectContent,
			}
			msg, err := b.Send[telegram.Message](ctx, req)
			if err != nil {
				return sendRichMessageOut{}, withMarkupHint("send rich message", err)
			}

			return sendRichMessageOut{MessageID: msg.MessageID}, nil
		},
		argon.OpenWorld(),
	)
}
