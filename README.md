# Ariel

[![CI](https://github.com/x-chunk/ariel/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/x-chunk/ariel/actions/workflows/ci.yml)

An MCP server that lets Claude and other MCP clients send Telegram
[rich messages](https://core.telegram.org/bots/api#sendrichmessage) through a bot:
formatted documents with headings, tables, task lists, formulas, collapsible
sections and buttons (Bot API 10.5). It speaks MCP over stdio.

## Installation

Download an archive for your platform from the
[releases](https://github.com/x-chunk/ariel/releases), or build from source with
Go 1.27 or later:

```sh
go install go.xchunk.org/ariel@latest
```

## Configuration

| Variable | Description |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Bot token issued by @BotFather. Checked with `getMe` on startup. |
| `TELEGRAM_CHATS` | Named chats, e.g. `me:123,team:-456`. The first one is the default. |

Every tool takes an optional `chat` name and uses the default chat without it.
The bot must be able to write to each chat: start a private chat with it, or add
it to the group or channel.

### Claude Code

Add the server to `.mcp.json` in your project, or to `~/.claude.json` for all
projects:

```json
{
  "mcpServers": {
    "ariel": {
      "command": "ariel",
      "env": {
        "TELEGRAM_BOT_TOKEN": "123456:ABC...",
        "TELEGRAM_CHATS": "me:123456789"
      }
    }
  }
}
```

## Tools

| Tool | Description |
|---|---|
| `listChats` | List the configured chats. |
| `sendRichMessage` | Send a rich message written in rich Markdown (`markdown`) or HTML (`html`). |
| `editMessage` | Replace the content of a sent message with rich content (`markdown`, `html`) or plain `text`. |
| `deleteMessage` | Delete a sent message. |
| `sendMessage` | Send a plain text message. Text over 4096 characters is split into several messages. |
| `sendDocument` | Send a local file or text content as a document. |

`sendMessage`, `sendDocument` and plain-text edits accept `parse_mode`
(`MarkdownV2` or `HTML`). Sending tools accept `silent` to deliver without sound.

## Teaching the model

The server exposes the `ariel://guide` resource: a guide to writing good rich
messages for phone readers, with the Markdown and HTML syntax Telegram accepts,
the limits and worked examples. Tool descriptions tell the model to read it
before the first rich message. The source is
[`internal/tools/guide.md`](internal/tools/guide.md).

Rich content is also checked before it is sent: its length, table width, media
URLs and button types. Errors say how to fix the content, so the model can
correct itself and retry.

## Behavior

- Bot API requests time out after 10 seconds (one minute for uploads) and are
  retried on flood control, server and network errors.
- Rich messages accept media only by http(s) URL. Send local files with
  `sendDocument`.
- Buttons that send callback queries are rejected: the server does not receive
  updates, so nothing would answer them.
- Some Telegram Desktop versions show rich messages as empty. Use `sendMessage`
  for short messages that must render everywhere.
- Splitting long formatted text in `sendMessage` can break markup across messages.

## Development

```sh
go vet ./...
go test -race ./...
```

Contribution rules for code comments, commits and the changelog are in
[AGENTS.md](AGENTS.md). Releases follow [Semantic Versioning](https://semver.org):
pushing a `vX.Y.Z` tag builds binaries and publishes a GitHub release with the
notes of that version from [CHANGELOG.md](CHANGELOG.md).
