package notifysettings

import "errors"

var (
	ErrInvalid  = errors.New("invalid notification settings")
	ErrNotFound = errors.New("notification settings not found")
)
