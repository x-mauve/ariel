package telegram

// SendMessage is the payload of the Bot API sendMessage method.
type SendMessage struct {
	ChatID              int64  `json:"chat_id"`
	Text                string `json:"text"`
	ParseMode           string `json:"parse_mode,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// Method returns the Bot API method name for the request.
func (SendMessage) Method() string {
	return "sendMessage"
}

// InputRichMessage is the Bot API InputRichMessage object: the content of a
// rich message. Exactly one of Markdown and HTML must be set; the server never
// sends the blocks form.
type InputRichMessage struct {
	Markdown            string `json:"markdown,omitempty"`
	HTML                string `json:"html,omitempty"`
	SkipEntityDetection bool   `json:"skip_entity_detection,omitempty"`
}

// SendRichMessage is the payload of the Bot API sendRichMessage method.
type SendRichMessage struct {
	ChatID              int64            `json:"chat_id"`
	RichMessage         InputRichMessage `json:"rich_message"`
	DisableNotification bool             `json:"disable_notification,omitempty"`
	ProtectContent      bool             `json:"protect_content,omitempty"`
}

// Method returns the Bot API method name for the request.
func (SendRichMessage) Method() string {
	return "sendRichMessage"
}

// EditMessageText is the payload of the Bot API editMessageText method. Set
// either Text, optionally with ParseMode, or RichMessage.
type EditMessageText struct {
	ChatID      int64             `json:"chat_id"`
	MessageID   int64             `json:"message_id"`
	Text        string            `json:"text,omitempty"`
	ParseMode   string            `json:"parse_mode,omitempty"`
	RichMessage *InputRichMessage `json:"rich_message,omitempty"`
}

// Method returns the Bot API method name for the request.
func (EditMessageText) Method() string {
	return "editMessageText"
}

// DeleteMessage is the payload of the Bot API deleteMessage method.
type DeleteMessage struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

// Method returns the Bot API method name for the request.
func (DeleteMessage) Method() string {
	return "deleteMessage"
}

// SendDocument is the payload of the Bot API sendDocument method. The document
// itself is attached as the "document" file field.
type SendDocument struct {
	ChatID              int64  `json:"chat_id"`
	Caption             string `json:"caption,omitempty"`
	ParseMode           string `json:"parse_mode,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// Method returns the Bot API method name for the request.
func (SendDocument) Method() string {
	return "sendDocument"
}

// GetMe is the payload of the Bot API getMe method.
type GetMe struct{}

// Method returns the Bot API method name for the request.
func (GetMe) Method() string {
	return "getMe"
}

// botInfo is the subset of the Bot API BotFullInfo object, returned by getMe,
// used by the server.
type botInfo struct {
	Username string `json:"username,omitempty"`
}

// Message is the subset of the Bot API Message object used by the server.
type Message struct {
	MessageID int64 `json:"message_id"`
}
