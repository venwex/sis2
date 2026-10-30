package ierr

import "errors"

var (
	ErrPasteNotFound     = errors.New("paste not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrForbidden         = errors.New("access denied")
	ErrShortCodeConflict = errors.New("short code already exists")
)
