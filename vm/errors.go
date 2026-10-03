package vm

import "fmt"

// RuntimeErrorKind classifies a runtime error.
type RuntimeErrorKind int

const (
	ErrStackOverflow RuntimeErrorKind = iota
	ErrStackUnderflow
	ErrInvalidJumpTarget
	ErrTypeMismatch
	ErrDivisionByZero
	ErrCallDepthLimit
	ErrStepLimit
	ErrMemoryLimit
	ErrUncaughtException
	ErrInvalidOperation
	ErrLocalIndex
)

// RuntimeError is returned when execution cannot continue.
type RuntimeError struct {
	Kind    RuntimeErrorKind
	Message string
	Trace   []FrameInfo
}

func (e *RuntimeError) Error() string {
	if len(e.Trace) > 0 {
		return fmt.Sprintf("%s\n%s", e.Message, formatTrace(e.Trace))
	}
	return e.Message
}

func newRuntimeError(kind RuntimeErrorKind, msg string) *RuntimeError {
	return &RuntimeError{Kind: kind, Message: msg}
}

func newRuntimeErrorf(kind RuntimeErrorKind, format string, args ...interface{}) *RuntimeError {
	return &RuntimeError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// FrameInfo describes a single call frame for error traces.
type FrameInfo struct {
	Function string
	Offset   int
}

func formatTrace(trace []FrameInfo) string {
	s := "call trace (most recent last):\n"
	for i, f := range trace {
		s += fmt.Sprintf("  %d: %s at offset %d\n", i, f.Function, f.Offset)
	}
	return s
}
