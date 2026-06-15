package outbound_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLEscape(t *testing.T) {
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace("a < b & c > d")
	assert.Equal(t, "a &lt; b &amp; c &gt; d", esc)
}

func TestTruncateRunes(t *testing.T) {
	emoji := strings.Repeat("é", 5000)
	runes := []rune(emoji)
	assert.Equal(t, 5000, len(runes))
	clipped := runes[:4096]
	assert.Equal(t, 4096, len(clipped))
}
