package main

import (
	"testing"

	"go.xchunk.org/ariel/internal/telegram"
)

func TestParseChats(t *testing.T) {
	got, err := parseChats(" me:1 , team:-2,")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (telegram.Chat{Name: "me", ID: 1}) || got[1] != (telegram.Chat{Name: "team", ID: -2}) {
		t.Errorf("got %v", got)
	}

	for _, bad := range []string{"me", ":1", "me:x"} {
		if _, err := parseChats(bad); err == nil {
			t.Errorf("parseChats(%q): want error", bad)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_CHATS", "me:1,team:-2")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.chats) != 2 || cfg.chats[0].Name != "me" {
		t.Errorf("got %+v", cfg.chats)
	}

	for _, bad := range []string{"", "me:1,me:2"} {
		t.Setenv("TELEGRAM_CHATS", bad)
		if _, err := loadConfig(); err == nil {
			t.Errorf("loadConfig with TELEGRAM_CHATS=%q: want error", bad)
		}
	}
}
