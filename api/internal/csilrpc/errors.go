package csilrpc

import "net/http"

// Error is the caller-visible failure type. Handlers return it (typically
// via the constructors below) and the dispatcher serializes it as a
// CBOR-encoded csil.ServiceError with the matching HTTP status. Plain Go
// errors returned from a handler are treated as internal 500s.
//
// The code field doubles as both the application code and the HTTP status —
// it's intentionally small enough to make this trivial. Larger callers can
// branch on the message; small callers can branch on the status alone.
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }

// NewError constructs an *Error with an arbitrary HTTP-shaped code. Most
// callers should reach for the BadRequest/Unauthorized/Forbidden/NotFound/
// Conflict/Internal helpers below — those make the call site read.
func NewError(code int, msg string) *Error { return &Error{Code: code, Message: msg} }

func badRequest(msg string) *Error     { return &Error{Code: http.StatusBadRequest, Message: msg} }
func unauthorized(msg string) *Error   { return &Error{Code: http.StatusUnauthorized, Message: msg} }
func forbidden(msg string) *Error      { return &Error{Code: http.StatusForbidden, Message: msg} }
func notFound(msg string) *Error       { return &Error{Code: http.StatusNotFound, Message: msg} }
func conflict(msg string) *Error       { return &Error{Code: http.StatusConflict, Message: msg} }
func methodNotAllowed(m string) *Error { return &Error{Code: http.StatusMethodNotAllowed, Message: m} }
func internal(msg string) *Error       { return &Error{Code: http.StatusInternalServerError, Message: msg} }

// Exported counterparts for use from handler packages.
func BadRequest(msg string) *Error   { return badRequest(msg) }
func Unauthorized(msg string) *Error { return unauthorized(msg) }
func Forbidden(msg string) *Error    { return forbidden(msg) }
func NotFound(msg string) *Error     { return notFound(msg) }
func Conflict(msg string) *Error     { return conflict(msg) }
func Internal(msg string) *Error     { return internal(msg) }
