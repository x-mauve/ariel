package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"go.xchunk.org/argon"
	"go.xchunk.org/lithium"
)

// guideURI is the URI of the resource that teaches the model to write rich messages.
const guideURI = "ariel://guide"

// guide is the rich message writing guide served at guideURI.
//
//go:embed guide.md
var guide string

func registerGuide(srv *argon.Server) {
	argon.Resource(srv,
		guideURI,
		"guide",
		"How to write Telegram rich messages: when to use them, structure and style, "+
			"Markdown and HTML syntax, limits and examples. Read before the first sendRichMessage call",
		func(ctx context.Context) (argon.ResourceContents, error) {
			return argon.TextResource(guide), nil
		},
		argon.MIMEType("text/markdown"),
		argon.ResourceTitle("Rich message guide"),
	)
}

// withMarkupHint wraps err, returned by a Bot API call that sent rich markup,
// with a pointer to the guide when Telegram rejected the request as malformed.
// Other errors, e.g. network failures, are wrapped with op only.
func withMarkupHint(op string, err error) error {
	var apiErr *lithium.Error
	if errors.As(err, &apiErr) && apiErr.Code == 400 {
		return fmt.Errorf("%s: %w; fix the markup using the syntax in the %s resource and retry", op, err, guideURI)
	}

	return fmt.Errorf("%s: %w", op, err)
}
