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
)
