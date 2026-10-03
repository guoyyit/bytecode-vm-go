package vm

import "fmt"

// LoadReason 标识加载阶段失败的具体原因，便于调用方区分处理。
type LoadReason int

const (
	ReasonBadMagic LoadReason = iota
	ReasonBadChecksum
	ReasonTruncated
	ReasonUnsupportedVersion
	ReasonUnknownOpcode
	ReasonInvalidConstant
	ReasonInvalidFunction
	ReasonInvalidEntry
)

func (r LoadReason) String() string {
	switch r {
	case ReasonBadMagic:
		return "bad magic"
	case ReasonBadChecksum:
		return "checksum mismatch"
	case ReasonTruncated:
		return "truncated artifact"
	case ReasonUnsupportedVersion:
		return "unsupported version"
	case ReasonUnknownOpcode:
		return "unknown opcode"
	case ReasonInvalidConstant:
		return "invalid constant pool"
	case ReasonInvalidFunction:
		return "invalid function table"
	case ReasonInvalidEntry:
		return "invalid entry function"
	}
	return "unknown load reason"
}

// LoadError 表示字节码加载/校验失败。
type LoadError struct {
	Reason LoadReason
	Detail string
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("bytecode load error: %s: %s", e.Reason, e.Detail)
}

func loadErr(r LoadReason, format string, args ...any) *LoadError {
	return &LoadError{Reason: r, Detail: fmt.Sprintf(format, args...)}
}

// RuntimeKind 标识运行时错误的类别。
type RuntimeKind int

const (
	KindStackOverflow RuntimeKind = iota
	KindStackUnderflow
	KindBadJump
	KindCallDepthExceeded
	KindStepLimitExceeded
	KindMemoryLimitExceeded
	KindDivByZero
	KindTypeError
	KindInvalidLocal
	KindBadCall
	KindUncaughtException
)

func (k RuntimeKind) String() string {
	switch k {
	case KindStackOverflow:
		return "stack overflow"
	case KindStackUnderflow:
		return "stack underflow"
	case KindBadJump:
		return "bad jump target"
	case KindCallDepthExceeded:
		return "call depth exceeded"
	case KindStepLimitExceeded:
		return "step limit exceeded"
	case KindMemoryLimitExceeded:
		return "memory limit exceeded"
	case KindDivByZero:
		return "division by zero"
	case KindTypeError:
		return "type error"
	case KindInvalidLocal:
		return "invalid local slot"
	case KindBadCall:
		return "bad call"
	case KindUncaughtException:
		return "uncaught exception"
	}
	return "unknown runtime error"
}

// FrameInfo 描述调用轨迹中的一帧。
type FrameInfo struct {
	Function string
	PC       int
}

// RuntimeError 表示执行期错误，携带可读调用轨迹。
type RuntimeError struct {
	Kind    RuntimeKind
	Message string
	Trace   []FrameInfo
}

func (e *RuntimeError) Error() string {
	s := fmt.Sprintf("runtime error: %s: %s", e.Kind, e.Message)
	for _, f := range e.Trace {
		s += fmt.Sprintf("\n  at %s (pc=%d)", f.Function, f.PC)
	}
	return s
}
