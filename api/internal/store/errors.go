package store

import "errors"

var (
	ErrCliLoginNotFound  = errors.New("CLI login request not found")
	ErrCliLoginPending   = errors.New("CLI login request is pending")
	ErrCliLoginDenied    = errors.New("CLI login request was denied")
	ErrCliLoginExpired   = errors.New("CLI login request expired")
	ErrCliLoginUsed      = errors.New("CLI login request was already used")
	ErrCliSessionInvalid = errors.New("CLI session is invalid or expired")
	ErrRefreshTokenReuse = errors.New("refresh token reuse detected")
	// ErrAlreadyExists means the row the caller tried to add is already there.
	ErrAlreadyExists = errors.New("already exists")
	// ErrRefreshTokenRecentlyUsed means the token was rotated a moment ago.
	// Two browser tabs that resume at the same time send the same cookie; the
	// second one gets this instead of a reuse revocation. The session is
	// returned with it, and no new token is stored.
	ErrRefreshTokenRecentlyUsed = errors.New("refresh token was used moments ago")
)
