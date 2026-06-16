package outbound

import (
	"strings"
	"testing"
)

func computeSafeText(text string) string {
	return htmlEscaper.Replace(truncateRunes(text, maxTelegramMessageRunes))
}

func TestSafeText_TruncateBeforeEscape_DoesNotSplitEntity(t *testing.T) {
	prefix := strings.Repeat("a", 4090)
	suffix := "&" + strings.Repeat("z", 6)
	input := prefix + suffix
	runes := []rune(input)
	if len(runes) != 4097 {
		t.Fatalf("input rune length: want 4097, got %d", len(runes))
	}

	safe := computeSafeText(input)

	if strings.Contains(safe, "&") && !strings.Contains(safe, "&amp;") {
		t.Fatalf("bare & without semicolon detected — entity split: tail=%q", safe[max(0, len(safe)-20):])
	}

	for _, suffixChar := range []string{"&", "&a", "&am", "&amp"} {
		if strings.HasSuffix(safe, suffixChar) {
			t.Fatalf("safe text ends with truncated entity %q; tail=%q", suffixChar, safe[max(0, len(safe)-12):])
		}
	}
}

func TestSafeText_EscapesNormalContent(t *testing.T) {
	got := computeSafeText("a < b & c > d")
	want := "a &lt; b &amp; c &gt; d"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSafeText_TruncateBeforeEscape_KeepsTrailingAmpersandEscaped(t *testing.T) {
	prefix := strings.Repeat("a", 4095)
	input := prefix + "&"
	safe := computeSafeText(input)

	if !strings.HasSuffix(safe, "&amp;") {
		t.Fatalf("trailing & inside max range must be escaped intact; got suffix=%q", safe[max(0, len(safe)-8):])
	}
}

func TestSafeText_ShortInputUnchangedShape(t *testing.T) {
	got := computeSafeText("plain text")
	if got != "plain text" {
		t.Fatalf("got %q, want %q", got, "plain text")
	}
}
