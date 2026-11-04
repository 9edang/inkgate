package context

import (
	std "context"
	"time"
)

// The whole standard library context API, re-exported, so the project imports
// this package instead of "context" and gets the flow context in the same place.
// Context is the standard interface itself (a type alias, not a new type), so
// every value works with any library that expects context.Context.

type (
	Context         = std.Context
	CancelFunc      = std.CancelFunc
	CancelCauseFunc = std.CancelCauseFunc
)

var (
	Canceled         = std.Canceled
	DeadlineExceeded = std.DeadlineExceeded
)

func Background() Context { return std.Background() }
func TODO() Context       { return std.TODO() }

func WithCancel(parent Context) (Context, CancelFunc) { return std.WithCancel(parent) }
func WithCancelCause(parent Context) (Context, CancelCauseFunc) {
	return std.WithCancelCause(parent)
}
func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
	return std.WithDeadline(parent, d)
}
func WithDeadlineCause(parent Context, d time.Time, cause error) (Context, CancelFunc) {
	return std.WithDeadlineCause(parent, d, cause)
}
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return std.WithTimeout(parent, timeout)
}
func WithTimeoutCause(parent Context, timeout time.Duration, cause error) (Context, CancelFunc) {
	return std.WithTimeoutCause(parent, timeout, cause)
}
func WithValue(parent Context, key, val any) Context { return std.WithValue(parent, key, val) }
func WithoutCancel(parent Context) Context           { return std.WithoutCancel(parent) }
func AfterFunc(ctx Context, f func()) (stop func() bool) {
	return std.AfterFunc(ctx, f)
}
func Cause(c Context) error { return std.Cause(c) }
