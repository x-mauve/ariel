# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-11-23

### Added

- `sendRichMessage` tool that sends Telegram rich messages (Bot API 10.5) written in
  rich Markdown or HTML: headings, tables, task lists, formulas, collapsible sections
  and buttons.
- `editMessage` can replace a message with new rich content.
- `ariel://guide` resource that teaches the model to structure rich messages, with
  the supported syntax, limits and examples.
- Rich content is checked before sending: length, table width, media URLs and button
  types. Errors explain how to fix the content.
- The server reports its version to MCP clients.

### Changed

- The first entry of `TELEGRAM_CHATS` is the default chat.

### Removed

- `TELEGRAM_CHAT_ID`; list every chat in `TELEGRAM_CHATS` instead.
- `askConfirmation`, `pinMessage`, delayed sending (`delay_seconds`, `at`) and severity
  levels (`level`).

[Unreleased]: https://github.com/x-mauve/ariel/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/x-mauve/ariel/releases/tag/v0.1.0
