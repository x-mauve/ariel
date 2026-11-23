# Ariel

An MCP server that sends notifications to Telegram through a bot.

## Configuration

| Variable | Description |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Bot token issued by @BotFather. Checked with `getMe` on startup. |
| `TELEGRAM_CHATS` | Named chats, e.g. `me:123,team:-456`. |

At least one chat is required. The first entry of `TELEGRAM_CHATS` is the
default chat. Every tool takes an optional
`chat` name and uses the default chat without it.

## Tools

| Tool | Description |
|---|---|
| `listChats` | List the configured chats. |
| `sendMessage` | Send a message. Long text is split into several messages. |
| `editMessage` | Replace the text of a sent message, e.g. "⏳ building…" → "✅ done". |
| `deleteMessage` | Delete a sent message. |
| `sendDocument` | Send a local file or text content as a document. |

Text tools accept `parse_mode` (`MarkdownV2` or `HTML`) and `silent` to
deliver without sound.

Bot API requests time out after 10 seconds and are retried on flood control,
server and network errors.

## Limitations

- Splitting formatted text can break markup across messages.
