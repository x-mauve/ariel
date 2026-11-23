package tools

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"

	"go.xchunk.org/ariel/internal/telegram"
)

// documentFile returns the name and contents of the file described by in. It
// rejects inputs that set both or neither of path and content, and files over
// the Bot API upload limit.
func documentFile(in sendDocumentIn) (string, []byte, error) {
	switch {
	case (in.Path == "") == (in.Content == ""):
		return "", nil, fmt.Errorf("set exactly one of path and content")
	case in.Content != "":
		if len(in.Content) > telegram.MaxUploadSize {
			return "", nil, fmt.Errorf("content exceeds the %d MiB upload limit", telegram.MaxUploadSize>>20)
		}
		return cmp.Or(in.Filename, "report.txt"), []byte(in.Content), nil
	}

	info, err := os.Stat(in.Path)
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%s is not a regular file", in.Path)
	}
	if info.Size() > telegram.MaxUploadSize {
		return "", nil, fmt.Errorf("%s exceeds the %d MiB upload limit", in.Path, telegram.MaxUploadSize>>20)
	}

	data, err := os.ReadFile(in.Path)
	if err != nil {
		return "", nil, err
	}

	return cmp.Or(in.Filename, filepath.Base(in.Path)), data, nil
}
