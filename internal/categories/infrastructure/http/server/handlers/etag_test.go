package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEtagHeader(t *testing.T) {
	tests := []struct {
		name    string
		version int64
		want    string
	}{
		{"version 1", 1, `"v1"`},
		{"version 42", 42, `"v42"`},
		{"version 0", 0, `"v0"`},
		{"large version", 999999, `"v999999"`},
	}

	h := newETagHelper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.header(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseETag(t *testing.T) {
	tests := []struct {
		name string
		etag string
		want int64
	}{
		{"valid v prefix", `"v42"`, 42},
		{"valid without prefix", `"42"`, 42},
		{"with spaces", ` "v42" `, 42},
		{"invalid string", `"invalid"`, 0},
		{"empty string", "", 0},
		{"just v", `"v"`, 0},
	}

	h := newETagHelper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.parse(tt.etag)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckIfNoneMatch(t *testing.T) {
	tests := []struct {
		name           string
		ifNoneMatch    string
		currentVersion int64
		want           bool
	}{
		{"matches", `"v42"`, 42, true},
		{"does not match", `"v42"`, 43, false},
		{"empty header", "", 42, false},
		{"different format", `"42"`, 42, true},
	}

	h := newETagHelper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tt.ifNoneMatch)
			}
			got := h.checkIfNoneMatch(req, tt.currentVersion)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatETag(t *testing.T) {
	tests := []struct {
		name    string
		version int64
		want    string
	}{
		{"version 1", 1, `"v1"`},
		{"version 42", 42, `"v42"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatETag(tt.version)
			assert.Equal(t, tt.want, got)
		})
	}
}
