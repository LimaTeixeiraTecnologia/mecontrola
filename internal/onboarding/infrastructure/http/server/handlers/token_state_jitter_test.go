package handlers

import (
	"testing"
	"time"
)

func TestCryptoJitter_RangeBounds(t *testing.T) {
	minMs := tokenStateJitterMinMs
	maxMs := tokenStateJitterMaxMs

	low := time.Duration(minMs) * time.Millisecond
	high := time.Duration(maxMs) * time.Millisecond

	for i := 0; i < 256; i++ {
		got := cryptoJitter(minMs, maxMs)
		if got < low {
			t.Fatalf("iteration %d: got %v, want >= %v", i, got, low)
		}
		if got >= high {
			t.Fatalf("iteration %d: got %v, want < %v", i, got, high)
		}
	}
}

func TestCryptoJitter_DegenerateRangeReturnsFloor(t *testing.T) {
	got := cryptoJitter(5, 5)
	want := 5 * time.Millisecond
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	got = cryptoJitter(10, 3)
	want = 10 * time.Millisecond
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}
