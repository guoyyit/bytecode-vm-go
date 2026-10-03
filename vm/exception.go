package vm

import "fmt"

// Exception is a thrown value with a message and call trace.
type Exception struct {
	Message    string
	Trace      []FrameInfo
	Suppressed []*Exception
}

// NewException creates an exception with the given message.
func NewException(msg string) *Exception {
	return &Exception{Message: msg}
}

// String returns a human-readable representation including the trace.
func (e *Exception) String() string {
	s := fmt.Sprintf("exception: %s", e.Message)
	if len(e.Trace) > 0 {
		s += "\n" + formatTrace(e.Trace)
	}
	for _, sup := range e.Suppressed {
		s += fmt.Sprintf("\n  suppressed: %s", sup.Message)
	}
	return s
}

func (e *Exception) memSize() int64 {
	size := int64(48)
	size += int64(len(e.Trace)) * 32
	for _, sup := range e.Suppressed {
		size += sup.memSize()
	}
	return size
}

// tryRecord marks an exception-handling region in a frame.
type tryRecord struct {
	handlerOffset int32 // -1 if no catch handler
	finallyOffset int32 // -1 if no finally cleanup
}
