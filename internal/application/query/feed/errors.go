package feed

import "errors"

// ErrInvalidCursor is returned when the feed cursor cannot be decoded.
var ErrInvalidCursor = errors.New("invalid feed cursor")
