package websession

import (
	"strings"
	"testing"
)

func TestDeriveAutoTitleFromMessage(t *testing.T) {
	t.Run("uses first sentence from first non-empty line", func(t *testing.T) {
		title := deriveAutoTitleFromMessage("\n\n修复登录接口超时问题。顺便补一下测试。")
		if title != "修复登录接口超时问题。" {
			t.Fatalf("expected first sentence, got %q", title)
		}
	})

	t.Run("falls back to first line when no sentence boundary", func(t *testing.T) {
		title := deriveAutoTitleFromMessage("Refactor websocket session syncing without changing API")
		if title != "Refactor websocket session syncing without changing API" {
			t.Fatalf("unexpected title %q", title)
		}
	})

	t.Run("collapses whitespace and truncates long titles", func(t *testing.T) {
		title := deriveAutoTitleFromMessage("  this   title    should   be   compacted  " + strings.Repeat("x", 80))
		if !strings.HasPrefix(title, "this title should be compacted ") {
			t.Fatalf("expected compacted whitespace, got %q", title)
		}
		if len([]rune(title)) != maxAutoTitleRunes {
			t.Fatalf("expected %d runes, got %d", maxAutoTitleRunes, len([]rune(title)))
		}
		if !strings.HasSuffix(title, "...") {
			t.Fatalf("expected truncated suffix, got %q", title)
		}
	})
}
