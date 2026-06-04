package valueobjects

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

type ExternalEventID struct{ value string }

func NewExternalEventIDCascade(rawBody []byte) (ExternalEventID, error) {
	if len(rawBody) == 0 {
		return ExternalEventID{}, ErrEmptyPayload
	}
	if !json.Valid(rawBody) {
		return ExternalEventID{}, ErrMalformedPayload
	}
	var probe struct {
		ID    string `json:"id"`
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	_ = json.Unmarshal(rawBody, &probe)
	if v := strings.TrimSpace(probe.ID); v != "" {
		return ExternalEventID{value: v}, nil
	}
	if v := strings.TrimSpace(probe.Order.ID); v != "" {
		return ExternalEventID{value: v}, nil
	}
	sum := sha256.Sum256(rawBody)
	return ExternalEventID{value: "sha256:" + hex.EncodeToString(sum[:])}, nil
}

func (e ExternalEventID) String() string { return e.value }
func (e ExternalEventID) IsZero() bool   { return e.value == "" }
