package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidCursor = errors.New("pagination: invalid cursor")

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func Encode(createdAt time.Time, id string) (string, error) {
	raw, err := json.Marshal(Cursor{CreatedAt: createdAt.UTC(), ID: id})
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(raw), nil
}

func Decode(cursor string) (Cursor, error) {
	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: base64 decode: %s", ErrInvalidCursor, err)
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cursor{}, fmt.Errorf("%w: json unmarshal: %s", ErrInvalidCursor, err)
	}
	if c.CreatedAt.IsZero() {
		return Cursor{}, fmt.Errorf("%w: created_at is zero", ErrInvalidCursor)
	}
	if c.ID == "" {
		return Cursor{}, fmt.Errorf("%w: id is empty", ErrInvalidCursor)
	}
	return c, nil
}
