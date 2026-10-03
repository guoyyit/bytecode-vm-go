package bytecode

import "fmt"

// LoadErrorKind classifies why a bytecode artifact was rejected at load time.
type LoadErrorKind int

const (
	ErrBadMagic LoadErrorKind = iota
	ErrBadFormatVersion
	ErrUnsupportedVersion
	ErrTruncated
	ErrBadConstantType
	ErrConstantIndexOutOfRange
	ErrBadOpcode
	ErrOpcodeVersionMismatch
	ErrBadJumpTarget
	ErrLocalIndexOutOfRange
	ErrFunctionIndexOutOfRange
	ErrArgCountMismatch
	ErrInvalidCode
	ErrEmptyModule
)

// LoadError is returned when a bytecode artifact fails structural validation.
type LoadError struct {
	Kind    LoadErrorKind
	Message string
	Offset  int // byte offset in the artifact where the problem was detected (-1 if N/A)
}

func (e *LoadError) Error() string {
	if e.Offset >= 0 {
		return fmt.Sprintf("bytecode load error at offset %d: %s", e.Offset, e.Message)
	}
	return fmt.Sprintf("bytecode load error: %s", e.Message)
}

func newLoadError(kind LoadErrorKind, offset int, msg string) *LoadError {
	return &LoadError{Kind: kind, Message: msg, Offset: offset}
}

func newLoadErrorf(kind LoadErrorKind, offset int, format string, args ...interface{}) *LoadError {
	return &LoadError{Kind: kind, Message: fmt.Sprintf(format, args...), Offset: offset}
}
